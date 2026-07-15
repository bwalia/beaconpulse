package controlplane

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/google/uuid"

	"beacon/internal/domain/monitor"
	"beacon/internal/domain/plan"
)

// MonitorReader is the narrow read dependency the syncer needs: the full set of
// enabled monitors to project into config, plus each org's effective plan so we
// can cap probing to the tier's monitor limit.
type MonitorReader interface {
	ListAllEnabled(ctx context.Context) ([]monitor.Monitor, error)
	EffectivePlans(ctx context.Context) (map[uuid.UUID]plan.Plan, error)
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
	plans, err := s.reader.EffectivePlans(ctx)
	if err != nil {
		return fmt.Errorf("controlplane: list org plans: %w", err)
	}
	// Enforce the "fall back to Free" rule: probe at most the effective tier's
	// monitor limit per org, oldest first. A depleted pay-as-you-go org keeps its
	// 10 free monitors probing; the rest go dark until it pays again.
	monitors = capToPlan(monitors, plans)

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

// capToPlan keeps, per org, only the oldest N enabled monitors where N is the
// effective tier's MaxMonitors — the control-plane half of "fall back to Free".
// Input order is preserved (deterministic output, no config churn).
func capToPlan(monitors []monitor.Monitor, plans map[uuid.UUID]plan.Plan) []monitor.Monitor {
	byOrg := map[uuid.UUID][]int{}
	for i := range monitors {
		byOrg[monitors[i].OrgID] = append(byOrg[monitors[i].OrgID], i)
	}
	allowed := make(map[uuid.UUID]bool, len(monitors))
	for org, idxs := range byOrg {
		max := plan.LimitsFor(plans[org]).MaxMonitors // missing org → Free limits
		sort.SliceStable(idxs, func(a, b int) bool {
			return monitors[idxs[a]].CreatedAt.Before(monitors[idxs[b]].CreatedAt)
		})
		for k, i := range idxs {
			if k < max {
				allowed[monitors[i].ID] = true
			}
		}
	}
	kept := make([]monitor.Monitor, 0, len(monitors))
	for _, m := range monitors {
		if allowed[m.ID] {
			kept = append(kept, m)
		}
	}
	return kept
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
