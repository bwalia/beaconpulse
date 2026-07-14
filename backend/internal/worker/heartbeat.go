package worker

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"beacon/internal/domain/monitor"
)

// heartbeatReader is the narrow dependency the exporter needs.
type heartbeatReader interface {
	ListAllEnabled(ctx context.Context) ([]monitor.Monitor, error)
}

// HeartbeatExporter publishes, for every enabled heartbeat monitor, a gauge of
// its last-ping unix time. Prometheus scrapes it (job beacon-worker) and the
// generated HeartbeatMissed rule alerts when `time() - gauge` exceeds the
// monitor's interval + grace.
//
// Why the WORKER owns this gauge, not the API:
//   - The gauge must have exactly ONE series per monitor. If the API owned it,
//     each API replica would hold its own in-memory value and a ping to replica A
//     would leave replica B's series stale — Prometheus would scrape both and the
//     stale one would fire a false alert. The worker runs single-instance
//     (Recreate strategy), so it is the natural single writer.
//   - The API ping path stays stateless: it only writes last_ping_at to the DB.
//     That is the component you scale horizontally, and it now can be.
//   - Durability is free. The exporter rebuilds the gauge from the DB every tick,
//     so a worker restart re-seeds itself on its next run — no special boot path.
type HeartbeatExporter struct {
	repo heartbeatReader
	gsv  *prometheus.GaugeVec
}

// NewHeartbeatExporter builds the exporter and registers its gauge on reg.
func NewHeartbeatExporter(repo heartbeatReader, reg prometheus.Registerer) *HeartbeatExporter {
	gsv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "beacon_heartbeat_last_ping_timestamp_seconds",
		Help: "Unix time of the last received ping for a heartbeat monitor.",
	}, []string{"monitor_id", "org_id", "project_id"})
	reg.MustRegister(gsv)
	return &HeartbeatExporter{repo: repo, gsv: gsv}
}

// Run rebuilds the gauge from the current set of enabled heartbeats.
//
// Reset-then-repopulate on each tick is deliberate: it drops the series for any
// heartbeat that has since been paused or deleted, so a removed monitor stops
// alerting rather than lingering as a stale, forever-missed series.
func (e *HeartbeatExporter) Run(ctx context.Context) error {
	monitors, err := e.repo.ListAllEnabled(ctx)
	if err != nil {
		return fmt.Errorf("heartbeat exporter: list monitors: %w", err)
	}

	e.gsv.Reset()
	for i := range monitors {
		m := &monitors[i]
		if m.Type != monitor.TypeHeartbeat || m.LastPingAt == nil {
			continue
		}
		e.gsv.WithLabelValues(
			m.ID.String(), m.OrgID.String(), m.ProjectID.String(),
		).Set(float64(m.LastPingAt.Unix()))
	}
	return nil
}
