package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
)

// fakeRepo records the last created/updated window and lets tests assert on it
// without a database.
type fakeRepo struct {
	created   *Window
	suppress  bool
	suppErr   error
	lastAt    time.Time
	lastMonID uuid.UUID
}

func (f *fakeRepo) Create(_ context.Context, w *Window) error { f.created = w; return nil }
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*Window, error) {
	return nil, apperror.NotFound("nope")
}
func (f *fakeRepo) List(context.Context, uuid.UUID, ListFilter) ([]Window, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Update(context.Context, *Window) error                             { return nil }
func (f *fakeRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeRepo) ActiveForMonitor(_ context.Context, _ uuid.UUID, monitorID uuid.UUID, at time.Time) (bool, error) {
	f.lastMonID, f.lastAt = monitorID, at
	return f.suppress, f.suppErr
}

type noopRecorder struct{}

func (noopRecorder) Record(context.Context, audit.Entry) error { return nil }

func writer() Actor {
	return Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleAdmin}
}

func baseInput() CreateInput {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	return CreateInput{
		Title:    "Deploy",
		StartsAt: start,
		EndsAt:   start.Add(time.Hour),
		Scope:    ScopeOrg,
	}
}

func TestCreate_ValidationRejects(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(in *CreateInput)
	}{
		{"empty title", func(in *CreateInput) { in.Title = "" }},
		{"end before start", func(in *CreateInput) { in.EndsAt = start.Add(-time.Hour) }},
		{"end equals start", func(in *CreateInput) { in.EndsAt = start }},
		{"missing times", func(in *CreateInput) { in.StartsAt, in.EndsAt = time.Time{}, time.Time{} }},
		{"bad scope", func(in *CreateInput) { in.Scope = Scope("region") }},
		{"org scope with ids", func(in *CreateInput) { in.Scope = ScopeOrg; in.ScopeIDs = []uuid.UUID{uuid.New()} }},
		{"monitor scope without ids", func(in *CreateInput) { in.Scope = ScopeMonitor; in.ScopeIDs = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(&fakeRepo{}, noopRecorder{})
			in := baseInput()
			tc.mutate(&in)
			if _, err := svc.Create(context.Background(), writer(), in); err == nil {
				t.Fatal("expected a validation error, got nil")
			}
		})
	}
}

func TestCreate_ViewerForbidden(t *testing.T) {
	svc := NewService(&fakeRepo{}, noopRecorder{})
	viewer := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleViewer}
	if _, err := svc.Create(context.Background(), viewer, baseInput()); err == nil {
		t.Fatal("viewer must not be allowed to create a window")
	}
}

func TestCreate_DedupesScopeIDsAndStamps(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, noopRecorder{})
	dup := uuid.New()
	in := baseInput()
	in.Scope = ScopeMonitor
	in.ScopeIDs = []uuid.UUID{dup, dup, uuid.Nil, uuid.New()}

	w, err := svc.Create(context.Background(), writer(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(w.ScopeIDs) != 2 {
		t.Fatalf("ScopeIDs = %v, want 2 (dedup + drop nil)", w.ScopeIDs)
	}
	if repo.created == nil || repo.created.CreatedBy == nil || repo.created.CreatedAt.IsZero() {
		t.Fatal("Create did not stamp actor/timestamps before persisting")
	}
}

func TestIsSuppressed_PassesThroughToRepo(t *testing.T) {
	repo := &fakeRepo{suppress: true}
	svc := NewService(repo, noopRecorder{})
	org, mon := uuid.New(), uuid.New()
	at := time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC)

	got, err := svc.IsSuppressed(context.Background(), org, mon, at)
	if err != nil {
		t.Fatalf("IsSuppressed() error = %v", err)
	}
	if !got {
		t.Fatal("expected suppressed=true from the repo")
	}
	if repo.lastMonID != mon || !repo.lastAt.Equal(at) {
		t.Fatalf("repo called with (%v,%v), want (%v,%v)", repo.lastMonID, repo.lastAt, mon, at)
	}
}
