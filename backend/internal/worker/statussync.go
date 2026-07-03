package worker

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"beacon/internal/adapter/promapi"
	"beacon/internal/domain/monitor"
)

// statusWriter is the narrow dependency the StatusSync needs.
type statusWriter interface {
	ApplyStatusUpdates(ctx context.Context, updates []monitor.StatusUpdate) (int64, error)
}

// StatusSync reads probe results from Prometheus and writes the derived up/down
// status back onto each monitor, giving the dashboard a live view. This closes
// the loop: the control plane pushes config into Prometheus, and this pulls
// results back out.
type StatusSync struct {
	prom *promapi.Client
	repo statusWriter
}

// NewStatusSync builds a StatusSync.
func NewStatusSync(prom *promapi.Client, repo statusWriter) *StatusSync {
	return &StatusSync{prom: prom, repo: repo}
}

// Run performs one reconciliation: query probe_success for every monitor and map
// each series (identified by its monitor_id label) to up/down.
func (s *StatusSync) Run(ctx context.Context) error {
	samples, err := s.prom.QueryVector(ctx, "probe_success")
	if err != nil {
		return fmt.Errorf("statussync: query probe_success: %w", err)
	}
	if len(samples) == 0 {
		return nil // nothing being probed yet
	}

	updates := make([]monitor.StatusUpdate, 0, len(samples))
	for _, sample := range samples {
		idStr := sample.Labels["monitor_id"]
		if idStr == "" {
			continue
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		status := monitor.StatusDown
		if sample.Value == 1 {
			status = monitor.StatusUp
		}
		updates = append(updates, monitor.StatusUpdate{
			MonitorID: id,
			Status:    status,
			CheckedAt: sample.Timestamp,
		})
	}

	if _, err := s.repo.ApplyStatusUpdates(ctx, updates); err != nil {
		return fmt.Errorf("statussync: apply updates: %w", err)
	}
	return nil
}
