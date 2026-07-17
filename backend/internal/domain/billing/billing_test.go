package billing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

// fakeRepo records credit and lets tests simulate a duplicate webhook.
type fakeRepo struct {
	state    State
	credited int64
	seen     map[string]bool // stripe event ids already applied
}

func newFakeRepo(st State) *fakeRepo { return &fakeRepo{state: st, seen: map[string]bool{}} }

func (f *fakeRepo) State(context.Context, uuid.UUID) (State, error) { return f.state, nil }
func (f *fakeRepo) SetCustomerID(_ context.Context, _ uuid.UUID, id string) error {
	f.state.StripeCustomerID = id
	return nil
}
func (f *fakeRepo) ApplyTopUp(_ context.Context, _ uuid.UUID, addSeconds, _ int64, eventID string) (bool, error) {
	if f.seen[eventID] {
		return false, nil
	}
	f.seen[eventID] = true
	f.credited += addSeconds
	f.state.CreditSeconds += addSeconds
	return true, nil
}
func (f *fakeRepo) ApplySubscription(_ context.Context, _ uuid.UUID, p plan.Plan, status string, _ time.Time, eventID string) (bool, error) {
	if f.seen[eventID] {
		return false, nil
	}
	f.seen[eventID] = true
	f.state.Plan = p
	f.state.SubscriptionStatus = status
	return true, nil
}
func (f *fakeRepo) DeductCredit(context.Context, int64) error { return nil }

// fakePay returns fixed URLs and records the last inputs. undelivered stands in for
// the payments Stripe took but could not hand us; listCalls counts the polls.
type fakePay struct {
	lastTopUp   TopUpInput
	undelivered []WebhookEvent
	listCalls   int
	listErr     error
	since       time.Time
}

func (fakePay) Configured(plan.Plan) bool { return true }
func (fakePay) EnsureCustomer(context.Context, uuid.UUID, string) (string, error) {
	return "cus_x", nil
}
func (fakePay) SubscriptionCheckoutURL(context.Context, CheckoutInput) (string, error) {
	return "https://checkout/sub", nil
}
func (p *fakePay) TopUpCheckoutURL(_ context.Context, in TopUpInput) (string, error) {
	p.lastTopUp = in
	return "https://checkout/topup", nil
}
func (p *fakePay) RecentTopUps(_ context.Context, since time.Time) ([]WebhookEvent, error) {
	p.listCalls++
	p.since = since
	return p.undelivered, p.listErr
}

type noopRecorder struct{}

func (noopRecorder) Record(context.Context, audit.Entry) error { return nil }

func admin() Actor {
	return Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleOwner}
}

func TestEffectivePlan(t *testing.T) {
	tests := []struct {
		name string
		st   State
		want plan.Plan
	}{
		{"free by default", State{Plan: plan.Free}, plan.Free},
		{"active subscription grants its tier", State{Plan: plan.Pro, SubscriptionStatus: "active"}, plan.Pro},
		{"lapsed subscription falls back", State{Plan: plan.Pro, SubscriptionStatus: "canceled"}, plan.Free},
		{"credit grants pay-as-you-go", State{Plan: plan.Free, CreditSeconds: 100}, plan.PayAsYouGo},
		{"subscription beats credit", State{Plan: plan.Starter, SubscriptionStatus: "active", CreditSeconds: 100}, plan.Starter},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.st.Effective(); got != tc.want {
				t.Fatalf("Effective() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyWebhook_TopUpCreditsMonitorSeconds(t *testing.T) {
	repo := newFakeRepo(State{})
	svc := NewService(repo, &fakePay{}, noopRecorder{}, 5) // $1 = 5 monitor-hours
	org := uuid.New()

	// $2.00 → 2 × 5 × 3600 = 36000 monitor-seconds.
	ev := WebhookEvent{ID: "evt_1", Kind: KindTopUp, OrgID: org, AmountCents: 200}
	applied, err := svc.ApplyWebhook(context.Background(), ev)
	if err != nil {
		t.Fatalf("ApplyWebhook: %v", err)
	}
	if !applied {
		t.Fatal("first delivery should report applied")
	}
	if repo.credited != 36000 {
		t.Fatalf("credited = %d, want 36000", repo.credited)
	}
	// Duplicate delivery (same event id) must not credit again.
	applied, err = svc.ApplyWebhook(context.Background(), ev)
	if err != nil {
		t.Fatalf("ApplyWebhook dup: %v", err)
	}
	if applied {
		t.Fatal("duplicate delivery should report NOT applied")
	}
	if repo.credited != 36000 {
		t.Fatalf("duplicate webhook double-credited: %d", repo.credited)
	}
}

func TestStartTopUp_RejectsBelowMinimumAndNonAdmin(t *testing.T) {
	svc := NewService(newFakeRepo(State{}), &fakePay{}, noopRecorder{}, 5)

	if _, err := svc.StartTopUp(context.Background(), admin(), 50); !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation for < $1, got %v", err)
	}
	member := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleMember}
	if _, err := svc.StartTopUp(context.Background(), member, 500); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("expected forbidden for member, got %v", err)
	}
}

func TestBilling_DisabledWithoutStripe(t *testing.T) {
	svc := NewService(newFakeRepo(State{}), nil, noopRecorder{}, 5) // no payments
	if svc.Enabled() {
		t.Fatal("Enabled() should be false without a payment provider")
	}
	if _, err := svc.StartTopUp(context.Background(), admin(), 500); err == nil {
		t.Fatal("expected an error when billing is not configured")
	}
}

func TestCatalogHasThreePlans(t *testing.T) {
	svc := NewService(newFakeRepo(State{}), nil, noopRecorder{}, 5)
	if len(svc.Catalog()) != 3 {
		t.Fatalf("expected 3 plans, got %d", len(svc.Catalog()))
	}
}

// TestReconcile_CreditsAPaymentTheWebhookNeverDelivered is the whole point of the
// reconciler, and it is a real incident reduced to a test: a customer paid, the
// endpoint was rejecting events at that moment, Stripe eventually stopped trying,
// and the money was gone with nothing credited and nothing aware of it.
func TestReconcile_CreditsAPaymentTheWebhookNeverDelivered(t *testing.T) {
	repo := newFakeRepo(State{})
	org := uuid.New()
	pay := &fakePay{undelivered: []WebhookEvent{
		{ID: "evt_lost", Kind: KindTopUp, OrgID: org, AmountCents: 500}, // $5
	}}
	svc := NewService(repo, pay, noopRecorder{}, 5) // $1 = 5 monitor-hours

	repaired, err := svc.Reconcile(context.Background(), time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if repaired != 1 {
		t.Fatalf("repaired = %d, want 1", repaired)
	}
	// $5 × 5 monitor-hours × 3600 = 90000 monitor-seconds.
	if repo.credited != 90000 {
		t.Fatalf("credited = %d, want 90000 (the lost payment was not made good)", repo.credited)
	}
}

// TestReconcile_DoesNotDoubleCreditADeliveredPayment is the property that makes
// polling safe to run alongside the webhook. If a replay could credit twice, the
// safety net would be worse than the hole it covers — customers would be handed free
// balance every pass.
func TestReconcile_DoesNotDoubleCreditADeliveredPayment(t *testing.T) {
	repo := newFakeRepo(State{})
	org := uuid.New()
	ev := WebhookEvent{ID: "evt_dup", Kind: KindTopUp, OrgID: org, AmountCents: 500}
	pay := &fakePay{undelivered: []WebhookEvent{ev}}
	svc := NewService(repo, pay, noopRecorder{}, 5)

	// The webhook arrives first and credits.
	if _, err := svc.ApplyWebhook(context.Background(), ev); err != nil {
		t.Fatalf("ApplyWebhook: %v", err)
	}
	before := repo.credited

	// The reconciler then sees the same event (Stripe reporting it undelivered, or a
	// poll racing the delivery). It must change nothing.
	repaired, err := svc.Reconcile(context.Background(), time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if repaired != 0 {
		t.Fatalf("repaired = %d, want 0 — reconcile re-applied a delivered payment", repaired)
	}
	if repo.credited != before {
		t.Fatalf("double credited: %d -> %d", before, repo.credited)
	}

	// And it stays stable however many times it runs.
	for i := 0; i < 3; i++ {
		if _, err := svc.Reconcile(context.Background(), time.Now().Add(-72*time.Hour)); err != nil {
			t.Fatalf("Reconcile pass %d: %v", i, err)
		}
	}
	if repo.credited != before {
		t.Fatalf("repeated reconciles drifted the balance: %d -> %d", before, repo.credited)
	}
}

// TestReconcile_SurvivesOneBadEvent — a single unusable event must not strand every
// other customer's payment behind it.
func TestReconcile_SurvivesOneBadEvent(t *testing.T) {
	repo := newFakeRepo(State{})
	good := uuid.New()
	pay := &fakePay{undelivered: []WebhookEvent{
		{ID: "evt_no_org", Kind: KindTopUp, OrgID: uuid.Nil, AmountCents: 500}, // unattributable
		{ID: "evt_good", Kind: KindTopUp, OrgID: good, AmountCents: 500},
	}}
	svc := NewService(repo, pay, noopRecorder{}, 5)

	repaired, err := svc.Reconcile(context.Background(), time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if repaired != 1 {
		t.Fatalf("repaired = %d, want 1 (the good payment behind the bad one was skipped)", repaired)
	}
	if repo.credited != 90000 {
		t.Fatalf("credited = %d, want 90000", repo.credited)
	}
}

// TestReconcile_NoopWithoutStripe — a deployment that sells nothing must still boot
// and run every other worker task.
func TestReconcile_NoopWithoutStripe(t *testing.T) {
	svc := NewService(newFakeRepo(State{}), nil, noopRecorder{}, 5)
	repaired, err := svc.Reconcile(context.Background(), time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatalf("Reconcile without payments should be a no-op, got %v", err)
	}
	if repaired != 0 {
		t.Fatalf("repaired = %d, want 0", repaired)
	}
}
