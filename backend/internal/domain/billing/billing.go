// Package billing manages how an organization pays for monitoring. There are two
// ways to pay, both through Stripe, and they compose:
//
//   - Pay-as-you-go: buy any amount; it becomes a balance of MONITOR-SECONDS. A
//     worker deducts one second of credit per enabled monitor per second, so more
//     domains burn the balance faster. When it hits zero the org falls back to Free.
//   - Subscription: a recurring Stripe subscription for the Starter/Pro tiers.
//
// The EFFECTIVE plan — the limits that actually apply — is computed, never stored:
// the subscribed tier while its subscription is active, else pay-as-you-go while
// credit remains, else Free. The domain is provider-agnostic: Stripe lives behind
// the Payments interface, this package only knows money, credit and plans.
package billing

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

// ErrCustomerInvalid signals that the org's stored Stripe customer id no longer
// exists in the active Stripe account — typically after the account/key is
// switched, or the customer is deleted in Stripe. A Payments adapter returns it
// (wrapped) from a checkout call; the service recreates the customer under the
// current account and retries once instead of surfacing a 500.
var ErrCustomerInvalid = errors.New("stripe customer no longer exists")

// ErrWebhookNotSignature marks a webhook the provider rejected for a reason OTHER
// than a bad signature — in practice, the endpoint sending events built for a
// different API version than the SDK is pinned to. It exists so the endpoint can
// report that honestly: a version mismatch surfaced as "invalid signature" sends
// operators rotating a secret that was never wrong.
var ErrWebhookNotSignature = errors.New("webhook rejected (not a signature failure)")

// State is an org's billing snapshot.
type State struct {
	// Plan is the SUBSCRIBED tier (free by default); it is not the effective tier.
	Plan               plan.Plan
	SubscriptionStatus string // Stripe status, "" when never subscribed
	PeriodEnd          time.Time
	StripeCustomerID   string
	// CreditSeconds is the remaining pay-as-you-go balance, in monitor-seconds.
	CreditSeconds int64
}

// SubscriptionActive reports whether a paid subscription currently grants its tier.
func (s State) SubscriptionActive() bool {
	return s.SubscriptionStatus == "active" || s.SubscriptionStatus == "trialing"
}

// Effective is the plan whose limits actually apply right now.
func (s State) Effective() plan.Plan {
	return plan.Effective(s.Plan, s.SubscriptionActive(), s.CreditSeconds)
}

// Actor is the authenticated caller.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
	// Email is used to create the Stripe customer on first checkout (best-effort).
	Email string
}

// Repository persists billing state and the Stripe idempotency ledger.
type Repository interface {
	State(ctx context.Context, orgID uuid.UUID) (State, error)
	SetCustomerID(ctx context.Context, orgID uuid.UUID, customerID string) error
	// ApplyTopUp idempotently adds credit if eventID has not been seen; returns
	// whether it applied (false = duplicate webhook, already credited).
	ApplyTopUp(ctx context.Context, orgID uuid.UUID, addSeconds, amountCents int64, eventID string) (applied bool, err error)
	// ApplySubscription idempotently sets the org's subscribed tier + Stripe status.
	ApplySubscription(ctx context.Context, orgID uuid.UUID, p plan.Plan, status string, periodEnd time.Time, eventID string) (applied bool, err error)
	// DeductCredit burns `elapsedSeconds` × (enabled monitor count) from every org
	// that has credit, flooring at zero. This is the pay-as-you-go meter.
	DeductCredit(ctx context.Context, elapsedSeconds int64) error
	// CreditTotals reports credit ever granted and credit remaining, so the UI can
	// tell someone what they have SPENT rather than only what is left.
	CreditTotals(ctx context.Context, orgID uuid.UUID) (granted, remaining int64, err error)
}

// Payments is the payment provider (Stripe). Kept an interface so the domain and
// its tests never import the SDK.
type Payments interface {
	// Configured reports whether tier p can be subscribed to right now — i.e. its
	// price id is set. False for tiers the operator has not wired a price for.
	Configured(p plan.Plan) bool
	// EnsureCustomer returns the org's Stripe customer id, creating it if needed.
	EnsureCustomer(ctx context.Context, orgID uuid.UUID, email string) (string, error)
	// SubscriptionCheckoutURL returns a Stripe Checkout URL for a recurring tier.
	SubscriptionCheckoutURL(ctx context.Context, in CheckoutInput) (string, error)
	// TopUpCheckoutURL returns a Stripe Checkout URL for a one-time custom amount.
	TopUpCheckoutURL(ctx context.Context, in TopUpInput) (string, error)
	// RecentTopUps lists top-up payments the provider recorded since `since` that it
	// could not confirm delivering to us. It is what lets billing be pull-based as
	// well as push-based; see Service.Reconcile.
	RecentTopUps(ctx context.Context, since time.Time) ([]WebhookEvent, error)
}

// CheckoutInput / TopUpInput are what the service hands the payment provider.
type CheckoutInput struct {
	OrgID      uuid.UUID
	CustomerID string
	Plan       plan.Plan
}
type TopUpInput struct {
	OrgID       uuid.UUID
	CustomerID  string
	AmountCents int64
}

// Service implements billing use cases.
type Service struct {
	repo                  Repository
	pay                   Payments // nil when Stripe is not configured
	auditlog              audit.Recorder
	monitorHoursPerDollar int
}

// NewService wires the billing service. pay may be nil (Stripe disabled), in which
// case the checkout methods return a clear "not configured" error.
func NewService(repo Repository, pay Payments, auditlog audit.Recorder, monitorHoursPerDollar int) *Service {
	if monitorHoursPerDollar <= 0 {
		monitorHoursPerDollar = 5
	}
	return &Service{repo: repo, pay: pay, auditlog: auditlog, monitorHoursPerDollar: monitorHoursPerDollar}
}

// Catalog returns the subscribable plans for the pricing UI.
func (s *Service) Catalog() []plan.Info { return plan.Catalog() }

// MonitorHoursPerDollar exposes the pay-as-you-go rate for the UI.
func (s *Service) MonitorHoursPerDollar() int { return s.monitorHoursPerDollar }

// Enabled reports whether Stripe is wired up.
func (s *Service) Enabled() bool { return s.pay != nil }

// Subscribable reports whether tier p can be purchased as a subscription right now
// (Stripe configured and a price set). The UI uses it to avoid offering a dead
// button for a tier whose price the operator has not wired.
func (s *Service) Subscribable(p plan.Plan) bool {
	return s.pay != nil && (p == plan.Starter || p == plan.Pro) && s.pay.Configured(p)
}

// Overview is the customer-facing billing summary.
type Overview struct {
	SubscribedPlan     plan.Plan
	EffectivePlan      plan.Plan
	SubscriptionStatus string
	PeriodEnd          time.Time
	CreditSeconds      int64
	Limits             plan.Limits
	// GrantedCreditSeconds is everything ever bought; ConsumedCreditSeconds is what
	// monitoring has burned through. Both are surfaced because a balance alone does
	// not answer the question people actually ask — "how long have I had, and how
	// long do I have left?" — and leaves them reconstructing it from receipts.
	GrantedCreditSeconds  int64
	ConsumedCreditSeconds int64
}

// Overview returns the caller org's billing state.
func (s *Service) Overview(ctx context.Context, actor Actor) (Overview, error) {
	st, err := s.repo.State(ctx, actor.OrgID)
	if err != nil {
		return Overview{}, err
	}
	eff := st.Effective()
	// Best-effort: a balance is still worth showing if the totals query fails.
	granted, remaining, terr := s.repo.CreditTotals(ctx, actor.OrgID)
	consumed := int64(0)
	if terr == nil && granted > remaining {
		consumed = granted - remaining
	}
	return Overview{
		SubscribedPlan:        st.Plan,
		EffectivePlan:         eff,
		SubscriptionStatus:    st.SubscriptionStatus,
		PeriodEnd:             st.PeriodEnd,
		CreditSeconds:         st.CreditSeconds,
		Limits:                plan.LimitsFor(eff),
		GrantedCreditSeconds:  granted,
		ConsumedCreditSeconds: consumed,
	}, nil
}

// StartTopUp creates a Stripe Checkout session for a one-time pay-as-you-go top-up
// of amountCents and returns its URL. Admins/owners only (they hold the card).
func (s *Service) StartTopUp(ctx context.Context, actor Actor, amountCents int64) (string, error) {
	if err := s.guard(actor); err != nil {
		return "", err
	}
	if amountCents < 100 {
		return "", apperror.Validation("amount too small",
			apperror.FieldError{Field: "amount", Message: "minimum top-up is $1"})
	}
	return s.startCheckout(ctx, actor, func(cust string) (string, error) {
		return s.pay.TopUpCheckoutURL(ctx, TopUpInput{OrgID: actor.OrgID, CustomerID: cust, AmountCents: amountCents})
	})
}

// StartSubscription creates a Stripe Checkout session for a recurring tier.
func (s *Service) StartSubscription(ctx context.Context, actor Actor, p plan.Plan) (string, error) {
	if err := s.guard(actor); err != nil {
		return "", err
	}
	if p != plan.Starter && p != plan.Pro {
		return "", apperror.Validation("not a subscribable plan",
			apperror.FieldError{Field: "plan", Message: "must be starter or pro"})
	}
	if !s.pay.Configured(p) {
		return "", apperror.Validation("subscriptions are not set up for this plan yet",
			apperror.FieldError{Field: "plan", Message: "no Stripe price is configured — use pay-as-you-go, or ask an admin to set STRIPE_PRICE_*"})
	}
	return s.startCheckout(ctx, actor, func(cust string) (string, error) {
		return s.pay.SubscriptionCheckoutURL(ctx, CheckoutInput{OrgID: actor.OrgID, CustomerID: cust, Plan: p})
	})
}

// Kind is a normalized webhook event category the service knows how to apply.
type Kind int

const (
	// KindIgnore is an event we intentionally do not act on.
	KindIgnore Kind = iota
	// KindTopUp is a completed one-time pay-as-you-go payment.
	KindTopUp
	// KindSubscription is a subscription lifecycle change.
	KindSubscription
)

// WebhookEvent is a Stripe event normalized by the adapter for the service to
// apply. The adapter verifies the signature; the service only trusts this struct.
type WebhookEvent struct {
	ID   string // Stripe event id — the idempotency key
	Kind Kind

	// Top-up fields.
	OrgID       uuid.UUID
	AmountCents int64

	// Subscription fields.
	Plan      plan.Plan
	Status    string
	PeriodEnd time.Time
}

// ApplyWebhook applies a verified, normalized Stripe event idempotently.
// Reconcile credits any top-up the provider took money for but never managed to
// tell us about, and returns how many it had to repair.
//
// Webhooks are best-effort, and treating them as the only path means a delivery
// failure is indistinguishable from a payment that never happened: the money leaves
// the customer's card, nothing is credited, and nothing in the system is aware there
// is anything to notice. That is not hypothetical — it happened here, when the
// endpoint spent an hour rejecting events over an API version mismatch and a real
// payment was simply lost. So the provider is also polled: it is the authority on
// what was paid, and asking it closes the gap that waiting cannot.
//
// Replay is safe by construction. Every top-up is keyed by its provider event id and
// applied through exactly the path a webhook would take, so an event that did arrive
// is a no-op, a delivery racing this poll cannot double-credit, and running it more
// often costs nothing but a query.
//
// Only top-ups are reconciled. They are additive and idempotent, so replaying one
// late is always right. Subscription events are last-writer-wins state, where
// replaying a missed-but-older event could overwrite a newer one and downgrade a
// paying customer — worse than the gap it would close.
func (s *Service) Reconcile(ctx context.Context, since time.Time) (int, error) {
	if s.pay == nil {
		return 0, nil
	}
	events, err := s.pay.RecentTopUps(ctx, since)
	if err != nil {
		return 0, err
	}
	repaired := 0
	for _, ev := range events {
		applied, err := s.ApplyWebhook(ctx, ev)
		if err != nil {
			// One bad event must not stall the repair of every other customer's
			// payment; the next pass retries it anyway.
			continue
		}
		if applied {
			repaired++
		}
	}
	return repaired, nil
}

// ApplyWebhook settles one provider event. It reports whether the event actually
// changed anything: an event already applied — a Stripe retry, or the reconciler and
// a delivery arriving at once — is a no-op, and the caller can tell the difference
// between "handled" and "handled for the first time".
func (s *Service) ApplyWebhook(ctx context.Context, ev WebhookEvent) (bool, error) {
	switch ev.Kind {
	case KindTopUp:
		if ev.OrgID == uuid.Nil {
			return false, nil // no org to credit; ignore rather than error a webhook
		}
		addSeconds := ev.AmountCents * int64(s.monitorHoursPerDollar) * 3600 / 100
		applied, err := s.repo.ApplyTopUp(ctx, ev.OrgID, addSeconds, ev.AmountCents, ev.ID)
		if err != nil {
			return false, err
		}
		if applied {
			org := ev.OrgID
			_ = s.auditlog.Record(ctx, audit.Entry{
				OrgID: &org, Action: "billing.credit_added",
				ResourceType: "organization", ResourceID: org.String(),
				Metadata: map[string]any{"amount_cents": ev.AmountCents, "credit_seconds": addSeconds},
			})
		}
		return applied, nil
	case KindSubscription:
		if ev.OrgID == uuid.Nil {
			return false, nil
		}
		applied, err := s.repo.ApplySubscription(ctx, ev.OrgID, ev.Plan, ev.Status, ev.PeriodEnd, ev.ID)
		if err != nil {
			return false, err
		}
		if applied {
			org := ev.OrgID
			_ = s.auditlog.Record(ctx, audit.Entry{
				OrgID: &org, Action: "billing.subscription_updated",
				ResourceType: "organization", ResourceID: org.String(),
				Metadata: map[string]any{"plan": string(ev.Plan), "status": ev.Status},
			})
		}
		return applied, nil
	default:
		return false, nil
	}
}

// ConsumeCredit burns pay-as-you-go credit for the elapsed window. Called by the
// worker meter; safe to call with a zero balance (deducts nothing).
func (s *Service) ConsumeCredit(ctx context.Context, elapsedSeconds int64) error {
	return s.repo.DeductCredit(ctx, elapsedSeconds)
}

func (s *Service) guard(actor Actor) error {
	if s.pay == nil {
		return apperror.Validation("billing is not configured on this deployment")
	}
	if !actor.Role.CanAdminister() {
		return apperror.Forbidden("only owners and admins can manage billing")
	}
	return nil
}

// ensureCustomer returns the org's Stripe customer id, creating and persisting it
// on first use.
func (s *Service) ensureCustomer(ctx context.Context, actor Actor) (string, error) {
	st, err := s.repo.State(ctx, actor.OrgID)
	if err != nil {
		return "", err
	}
	if st.StripeCustomerID != "" {
		return st.StripeCustomerID, nil
	}
	cust, err := s.pay.EnsureCustomer(ctx, actor.OrgID, actor.Email)
	if err != nil {
		return "", err
	}
	if err := s.repo.SetCustomerID(ctx, actor.OrgID, cust); err != nil {
		return "", err
	}
	return cust, nil
}

// startCheckout runs a checkout that needs the org's Stripe customer, healing a
// stale customer id once. If Stripe rejects the stored customer as non-existent
// (ErrCustomerInvalid — e.g. the account was switched or the customer deleted in
// Stripe), it forgets that id, creates a fresh customer under the active account,
// and retries the checkout exactly once instead of returning a 500.
func (s *Service) startCheckout(ctx context.Context, actor Actor, checkout func(customerID string) (string, error)) (string, error) {
	cust, err := s.ensureCustomer(ctx, actor)
	if err != nil {
		return "", err
	}
	url, err := checkout(cust)
	if !errors.Is(err, ErrCustomerInvalid) {
		return url, err
	}
	fresh, err := s.pay.EnsureCustomer(ctx, actor.OrgID, actor.Email)
	if err != nil {
		return "", err
	}
	if err := s.repo.SetCustomerID(ctx, actor.OrgID, fresh); err != nil {
		return "", err
	}
	return checkout(fresh)
}
