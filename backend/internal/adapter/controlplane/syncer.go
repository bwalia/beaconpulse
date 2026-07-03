package controlplane

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"beacon/internal/domain/monitor"
)

// MonitorReader is the narrow read dependency the syncer needs: the full set of
// enabled monitors to project into config.
type MonitorReader interface {
	ListAllEnabled(ctx context.Context) ([]monitor.Monitor, error)
}

// Paths locates the files the syncer owns and rewrites.
type Paths struct {
	BlackboxConfigFile string
	ScrapeFile         string
	RulesFile          string
}

// Syncer is the concrete monitor.Syncer. It serializes syncs with a mutex so
// concurrent monitor edits cannot interleave file writes, and reloads Prometheus
// and Blackbox after writing.
type Syncer struct {
	reader   MonitorReader
	genCfg   GeneratorConfig
	paths    Paths
	reloader *Reloader
	mu       sync.Mutex
}

// NewSyncer builds a Syncer.
func NewSyncer(reader MonitorReader, genCfg GeneratorConfig, paths Paths, reloader *Reloader) *Syncer {
	return &Syncer{reader: reader, genCfg: genCfg, paths: paths, reloader: reloader}
}

var _ monitor.Syncer = (*Syncer)(nil)

// Sync regenerates all control-plane config from the database and reloads the
// monitoring services. It is safe to call concurrently and is idempotent.
func (s *Syncer) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	monitors, err := s.reader.ListAllEnabled(ctx)
	if err != nil {
		return fmt.Errorf("controlplane: list monitors: %w", err)
	}
	arts, err := Generate(s.genCfg, monitors)
	if err != nil {
		return fmt.Errorf("controlplane: generate: %w", err)
	}

	if err := writeAtomic(s.paths.BlackboxConfigFile, arts.BlackboxYAML); err != nil {
		return err
	}
	if err := writeAtomic(s.paths.ScrapeFile, arts.ScrapeYAML); err != nil {
		return err
	}
	if err := writeAtomic(s.paths.RulesFile, arts.RulesYAML); err != nil {
		return err
	}

	// Reload both services. Errors are aggregated so one failing reload does not
	// hide the other; the worker will retry the whole sync.
	var reloadErr error
	if err := s.reloader.ReloadBlackbox(ctx); err != nil {
		reloadErr = errors.Join(reloadErr, err)
	}
	if err := s.reloader.ReloadPrometheus(ctx); err != nil {
		reloadErr = errors.Join(reloadErr, err)
	}
	return reloadErr
}

// writeAtomic writes data to path via a temp file + rename so readers never see
// a partially-written config. It creates the parent directory if needed.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("controlplane: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".beacon-*.tmp")
	if err != nil {
		return fmt.Errorf("controlplane: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op if the rename succeeded

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("controlplane: write temp: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("controlplane: chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("controlplane: close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("controlplane: rename to %s: %w", path, err)
	}
	return nil
}
