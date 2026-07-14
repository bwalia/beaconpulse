package statuspage

import (
	"context"
	"testing"

	"beacon/internal/domain/monitor"
)

func group(statuses ...monitor.Status) Group {
	g := Group{Name: "g"}
	for _, s := range statuses {
		g.Monitors = append(g.Monitors, Monitor{Name: "m", Status: s})
	}
	return g
}

// mon builds one projected monitor with an explicit maintenance flag; mixed wraps
// a set of them in a single group. Used for the maintenance-override cases.
func mon(s monitor.Status, maint bool) Monitor {
	return Monitor{Name: "m", Status: s, InMaintenance: maint}
}
func mixed(ms ...Monitor) []Group { return []Group{{Name: "g", Monitors: ms}} }

func TestSummarise(t *testing.T) {
	tests := []struct {
		name   string
		groups []Group
		want   Overall
	}{
		{"no monitors", nil, OverallUnknown},
		{"all up", []Group{group(monitor.StatusUp, monitor.StatusUp)}, OverallOperational},
		{"all down is a full outage", []Group{group(monitor.StatusDown, monitor.StatusDown)}, OverallOutage},
		{"one down among many is degraded, not an outage",
			[]Group{group(monitor.StatusUp, monitor.StatusUp, monitor.StatusDown)}, OverallDegraded},
		{"a degraded monitor degrades the whole page",
			[]Group{group(monitor.StatusUp, monitor.StatusDegraded)}, OverallDegraded},
		{"down in one group degrades across groups",
			[]Group{group(monitor.StatusUp), group(monitor.StatusDown)}, OverallDegraded},

		// The important one. A monitor that has never reported is NOT up. Counting
		// silence as health is how a status page ends up cheerfully claiming
		// "operational" through an outage.
		{"never-reported monitors are not counted as up",
			[]Group{group(monitor.StatusUnknown, monitor.StatusUnknown)}, OverallUnknown},
		{"one unknown among up is not operational",
			[]Group{group(monitor.StatusUp, monitor.StatusUnknown)}, OverallUnknown},

		// Maintenance overrides down in the HEADLINE: a covered monitor is excluded
		// from the up/down tally so planned work never reads as an outage.
		{"a down monitor under maintenance is not an outage",
			mixed(mon(monitor.StatusDown, true)), OverallMaintenance},
		{"planned work alongside healthy monitors reads as maintenance",
			mixed(mon(monitor.StatusUp, false), mon(monitor.StatusDown, true)), OverallMaintenance},
		{"a genuine outage still shows through a coincident window elsewhere",
			mixed(mon(monitor.StatusDown, false), mon(monitor.StatusDown, true)), OverallOutage},
		{"maintenance does not mask a real degrade on a live monitor",
			mixed(mon(monitor.StatusDegraded, false), mon(monitor.StatusUp, false), mon(monitor.StatusDown, true)), OverallDegraded},
		{"all monitors up with none in maintenance stays operational",
			mixed(mon(monitor.StatusUp, false), mon(monitor.StatusUp, false)), OverallOperational},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Summarise(tc.groups); got != tc.want {
				t.Errorf("Summarise() = %q, want %q", got, tc.want)
			}
		})
	}
}

// stubRepo lets us drive Service.Get without a database.
type stubRepo struct {
	page *Page
	err  error
}

func (s stubRepo) GetBySlug(context.Context, string) (*Page, error) { return s.page, s.err }

func TestServiceGet_UnpublishedIsIndistinguishableFromMissing(t *testing.T) {
	// The repo collapses "no such org" and "org exists but unpublished" to nil.
	// The service must pass that through as nil rather than inventing an empty
	// page, or the endpoint becomes an oracle for which orgs exist.
	svc := NewService(stubRepo{page: nil})

	got, err := svc.Get(context.Background(), "whoever")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != nil {
		t.Fatalf("Get() = %+v, want nil for an unpublished/missing slug", got)
	}
}

func TestServiceGet_TitleFallsBackToOrgName(t *testing.T) {
	svc := NewService(stubRepo{page: &Page{
		OrgName: "Acme",
		Title:   "", // never set one
		Groups:  []Group{group(monitor.StatusUp)},
	}})

	got, err := svc.Get(context.Background(), "acme")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Title != "Acme" {
		t.Errorf("Title = %q, want the org name as fallback", got.Title)
	}
	if got.Overall != OverallOperational {
		t.Errorf("Overall = %q, want %q", got.Overall, OverallOperational)
	}
}
