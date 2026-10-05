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
	Points        []Window // absent window = no data
}

// Window is one slot of a status strip: every check that ran in the window
// ending at T, reduced to what a hover needs to explain the verdict. Only the
// fields the monitor's type produces are set.
type Window struct {
	T time.Time
	// V is the share of checks that passed (0..1). Heartbeats: 0 if a ping was
	// missed at any point in the window, else 1.
	V             float64
	Checks        int
	AvgMs         float64
	CodeMin       int // lowest/highest HTTP status seen; 0 = no response
	CodeMax       int
	KeywordFailed bool  // a body keyword check failed at least once
	SSLExpiry     int64 // unix time of the earliest certificate expiry seen
	Pings         int   // heartbeat pings received
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
	m.Up = firstSeries(s.rng(ctx, fmt.Sprintf(`avg_over_time(probe_success{monitor_id="%s"}[%s])`, id, promSeconds(step)), start, end, step))
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
	// Every range query below aggregates over [step], so each point covers the
	// whole window ending at it. A bare probe_success would sample one check per
	// window and paint an outage between samples green.
	slot := promSeconds(step)
	o.UptimeSeries = firstSeries(s.rng(ctx, fmt.Sprintf(`avg(avg_over_time(probe_success{org_id="%s"}[%s])) * 100`, org, slot), start, end, step))
	o.ResponseSeries = firstSeries(s.rng(ctx, fmt.Sprintf(`avg(probe_duration_seconds{org_id="%s"}) * 1000`, org), start, end, step))

	// Per-monitor average response time over the window, keyed by monitor id.
	respByID := map[string]float64{}
	for _, sample := range s.instant(ctx, fmt.Sprintf(`avg_over_time(probe_duration_seconds{org_id="%s"}[%s]) * 1000`, org, win)) {
		respByID[sample.Labels["monitor_id"]] = round2(sample.Value)
	}

	// byWindow runs a per-monitor range query and indexes it monitor -> window.
	// ponytail: sequential queries (~9 per overview); run them concurrently if
	// overview latency ever shows up.
	byWindow := func(expr string) map[string]map[int64]float64 {
		out := map[string]map[int64]float64{}
		for _, series := range s.rng(ctx, fmt.Sprintf(expr, org, slot), start, end, step) {
			id := series.Labels["monitor_id"]
			if out[id] == nil {
				out[id] = map[int64]float64{}
			}
			for _, p := range series.Points {
				out[id][p.T.Unix()] = p.V
			}
		}
		return out
	}
	checks := byWindow(`sum by (monitor_id) (count_over_time(probe_success{org_id="%s"}[%s]))`)
	avgMs := byWindow(`avg by (monitor_id) (avg_over_time(probe_duration_seconds{org_id="%s"}[%s])) * 1000`)
	codeMin := byWindow(`min by (monitor_id) (min_over_time(probe_http_status_code{org_id="%s"}[%s]))`)
	codeMax := byWindow(`max by (monitor_id) (max_over_time(probe_http_status_code{org_id="%s"}[%s]))`)
	keyword := byWindow(`max by (monitor_id) (max_over_time(probe_failed_due_to_regex{org_id="%s"}[%s]))`)
	sslExpiry := byWindow(`min by (monitor_id) (min_over_time(probe_ssl_earliest_cert_expiry{org_id="%s"}[%s]))`)

	for _, series := range s.rng(ctx, fmt.Sprintf(`avg_over_time(probe_success{org_id="%s"}[%s])`, org, slot), start, end, step) {
		id := series.Labels["monitor_id"]
		ws := make([]Window, len(series.Points))
		for i, p := range series.Points {
			k := p.T.Unix()
			ws[i] = Window{
				T: p.T, V: p.V,
				Checks:        int(checks[id][k]),
				AvgMs:         round2(avgMs[id][k]),
				CodeMin:       int(codeMin[id][k]),
				CodeMax:       int(codeMax[id][k]),
				KeywordFailed: keyword[id][k] > 0,
				SSLExpiry:     int64(sslExpiry[id][k]),
			}
		}
		o.Monitors = append(o.Monitors, MonitorUptime{
			MonitorID:     id,
			MonitorName:   series.Labels["monitor_name"],
			Target:        series.Labels["instance"],
			AvgResponseMs: respByID[id],
			Points:        ws,
		})
	}

	// Heartbeats aren't probed: a window is a miss if the HeartbeatMissed rule
	// fired in it, and pings counts how often the last-ping gauge moved.
	missed := byWindow(`max by (monitor_id) (max_over_time(ALERTS{org_id="%s",alertname="HeartbeatMissed",alertstate="firing"}[%s]))`)
	for _, series := range s.rng(ctx, fmt.Sprintf(`sum by (monitor_id) (changes(beacon_heartbeat_last_ping_timestamp_seconds{org_id="%s"}[%s]))`, org, slot), start, end, step) {
		id := series.Labels["monitor_id"]
		ws := make([]Window, len(series.Points))
		for i, p := range series.Points {
			v := 1.0
			if missed[id][p.T.Unix()] > 0 {
				v = 0
			}
			ws[i] = Window{T: p.T, V: v, Pings: int(p.V)}
		}
		o.Monitors = append(o.Monitors, MonitorUptime{MonitorID: id, Points: ws})
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

// promSeconds renders a duration as an exact PromQL range in seconds — window
// slots are often not whole minutes (1h / 48 = 75s).
func promSeconds(d time.Duration) string {
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
