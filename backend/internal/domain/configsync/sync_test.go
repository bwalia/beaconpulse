package configsync

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/domain/monitor"
	"beacon/internal/domain/project"
)

// fakeMonitors is an in-memory stand-in for the monitor service, recording writes so a
// test can assert that a re-run performed none.
type fakeMonitors struct {
	items      []monitor.Monitor
	creates    int
	updates    int
	deletes    int
	createErr  error
	projectFor uuid.UUID
}

func (f *fakeMonitors) List(_ context.Context, _ monitor.Actor, _ monitor.ListFilter) ([]monitor.Monitor, int, error) {
	return f.items, len(f.items), nil
}
func (f *fakeMonitors) Create(_ context.Context, _ monitor.Actor, in monitor.CreateInput) (*monitor.Monitor, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.creates++
	m := monitor.Monitor{
		ID: uuid.New(), ProjectID: in.ProjectID, Name: in.Name, Type: in.Type,
		Target: in.Target, IntervalSeconds: in.IntervalSeconds, Settings: in.Settings,
	}
	if in.Enabled != nil {
		m.Enabled = *in.Enabled
	}
	f.items = append(f.items, m)
	return &m, nil
}
func (f *fakeMonitors) Update(_ context.Context, _ monitor.Actor, id uuid.UUID, in monitor.UpdateInput) (*monitor.Monitor, error) {
	f.updates++
	for i := range f.items {
		if f.items[i].ID == id {
			if in.Target != nil {
				f.items[i].Target = *in.Target
			}
			if in.IntervalSeconds != nil {
				f.items[i].IntervalSeconds = *in.IntervalSeconds
			}
			return &f.items[i], nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeMonitors) Delete(_ context.Context, _ monitor.Actor, id uuid.UUID) error {
	f.deletes++
	out := f.items[:0]
	for _, m := range f.items {
		if m.ID != id {
			out = append(out, m)
		}
	}
	f.items = out
	return nil
}

type fakeProjects struct {
	items   []project.Project
	creates int
}

func (f *fakeProjects) List(context.Context, project.Actor, project.ListFilter) ([]project.Project, int, error) {
	return f.items, len(f.items), nil
}
func (f *fakeProjects) Create(_ context.Context, _ project.Actor, in project.CreateInput) (*project.Project, error) {
	f.creates++
	p := project.Project{ID: uuid.New(), Name: in.Name}
	f.items = append(f.items, p)
	return &p, nil
}

func actor() Actor {
	return Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleOwner}
}

func decl(names ...string) []DesiredMonitor {
	out := make([]DesiredMonitor, 0, len(names))
	for _, n := range names {
		out = append(out, DesiredMonitor{Name: n, Type: "https", Target: "https://" + n, IntervalSeconds: 60})
	}
	return out
}

// TestApplyIsIdempotent is the reason this package exists. A workflow re-runs on every
// push, retry and merge; if the second run created anything, a fortnight of pushes
// would leave forty copies of the same domain, each probed and each billed.
func TestApplyIsIdempotent(t *testing.T) {
	mons, projs := &fakeMonitors{}, &fakeProjects{}
	s := NewService(mons, projs)
	a := actor()
	in := Input{Project: "prod", Monitors: decl("api.example.com", "www.example.com")}

	first, err := s.Apply(context.Background(), a, in)
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if first.Created != 2 {
		t.Fatalf("first run created %d, want 2", first.Created)
	}

	second, err := s.Apply(context.Background(), a, in)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if second.Created != 0 || second.Updated != 0 {
		t.Fatalf("re-running the identical declaration changed things: created=%d updated=%d",
			second.Created, second.Updated)
	}
	if second.Unchanged != 2 {
		t.Fatalf("unchanged = %d, want 2", second.Unchanged)
	}
	// The strongest form of the property: no writes reached the service at all, so a
	// re-run costs nothing and triggers no control-plane reload.
	if mons.creates != 2 || mons.updates != 0 {
		t.Fatalf("a no-op re-run still wrote: creates=%d updates=%d", mons.creates, mons.updates)
	}
}

// TestApplyUpdatesOnlyWhatChanged — editing one line in a file must not rewrite every
// monitor in it.
func TestApplyUpdatesOnlyWhatChanged(t *testing.T) {
	mons, projs := &fakeMonitors{}, &fakeProjects{}
	s := NewService(mons, projs)
	a := actor()

	if _, err := s.Apply(context.Background(), a, Input{Project: "p", Monitors: decl("a.test", "b.test")}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	edited := decl("a.test", "b.test")
	edited[1].IntervalSeconds = 300 // one field, one monitor

	res, err := s.Apply(context.Background(), a, Input{Project: "p", Monitors: edited})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Updated != 1 || res.Unchanged != 1 {
		t.Fatalf("updated=%d unchanged=%d, want 1 and 1", res.Updated, res.Unchanged)
	}
}

// TestPruneIsOffByDefault is the safety property. A workflow with a broken glob or a
// failed template step declares nothing — and if that deleted everything, monitoring
// would vanish at the exact moment nobody is looking.
func TestPruneIsOffByDefault(t *testing.T) {
	mons, projs := &fakeMonitors{}, &fakeProjects{}
	s := NewService(mons, projs)
	a := actor()

	if _, err := s.Apply(context.Background(), a, Input{Project: "p", Monitors: decl("keep.test", "gone.test")}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The catastrophic case: the declaration is empty.
	res, err := s.Apply(context.Background(), a, Input{Project: "p", Monitors: nil})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if mons.deletes != 0 {
		t.Fatalf("an empty declaration DELETED %d monitors without prune being asked for", mons.deletes)
	}
	if res.WouldRemove != 2 {
		t.Fatalf("would_remove = %d, want 2 — the user must be told what it would do", res.WouldRemove)
	}
	if len(mons.items) != 2 {
		t.Fatal("monitors were removed")
	}
}

// TestPruneRemovesOnlyWhenAsked — and then it must actually work, or the flag is a lie.
func TestPruneRemovesOnlyWhenAsked(t *testing.T) {
	mons, projs := &fakeMonitors{}, &fakeProjects{}
	s := NewService(mons, projs)
	a := actor()

	if _, err := s.Apply(context.Background(), a, Input{Project: "p", Monitors: decl("keep.test", "gone.test")}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	res, err := s.Apply(context.Background(), a, Input{Project: "p", Monitors: decl("keep.test"), Prune: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Removed != 1 || mons.deletes != 1 {
		t.Fatalf("removed=%d deletes=%d, want 1 and 1", res.Removed, mons.deletes)
	}
	if len(mons.items) != 1 || mons.items[0].Name != "keep.test" {
		t.Fatal("the wrong monitor was removed")
	}
}

// TestDryRunChangesNothing — what a pull request runs to show a plan before merging.
func TestDryRunChangesNothing(t *testing.T) {
	mons, projs := &fakeMonitors{}, &fakeProjects{}
	s := NewService(mons, projs)
	a := actor()

	res, err := s.Apply(context.Background(), a, Input{
		Project: "p", Monitors: decl("new.test"), Prune: true, DryRun: true,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("the plan should say 1 would be created, got %d", res.Created)
	}
	if mons.creates != 0 || mons.deletes != 0 || projs.creates != 0 {
		t.Fatalf("a dry run wrote: creates=%d deletes=%d projects=%d",
			mons.creates, mons.deletes, projs.creates)
	}
}

// TestOneBadMonitorDoesNotDiscardTheRest — a plan-limit rejection or a typo'd target on
// entry 3 must not throw away the ninety-nine that were fine, or the whole file becomes
// unappliable until it is perfect.
func TestOneBadMonitorDoesNotDiscardTheRest(t *testing.T) {
	mons := &fakeMonitors{createErr: errors.New("monitor limit reached on the free plan")}
	s := NewService(mons, &fakeProjects{})

	res, err := s.Apply(context.Background(), actor(), Input{Project: "p", Monitors: decl("a.test", "b.test")})
	if err != nil {
		t.Fatalf("a per-item failure must not fail the whole request: %v", err)
	}
	if res.Failed != 2 {
		t.Fatalf("failed = %d, want 2", res.Failed)
	}
	for _, it := range res.Items {
		if it.Action == "error" && it.Error == "" {
			t.Fatal("a failed item did not say why")
		}
	}
}

// TestDuplicateNamesAreRejected — two entries with one name would fight forever, each
// run overwriting the other and reporting a change that never converges.
func TestDuplicateNamesAreRejected(t *testing.T) {
	s := NewService(&fakeMonitors{}, &fakeProjects{})
	_, err := s.Apply(context.Background(), actor(), Input{
		Project:  "p",
		Monitors: []DesiredMonitor{{Name: "same", Type: "https", Target: "https://a"}, {Name: "SAME", Type: "https", Target: "https://b"}},
	})
	if err == nil {
		t.Fatal("duplicate names (differing only in case) were accepted")
	}
}

// TestViewerCannotSync — a read-only key must not be able to rewrite the monitor set.
func TestViewerCannotSync(t *testing.T) {
	s := NewService(&fakeMonitors{}, &fakeProjects{})
	viewer := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleViewer}
	if _, err := s.Apply(context.Background(), viewer, Input{Monitors: decl("x.test")}); err == nil {
		t.Fatal("a viewer applied a declaration")
	}
}

// TestResultIsDeterministic — CI output gets diffed against the previous run, and the
// removal pass iterates a map, whose order Go randomises.
func TestResultIsDeterministic(t *testing.T) {
	mons, projs := &fakeMonitors{}, &fakeProjects{}
	s := NewService(mons, projs)
	a := actor()
	if _, err := s.Apply(context.Background(), a, Input{Project: "p", Monitors: decl("c.test", "a.test", "b.test")}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var first []string
	for i := 0; i < 10; i++ {
		res, err := s.Apply(context.Background(), a, Input{Project: "p", Monitors: nil})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		var order []string
		for _, it := range res.Items {
			order = append(order, it.Name)
		}
		if first == nil {
			first = order
			continue
		}
		for j := range order {
			if order[j] != first[j] {
				t.Fatalf("item order changed between identical runs: %v then %v", first, order)
			}
		}
	}
}
