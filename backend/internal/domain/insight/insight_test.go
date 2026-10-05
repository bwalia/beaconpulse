package insight

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/monitor"
	"beacon/internal/platform/apperror"
)

type fakeQuerier struct {
	exprs   []string
	samples []Sample
}

func (f *fakeQuerier) Query(_ context.Context, expr string) ([]Sample, error) {
	f.exprs = append(f.exprs, expr)
	return f.samples, nil
}
func (f *fakeQuerier) QueryRange(_ context.Context, expr string, _, _ time.Time, _ time.Duration) ([]RangeSeries, error) {
	f.exprs = append(f.exprs, expr)
	return nil, nil
}

type fakeLookup struct{ owned bool }

func (f *fakeLookup) GetByID(_ context.Context, _, id uuid.UUID) (*monitor.Monitor, error) {
	if !f.owned {
		return nil, apperror.NotFound("monitor not found")
	}
	return &monitor.Monitor{ID: id}, nil
}

func TestActiveAlertsFiltersByOrg(t *testing.T) {
	org := uuid.New()
	q := &fakeQuerier{samples: []Sample{{Labels: map[string]string{
		"alertname": "MonitorDown", "severity": "critical",
		"monitor_id": "m1", "monitor_name": "Site", "monitor_type": "https", "instance": "https://x",
	}, Value: 1}}}
	svc := NewService(q, &fakeLookup{owned: true})

	alerts, err := svc.ActiveAlerts(context.Background(), org)
	if err != nil {
		t.Fatalf("ActiveAlerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].MonitorName != "Site" {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}
	// The org_id must appear in the PromQL selector so cross-tenant data cannot
	// leak.
	if !strings.Contains(q.exprs[0], org.String()) {
		t.Errorf("query %q does not scope by org_id %q", q.exprs[0], org)
	}
}

func TestMonitorMetricsRejectsForeignMonitor(t *testing.T) {
	q := &fakeQuerier{}
	svc := NewService(q, &fakeLookup{owned: false}) // monitor not owned by caller's org

	_, err := svc.MonitorMetrics(context.Background(), uuid.New(), uuid.New(), 24*time.Hour)
	if !apperror.IsCode(err, apperror.CodeNotFound) {
		t.Fatalf("expected not-found for a monitor owned by another org, got %v", err)
	}
	if len(q.exprs) != 0 {
		t.Errorf("expected no Prometheus query for an unauthorized monitor, got %d", len(q.exprs))
	}
}

func TestMonitorMetricsQueriesWhenOwned(t *testing.T) {
	q := &fakeQuerier{samples: []Sample{{Value: 99.9}}}
	svc := NewService(q, &fakeLookup{owned: true})

	m, err := svc.MonitorMetrics(context.Background(), uuid.New(), uuid.New(), 24*time.Hour)
	if err != nil {
		t.Fatalf("MonitorMetrics: %v", err)
	}
	if m.UptimePercent == 0 {
		t.Error("expected uptime to be populated")
	}
	if len(q.exprs) == 0 {
		t.Error("expected Prometheus to be queried for an owned monitor")
	}
	// Every query must be scoped to the monitor id.
	for _, e := range q.exprs {
		if !strings.Contains(e, "monitor_id=") {
			t.Errorf("query %q not scoped by monitor_id", e)
		}
	}
}

// rangeQuerier answers range queries by the metric function they start with.
type rangeQuerier struct{ byPrefix map[string][]RangeSeries }

func (r *rangeQuerier) Query(context.Context, string) ([]Sample, error) { return nil, nil }
func (r *rangeQuerier) QueryRange(_ context.Context, expr string, _, _ time.Time, _ time.Duration) ([]RangeSeries, error) {
	for prefix, s := range r.byPrefix {
		if strings.HasPrefix(expr, prefix) {
			return s, nil
		}
	}
	return nil, nil
}

// TestOverviewWindowsCarryCheckDetails: each strip slot aggregates every check in
// its window (so a mid-window outage can't hide), joined with the details a hover
// explains, and heartbeats get slots from their ping gauge and missed alerts.
func TestOverviewWindowsCarryCheckDetails(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0).UTC()
	one := func(id string, v float64) []RangeSeries {
		return []RangeSeries{{Labels: map[string]string{"monitor_id": id}, Points: []Point{{T: t0, V: v}}}}
	}
	q := &rangeQuerier{byPrefix: map[string][]RangeSeries{
		"avg_over_time(probe_success":     one("web", 0.95),
		"sum by (monitor_id) (count_over": one("web", 60),
		"max by (monitor_id) (max_over_time(probe_http_status_code": one("web", 503),
		"max by (monitor_id) (max_over_time(probe_failed_due":       one("web", 1),
		"sum by (monitor_id) (changes(":                             one("hb", 4),
		"max by (monitor_id) (max_over_time(ALERTS":                 one("hb", 1),
	}}
	o, err := NewService(q, &fakeLookup{}).Overview(context.Background(), uuid.New(), 24*time.Hour, 48)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Window{}
	for _, m := range o.Monitors {
		got[m.MonitorID] = m.Points[0]
	}
	if w := got["web"]; w.V != 0.95 || w.Checks != 60 || w.CodeMax != 503 || !w.KeywordFailed {
		t.Errorf("web window = %+v", w)
	}
	if w := got["hb"]; w.V != 0 || w.Pings != 4 {
		t.Errorf("heartbeat window = %+v, want missed with 4 pings", w)
	}
}
