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
