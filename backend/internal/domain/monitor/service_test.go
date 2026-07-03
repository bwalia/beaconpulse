package monitor

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

// ---- fakes ----

type fakeRepo struct {
	monitors      map[uuid.UUID]*Monitor
	projectExists bool
	syncedCreate  bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{monitors: map[uuid.UUID]*Monitor{}, projectExists: true}
}

func (f *fakeRepo) Create(_ context.Context, m *Monitor) error { f.monitors[m.ID] = m; return nil }
func (f *fakeRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (*Monitor, error) {
	m, ok := f.monitors[id]
	if !ok || m.OrgID != orgID {
		return nil, apperror.NotFound("monitor not found")
	}
	return m, nil
}
func (f *fakeRepo) List(_ context.Context, _ uuid.UUID, _ ListFilter) ([]Monitor, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Update(_ context.Context, m *Monitor) error { f.monitors[m.ID] = m; return nil }
func (f *fakeRepo) SoftDelete(_ context.Context, _, id, _ uuid.UUID) error {
	delete(f.monitors, id)
	return nil
}
func (f *fakeRepo) SetEnabled(_ context.Context, _, id uuid.UUID, enabled bool, _ uuid.UUID) error {
	if m, ok := f.monitors[id]; ok {
		m.Enabled = enabled
	}
	return nil
}
func (f *fakeRepo) ProjectExists(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return f.projectExists, nil
}
func (f *fakeRepo) ListAllEnabled(_ context.Context) ([]Monitor, error) { return nil, nil }
func (f *fakeRepo) ApplyStatusUpdates(_ context.Context, _ []StatusUpdate) (int64, error) {
	return 0, nil
}
func (f *fakeRepo) CountByOrg(_ context.Context, orgID uuid.UUID) (int, error) {
	n := 0
	for _, m := range f.monitors {
		if m.OrgID == orgID {
			n++
		}
	}
	return n, nil
}

type fakeSyncer struct{ calls int }

func (f *fakeSyncer) Sync(_ context.Context) error { f.calls++; return nil }

type fakePlanReader struct{ p plan.Plan }

func (f fakePlanReader) Plan(_ context.Context, _ uuid.UUID) (plan.Plan, error) { return f.p, nil }

type noopRecorder struct{}

func (noopRecorder) Record(_ context.Context, _ audit.Entry) error { return nil }

// newSvc builds a service with a generous (Pro) plan unless a test needs limits.
func newSvc(repo Repository, syncer Syncer) *Service {
	return NewService(repo, syncer, fakePlanReader{plan.Pro}, noopRecorder{})
}

func writerActor() Actor {
	return Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleAdmin}
}

// ---- tests ----

func TestCreateMonitorTriggersSync(t *testing.T) {
	repo := newFakeRepo()
	syncer := &fakeSyncer{}
	svc := newSvc(repo, syncer)
	actor := writerActor()

	m, err := svc.Create(context.Background(), actor, CreateInput{
		ProjectID: uuid.New(),
		Name:      "Site",
		Type:      TypeHTTPS,
		Target:    "example.com",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.Target != "https://example.com" {
		t.Errorf("target = %q", m.Target)
	}
	if m.IntervalSeconds != defaultInterval || m.TimeoutSeconds != defaultTimeout {
		t.Errorf("defaults not applied: interval=%d timeout=%d", m.IntervalSeconds, m.TimeoutSeconds)
	}
	if syncer.calls != 1 {
		t.Errorf("expected 1 sync call, got %d", syncer.calls)
	}
}

func TestCreateMonitorUnknownProject(t *testing.T) {
	repo := newFakeRepo()
	repo.projectExists = false
	svc := newSvc(repo, &fakeSyncer{})

	_, err := svc.Create(context.Background(), writerActor(), CreateInput{
		ProjectID: uuid.New(), Name: "x", Type: TypeHTTP, Target: "http://x.com",
	})
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateMonitorViewerForbidden(t *testing.T) {
	svc := newSvc(newFakeRepo(), &fakeSyncer{})
	actor := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleViewer}
	_, err := svc.Create(context.Background(), actor, CreateInput{
		ProjectID: uuid.New(), Name: "x", Type: TypeHTTP, Target: "http://x.com",
	})
	if !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCreateMonitorTimeoutExceedsInterval(t *testing.T) {
	svc := newSvc(newFakeRepo(), &fakeSyncer{})
	_, err := svc.Create(context.Background(), writerActor(), CreateInput{
		ProjectID:       uuid.New(),
		Name:            "x",
		Type:            TypeHTTP,
		Target:          "http://x.com",
		IntervalSeconds: 10,
		TimeoutSeconds:  30,
	})
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateMonitorQuotaExceeded(t *testing.T) {
	repo := newFakeRepo()
	org := uuid.New()
	// Fill the Free plan's monitor quota (10).
	limits := plan.LimitsFor(plan.Free)
	for i := 0; i < limits.MaxMonitors; i++ {
		id := uuid.New()
		repo.monitors[id] = &Monitor{ID: id, OrgID: org}
	}
	svc := NewService(repo, &fakeSyncer{}, fakePlanReader{plan.Free}, noopRecorder{})
	actor := Actor{UserID: uuid.New(), OrgID: org, Role: auth.RoleAdmin}

	_, err := svc.Create(context.Background(), actor, CreateInput{
		ProjectID: uuid.New(), Name: "one too many", Type: TypeHTTPS, Target: "example.com",
	})
	if !apperror.IsCode(err, apperror.CodeQuotaExceeded) {
		t.Fatalf("expected quota_exceeded, got %v", err)
	}
}

func TestCreateMonitorIntervalFloor(t *testing.T) {
	// Free plan floor is 60s; a 30s interval must be rejected.
	svc := NewService(newFakeRepo(), &fakeSyncer{}, fakePlanReader{plan.Free}, noopRecorder{})
	_, err := svc.Create(context.Background(), writerActor(), CreateInput{
		ProjectID:       uuid.New(),
		Name:            "too fast",
		Type:            TypeHTTPS,
		Target:          "example.com",
		IntervalSeconds: 30,
		TimeoutSeconds:  10,
	})
	if !apperror.IsCode(err, apperror.CodeQuotaExceeded) {
		t.Fatalf("expected quota_exceeded for sub-floor interval, got %v", err)
	}
}
