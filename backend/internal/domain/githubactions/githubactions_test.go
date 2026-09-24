package githubactions

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/domain/monitor"
	"beacon/internal/platform/crypto"
)

// fakeRepo is an in-memory Repository: one monitor, resolvable by its token hash,
// recording the last status written.
type fakeRepo struct {
	hash       string
	m          *monitor.Monitor
	lastStatus monitor.Status
	applied    int
}

func (f *fakeRepo) GitHubByTokenHash(_ context.Context, hash string) (*monitor.Monitor, error) {
	if hash == f.hash {
		return f.m, nil
	}
	return nil, nil
}

func (f *fakeRepo) ApplyStatusUpdates(_ context.Context, updates []monitor.StatusUpdate) (int64, error) {
	for _, u := range updates {
		f.lastStatus = u.Status
		f.applied++
	}
	return int64(len(updates)), nil
}

func newFixture(m *monitor.Monitor) (*Service, *fakeRepo, string) {
	const token = "bpgh_testtoken"
	repo := &fakeRepo{hash: crypto.SHA256Hex(token), m: m}
	return NewService(repo), repo, token
}

func baseMonitor() *monitor.Monitor {
	return &monitor.Monitor{
		ID:         uuid.New(),
		OrgID:      uuid.New(),
		Name:       "CI",
		Type:       monitor.TypeGitHubActions,
		Target:     "bwalia/beaconpulse",
		Enabled:    true,
		LastStatus: monitor.StatusUnknown,
	}
}

func TestIngest_UnknownToken(t *testing.T) {
	svc, _, _ := newFixture(baseMonitor())
	if _, err := svc.Ingest(context.Background(), "bpgh_nope", Event{Status: "failure"}); err == nil {
		t.Fatal("expected NotFound for unknown token, got nil")
	}
}

func TestIngest_FailureFires(t *testing.T) {
	svc, repo, token := newFixture(baseMonitor())
	res, err := svc.Ingest(context.Background(), token, Event{Status: "failure", Workflow: "CI"})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Alert != AlertFiring {
		t.Fatalf("want AlertFiring, got %v", res.Alert)
	}
	if repo.lastStatus != monitor.StatusDown {
		t.Fatalf("want status down applied, got %q", repo.lastStatus)
	}
}

func TestIngest_SuccessAfterFailureResolves(t *testing.T) {
	m := baseMonitor()
	m.LastStatus = monitor.StatusDown
	svc, _, token := newFixture(m)
	res, err := svc.Ingest(context.Background(), token, Event{Status: "success"})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Alert != AlertResolved {
		t.Fatalf("want AlertResolved, got %v", res.Alert)
	}
}

func TestIngest_SuccessWhenUpIsQuiet(t *testing.T) {
	m := baseMonitor()
	m.LastStatus = monitor.StatusUp
	svc, _, token := newFixture(m)
	res, err := svc.Ingest(context.Background(), token, Event{Status: "success"})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Alert != AlertNone {
		t.Fatalf("want AlertNone for a routine green run, got %v", res.Alert)
	}
}

func TestIngest_CancelledIsIgnored(t *testing.T) {
	svc, repo, token := newFixture(baseMonitor())
	res, err := svc.Ingest(context.Background(), token, Event{Status: "cancelled"})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if !res.Ignored || res.Alert != AlertNone {
		t.Fatalf("want ignored/no-alert for cancelled, got ignored=%v alert=%v", res.Ignored, res.Alert)
	}
	if repo.applied != 0 {
		t.Fatalf("cancelled must not change status, but %d update(s) applied", repo.applied)
	}
}

func TestIngest_WorkflowFilterMismatchIgnored(t *testing.T) {
	m := baseMonitor()
	m.Settings.GitHubWorkflow = "Deploy"
	svc, _, token := newFixture(m)
	res, err := svc.Ingest(context.Background(), token, Event{Status: "failure", Workflow: "CI"})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if !res.Ignored || res.Alert != AlertNone {
		t.Fatalf("want a non-matching workflow ignored, got ignored=%v alert=%v", res.Ignored, res.Alert)
	}
}

func TestIngest_PausedMonitorIsQuiet(t *testing.T) {
	m := baseMonitor()
	m.Enabled = false
	svc, _, token := newFixture(m)
	res, err := svc.Ingest(context.Background(), token, Event{Status: "failure", Workflow: "CI"})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if !res.Ignored || res.Alert != AlertNone {
		t.Fatalf("want a paused monitor quiet, got ignored=%v alert=%v", res.Ignored, res.Alert)
	}
}
