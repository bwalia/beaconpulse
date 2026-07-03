// Package insight provides tenant-scoped read models over Prometheus. Because
// Prometheus itself is single-tenant (its UI shows every organization's data),
// Beacon never sends users to it directly; instead this package queries
// Prometheus filtered by the caller's org_id label so each organization sees
// only its own alerts and monitor metrics. Ownership is additionally verified
// against the database before returning per-monitor data.
package insight

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/monitor"
	"beacon/internal/platform/apperror"
)

// Alert is a currently-firing alert for one of the org's monitors.
type Alert struct {
	Name        string
	Severity    string
	MonitorID   string
	MonitorName string
	MonitorType string
	Target      string
	Since       time.Time
}

// Point is a timestamped value for charts.
type Point struct {
	T time.Time
	V float64
}

// MonitorMetrics summarizes a monitor's recent health from Prometheus.
type MonitorMetrics struct {
	MonitorID         string
	WindowHours       int
	UptimePercent     float64
	ResponseMsCurrent float64
	ResponseMsAvg     float64
	Up                []Point
	ResponseMs        []Point
}

// MonitorUptime is one monitor's up/down history for a status-bar row.
type MonitorUptime struct {
	MonitorID     string
	MonitorName   string
	Target        string
	AvgResponseMs float64
	Points        []Point // v: 1 = up, 0 = down, absent = no data
}

// Overview is the org-wide dashboard read model.
type Overview struct {
	WindowHours    int
	UptimePercent  float64
	AvgResponseMs  float64
	UptimeSeries   []Point // overall availability (%) over the window
	ResponseSeries []Point // avg response (ms) over the window
	Monitors       []MonitorUptime
}

// Sample / Series are the query results the domain consumes (mapped from the
// Prometheus adapter so this package doesn't import it).
type Sample struct {
	Labels map[string]string
	Value  float64
}

type RangeSeries struct {
	Labels map[string]string
	Points []Point
}

// Querier is the read port onto Prometheus, implemented by the promapi adapter.
type Querier interface {
	Query(ctx context.Context, expr string) ([]Sample, error)
	QueryRange(ctx context.Context, expr string, start, end time.Time, step time.Duration) ([]RangeSeries, error)
}

// MonitorLookup verifies a monitor belongs to an org (satisfied by the monitor
// repository). It returns a not-found error when the monitor is absent or owned
// by another tenant.
type MonitorLookup interface {
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*monitor.Monitor, error)
}

// Service implements the tenant-scoped insight use cases.
type Service struct {
	q        Querier
	monitors MonitorLookup
	now      func() time.Time
}

// NewService wires the insight service.
func NewService(q Querier, monitors MonitorLookup) *Service {
	return &Service{q: q, monitors: monitors, now: time.Now}
}

// ActiveAlerts returns the firing alerts scoped to the given organization. The
// org_id filter is applied in the PromQL selector, so no other tenant's alerts
// can be returned.
func (s *Service) ActiveAlerts(ctx context.Context, orgID uuid.UUID) ([]Alert, error) {
	firing, err := s.q.Query(ctx, fmt.Sprintf(`ALERTS{org_id="%s",alertstate="firing"}`, orgID))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// ALERTS_FOR_STATE carries the activation timestamp as its value; join by
	// (alertname, monitor_id) to show "firing since".
	since := map[string]time.Time{}
	if forState, err := s.q.Query(ctx, fmt.Sprintf(`ALERTS_FOR_STATE{org_id="%s"}`, orgID)); err == nil {
		for _, fs := range forState {
			since[alertKey(fs.Labels)] = time.Unix(int64(fs.Value), 0).UTC()
		}
	}

	out := make([]Alert, 0, len(firing))
	for _, a := range firing {
		alert := Alert{
			Name:        a.Labels["alertname"],
			Severity:    a.Labels["severity"],
			MonitorID:   a.Labels["monitor_id"],
			MonitorName: a.Labels["monitor_name"],
			MonitorType: a.Labels["monitor_type"],
			Target:      a.Labels["instance"],
		}
		if t, ok := since[alertKey(a.Labels)]; ok {
			alert.Since = t
		}
		out = append(out, alert)
	}
	return out, nil
}

// MonitorMetrics returns recent health metrics for a single monitor, after
// verifying it belongs to the caller's org.
func (s *Service) MonitorMetrics(ctx context.Context, orgID, monitorID uuid.UUID, window time.Duration) (*MonitorMetrics, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	if _, err := s.monitors.GetByID(ctx, orgID, monitorID); err != nil {
		return nil, err // NotFound if the monitor isn't this tenant's
	}

	id := monitorID.String()
	winStr := durationToPromRange(window)
	m := &MonitorMetrics{MonitorID: id, WindowHours: int(window.Hours())}

	m.UptimePercent = round2(firstValue(s.instant(ctx, fmt.Sprintf(`avg_over_time(probe_success{monitor_id="%s"}[%s]) * 100`, id, winStr))))
	m.ResponseMsAvg = round2(firstValue(s.instant(ctx, fmt.Sprintf(`avg_over_time(probe_duration_seconds{monitor_id="%s"}[%s]) * 1000`, id, winStr))))
	m.ResponseMsCurrent = round2(firstValue(s.instant(ctx, fmt.Sprintf(`probe_duration_seconds{monitor_id="%s"} * 1000`, id))))

	end := s.now().UTC()
	start := end.Add(-window)
	step := window / 60 // ~60 points across the window
	m.ResponseMs = firstSeries(s.rng(ctx, fmt.Sprintf(`probe_duration_seconds{monitor_id="%s"} * 1000`, id), start, end, step))
	m.Up = firstSeries(s.rng(ctx, fmt.Sprintf(`probe_success{monitor_id="%s"}`, id), start, end, step))
	return m, nil
}

// Overview returns the org-wide dashboard metrics: aggregate uptime and response
// time (instant + time series) plus per-monitor uptime history. All queries are
// scoped to the org via the org_id label.
func (s *Service) Overview(ctx context.Context, orgID uuid.UUID, window time.Duration, buckets int) (*Overview, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	if buckets < 10 {
		buckets = 48
	}
	org := orgID.String()
	win := durationToPromRange(window)
	o := &Overview{WindowHours: int(window.Hours())}

	o.UptimePercent = round2(firstValue(s.instant(ctx, fmt.Sprintf(`avg(avg_over_time(probe_success{org_id="%s"}[%s])) * 100`, org, win))))
	o.AvgResponseMs = round2(firstValue(s.instant(ctx, fmt.Sprintf(`avg(avg_over_time(probe_duration_seconds{org_id="%s"}[%s])) * 1000`, org, win))))

	end := s.now().UTC()
	start := end.Add(-window)
	step := window / time.Duration(buckets)
	o.UptimeSeries = firstSeries(s.rng(ctx, fmt.Sprintf(`avg(probe_success{org_id="%s"}) * 100`, org), start, end, step))
	o.ResponseSeries = firstSeries(s.rng(ctx, fmt.Sprintf(`avg(probe_duration_seconds{org_id="%s"}) * 1000`, org), start, end, step))

	// Per-monitor average response time over the window, keyed by monitor id.
	respByID := map[string]float64{}
	for _, sample := range s.instant(ctx, fmt.Sprintf(`avg_over_time(probe_duration_seconds{org_id="%s"}[%s]) * 1000`, org, win)) {
		respByID[sample.Labels["monitor_id"]] = round2(sample.Value)
	}

	for _, series := range s.rng(ctx, fmt.Sprintf(`probe_success{org_id="%s"}`, org), start, end, step) {
		id := series.Labels["monitor_id"]
		o.Monitors = append(o.Monitors, MonitorUptime{
			MonitorID:     id,
			MonitorName:   series.Labels["monitor_name"],
			Target:        series.Labels["instance"],
			AvgResponseMs: respByID[id],
			Points:        series.Points,
		})
	}
	return o, nil
}

// ---- helpers ----

func (s *Service) instant(ctx context.Context, expr string) []Sample {
	res, err := s.q.Query(ctx, expr)
	if err != nil {
		return nil
	}
	return res
}

func (s *Service) rng(ctx context.Context, expr string, start, end time.Time, step time.Duration) []RangeSeries {
	res, err := s.q.QueryRange(ctx, expr, start, end, step)
	if err != nil {
		return nil
	}
	return res
}

func alertKey(labels map[string]string) string {
	return labels["alertname"] + "|" + labels["monitor_id"]
}

func firstValue(samples []Sample) float64 {
	if len(samples) == 0 {
		return 0
	}
	return samples[0].Value
}

func firstSeries(series []RangeSeries) []Point {
	if len(series) == 0 {
		return nil
	}
	return series[0].Points
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }

// durationToPromRange renders a duration as a PromQL range like "24h" or "60m".
func durationToPromRange(d time.Duration) string {
	if d >= time.Hour && d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
