package diagnose

import (
	"context"
	"errors"
	"testing"
	"time"

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

type fakeMeter struct {
	balance   int64
	runs      int
	charged   int64
	refunded  int64
	recorded  int
}

func (m *fakeMeter) ChargeCredit(_ context.Context, _ uuid.UUID, cost int64) (bool, error) {
	if m.balance < cost {
		return false, nil
	}
	m.balance -= cost
	m.charged += cost
	return true, nil
}
func (m *fakeMeter) RefundCredit(_ context.Context, _ uuid.UUID, cost int64) error {
	m.balance += cost
	m.refunded += cost
	return nil
}
func (m *fakeMeter) CountRunsSince(context.Context, uuid.UUID, time.Time) (int, error) {
	return m.runs, nil
}
func (m *fakeMeter) RecordRun(context.Context, uuid.UUID, uuid.UUID, int64) error {
	m.recorded++
	return nil
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
		fakePlans{plan.Free}, prober, &fakeExplainer{}, &fakeMeter{balance: 1e9}, 1800)

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
				fakePlans{p}, prober, &fakeExplainer{analysis: &Analysis{Summary: "ok"}}, &fakeMeter{balance: 1e9}, 1800)

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
	svc := NewService(monitors, fakePlans{plan.Pro}, &fakeProber{}, &fakeExplainer{}, &fakeMeter{balance: 1e9}, 1800)

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
		&fakeExplainer{err: errors.New("model timed out")}, &fakeMeter{balance: 1e9}, 1800)

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
		fakePlans{plan.Pro}, &fakeProber{ev: Evidence{Target: "https://x.test"}}, nil, &fakeMeter{balance: 1e9}, 1800)

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


func metered(p plan.Plan, m *fakeMeter, explain Explainer) *Service {
	return NewService(&fakeMonitors{target: "https://x.test", monitorType: "https"},
		fakePlans{p}, &fakeProber{ev: Evidence{Target: "https://x.test"}}, explain, m, 1800)
}

func ok() *fakeExplainer { return &fakeExplainer{analysis: &Analysis{Summary: "cert expired"}} }

// TestRun_PayAsYouGoIsChargedPerDiagnosis — the credit is the meter, so a run has to
// move it.
func TestRun_PayAsYouGoIsChargedPerDiagnosis(t *testing.T) {
	m := &fakeMeter{balance: 5000}
	svc := metered(plan.PayAsYouGo, m, ok())

	if _, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.charged != 1800 {
		t.Fatalf("charged = %d, want 1800", m.charged)
	}
	if m.balance != 3200 {
		t.Fatalf("balance = %d, want 3200", m.balance)
	}
	if m.recorded != 1 {
		t.Fatalf("the run was not recorded")
	}
}

// TestRun_PayAsYouGoWithoutEnoughCreditIsRefusedBeforeTheWork — no credit, no GPU.
func TestRun_PayAsYouGoWithoutEnoughCreditIsRefused(t *testing.T) {
	m := &fakeMeter{balance: 100} // less than one diagnosis
	prober := &fakeProber{}
	svc := NewService(&fakeMonitors{target: "https://x.test", monitorType: "https"},
		fakePlans{plan.PayAsYouGo}, prober, ok(), m, 1800)

	_, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New())
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	if prober.called {
		t.Fatal("an org that could not pay still made the server probe")
	}
	if m.balance != 100 {
		t.Fatalf("balance moved on a refused run: %d", m.balance)
	}
}

// TestRun_RefundsWhenTheModelFails is the fairness property: they paid for a
// diagnosis and got a probe dump, so they must not be billed for one.
func TestRun_RefundsWhenTheModelFails(t *testing.T) {
	m := &fakeMeter{balance: 5000}
	svc := metered(plan.PayAsYouGo, m, &fakeExplainer{err: errors.New("model down")})

	out, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Analysis != nil {
		t.Fatal("no analysis expected")
	}
	if m.balance != 5000 {
		t.Fatalf("balance = %d, want 5000 — a failed diagnosis was charged for", m.balance)
	}
	if m.refunded != 1800 {
		t.Fatalf("refunded = %d, want 1800", m.refunded)
	}
	if m.recorded != 0 {
		t.Fatal("a failed diagnosis was written to the ledger")
	}
}

// TestRun_SubscriptionSpendsQuotaNotCredit — a subscriber already paid a flat fee;
// charging their leftover credit too would bill them twice for one feature.
func TestRun_SubscriptionSpendsQuotaNotCredit(t *testing.T) {
	m := &fakeMeter{balance: 5000, runs: 3}
	svc := metered(plan.Starter, m, ok())

	if _, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.charged != 0 || m.balance != 5000 {
		t.Fatalf("a subscriber's credit was charged: charged=%d balance=%d", m.charged, m.balance)
	}
	if m.recorded != 1 {
		t.Fatal("the run was not counted against the quota")
	}
}

// TestRun_SubscriptionOverQuotaIsRefused — and refused before any work.
func TestRun_SubscriptionOverQuotaIsRefused(t *testing.T) {
	limit := plan.LimitsFor(plan.Starter).MonthlyDiagnoses
	m := &fakeMeter{balance: 5000, runs: limit}
	prober := &fakeProber{}
	svc := NewService(&fakeMonitors{target: "https://x.test", monitorType: "https"},
		fakePlans{plan.Starter}, prober, ok(), m, 1800)

	_, err := svc.Run(context.Background(), actor(uuid.New()), uuid.New())
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected the quota to refuse, got %v", err)
	}
	if prober.called {
		t.Fatal("an over-quota org still made the server probe")
	}
}

// TestMonthStart pins the reset the quota message promises: the 1st, UTC.
// Lives here because this is the code that spends against it.
func TestMonthStart(t *testing.T) {
	got := plan.MonthStart(time.Date(2026, 7, 17, 5, 44, 0, 0, time.UTC))
	want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("monthStart = %v, want %v", got, want)
	}
}
