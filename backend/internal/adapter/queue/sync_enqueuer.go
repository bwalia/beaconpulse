package queue

import (
	"context"

	"beacon/internal/domain/monitor"
)

// JobControlPlaneSync is enqueued whenever monitors change and the control plane
// must be reconciled. The worker's handler runs the actual Prometheus/Blackbox
// regeneration and reload.
const JobControlPlaneSync = "controlplane.sync"

// SyncEnqueuer implements monitor.Syncer by enqueuing a reconciliation job
// rather than performing it inline. This keeps API requests fast and moves the
// (potentially slow, failure-prone) reload off the request path and onto the
// crash-resilient worker.
type SyncEnqueuer struct {
	q *Queue
}

// NewSyncEnqueuer builds a SyncEnqueuer.
func NewSyncEnqueuer(q *Queue) *SyncEnqueuer { return &SyncEnqueuer{q: q} }

var _ monitor.Syncer = (*SyncEnqueuer)(nil)

// Sync enqueues a control-plane reconciliation job.
func (e *SyncEnqueuer) Sync(ctx context.Context) error {
	return e.q.Enqueue(ctx, JobControlPlaneSync, nil)
}
