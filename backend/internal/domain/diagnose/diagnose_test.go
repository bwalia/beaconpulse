package diagnose

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

type fakeMonitors struct {
	target, monitorType string
	err                 error
	gotOrg              uuid.UUID
}

func (f *fakeMonitors) TargetFor(_ context.Context, orgID, _ uuid.UUID) (string, string, error) {
	f.gotOrg = orgID
	return f.target, f.monitorType, f.err
}

type fakePlans struct{ p plan.Plan }

func (f fakePlans) Plan(context.Context, uuid.UUID) (plan.Plan, error) { return f.p, nil }

type fakeProber struct {
	ev     Evidence
	err    error
	called bool
}

func (f *fakeProber) Probe(context.Context, string, string) (Evidence, error) {
	f.called = true
	return f.ev, f.err
}

type fakeExplainer struct {
	analysis *Analysis
	err      error
	called   bool
}

func (f *fakeExplainer) Explain(context.Context, Evidence) (*Analysis, error) {
	f.called = true
	return f.analysis, f.err
}

func actor(org uuid.UUID) Actor {
	return Actor{UserID: uuid.New(), OrgID: org, Role: auth.RoleOwner}
}

// TestRun_FreePlanIsRefusedBeforeProbing is the gate, and the assertion that matters
// is not the error — it is that the prober never ran. A Free org must not be able to
// make our infrastructure open sockets to an arbitrary host, whatever the reply says.
func TestRun_FreePlanIsRefusedBeforeProbing(t *testing.T) {
	prober := &fakeProber{}
	svc := NewService(&fakeMonitors{target: "https://x.test", monitorType: "https"},
		fakePlans{plan.Free}, prober, &fakeExplainer{})

	_, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New())
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected a validation error for a Free org, got %v", err)
	}
	if prober.called {
		t.Fatal("a Free org made the server probe a target — the gate must come first")
	}
}

// TestRun_PayAsYouGoCounts: PAYG has no subscription but is unambiguously paying.
// Gating on the subscribed plan instead of the effective one would sell someone
// credit and then refuse them the feature it buys.
func TestRun_PayAsYouGoCounts(t *testing.T) {
	for _, p := range []plan.Plan{plan.PayAsYouGo, plan.Starter, plan.Pro} {
		t.Run(string(p), func(t *testing.T) {
			prober := &fakeProber{ev: Evidence{Target: "https://x.test"}}
			svc := NewService(&fakeMonitors{target: "https://x.test", monitorType: "https"},
				fakePlans{p}, prober, &fakeExplainer{analysis: &Analysis{Summary: "ok"}})

			if _, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New()); err != nil {
				t.Fatalf("plan %q should be allowed to diagnose: %v", p, err)
			}
			if !prober.called {
				t.Fatalf("plan %q did not reach the prober", p)
			}
		})
	}
}

// TestRun_ScopesTheLookupToTheCallersOrg — the org id must reach the query, so a
// guessed monitor id from another tenant is not found rather than diagnosed.
func TestRun_ScopesTheLookupToTheCallersOrg(t *testing.T) {
	org := uuid.New()
	monitors := &fakeMonitors{target: "https://x.test", monitorType: "https"}
	svc := NewService(monitors, fakePlans{plan.Pro}, &fakeProber{}, &fakeExplainer{})

	if _, err := svc.Run(context.Background(), actor(org), uuid.New()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if monitors.gotOrg != org {
		t.Fatalf("monitor lookup used org %s, want the caller's %s", monitors.gotOrg, org)
	}
}

// TestRun_AIFailureStillReturnsTheEvidence — the measurements are the part that
// cannot be guessed at. A model that is down, slow, or talking nonsense must cost the
// user prose, not facts: an expired certificate says the same thing unphrased.
func TestRun_AIFailureStillReturnsTheEvidence(t *testing.T) {
	ev := Evidence{Target: "https://x.test", TLS: TLSFinding{Attempted: true, Expired: true}}
	svc := NewService(&fakeMonitors{target: "https://x.test", monitorType: "https"},
		fakePlans{plan.Pro}, &fakeProber{ev: ev},
		&fakeExplainer{err: errors.New("model timed out")})

	out, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New())
	if err != nil {
		t.Fatalf("an AI failure must not fail the request: %v", err)
	}
	if out.Analysis != nil {
		t.Fatal("no analysis should be returned when the model failed")
	}
	if out.AnalysisError == "" {
		t.Fatal("the caller must be told why the prose is missing")
	}
	if !out.Evidence.TLS.Expired {
		t.Fatal("the evidence was lost along with the model's answer")
	}
}

// TestRun_WithoutAnExplainerReturnsEvidenceOnly — a deployment with probing but no
// model still answers with facts.
func TestRun_WithoutAnExplainerReturnsEvidenceOnly(t *testing.T) {
	svc := NewService(&fakeMonitors{target: "https://x.test", monitorType: "https"},
		fakePlans{plan.Pro}, &fakeProber{ev: Evidence{Target: "https://x.test"}}, nil)

	out, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.AnalysisError == "" {
		t.Fatal("expected an explanation of the missing analysis")
	}
	if out.Evidence.Target != "https://x.test" {
		t.Fatal("expected the evidence to be returned regardless")
	}
}
