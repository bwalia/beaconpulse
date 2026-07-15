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

// fakePay returns fixed URLs and records the last inputs.
type fakePay struct{ lastTopUp TopUpInput }

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
	if err := svc.ApplyWebhook(context.Background(), ev); err != nil {
		t.Fatalf("ApplyWebhook: %v", err)
	}
	if repo.credited != 36000 {
		t.Fatalf("credited = %d, want 36000", repo.credited)
	}
	// Duplicate delivery (same event id) must not credit again.
	if err := svc.ApplyWebhook(context.Background(), ev); err != nil {
		t.Fatalf("ApplyWebhook dup: %v", err)
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
