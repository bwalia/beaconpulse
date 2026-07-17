// Package stripe adapts the Stripe SDK to the billing domain's Payments interface
// and normalizes Stripe webhooks into billing.WebhookEvent. It is the ONLY place
// the SDK is imported, so the domain stays provider-agnostic and testable.
package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/checkout/session"
	"github.com/stripe/stripe-go/v86/customer"
	"github.com/stripe/stripe-go/v86/event"
	"github.com/stripe/stripe-go/v86/webhook"

	"beacon/internal/domain/billing"
	"beacon/internal/domain/plan"
)

// customerInvalid reports whether err is Stripe rejecting a checkout because the
// customer id we passed does not exist in this account. It's the signal the
// billing service uses to recreate the customer and retry, rather than 500. This
// happens after the Stripe account/key is switched (the stored customer belongs to
// the old account) or a customer is deleted in the Stripe dashboard.
func customerInvalid(err error) bool {
	var se *stripe.Error
	if !errors.As(err, &se) {
		return false
	}
	return se.Code == stripe.ErrorCodeResourceMissing &&
		(se.Param == "customer" || strings.Contains(strings.ToLower(se.Msg), "customer"))
}

// Client implements billing.Payments over Stripe.
type Client struct {
	priceStarter  string
	pricePro      string
	successURL    string
	cancelURL     string
	webhookSecret string
}

// Config holds what the adapter needs; the secret key is applied globally.
type Config struct {
	SecretKey     string
	PriceStarter  string
	PricePro      string
	SuccessURL    string
	CancelURL     string
	WebhookSecret string
}

// New sets the global Stripe API key and returns a Client.
func New(cfg Config) *Client {
	stripe.Key = cfg.SecretKey
	return &Client{
		priceStarter:  cfg.PriceStarter,
		pricePro:      cfg.PricePro,
		successURL:    cfg.SuccessURL,
		cancelURL:     cfg.CancelURL,
		webhookSecret: cfg.WebhookSecret,
	}
}

var _ billing.Payments = (*Client)(nil)

// Configured reports whether tier p has a Stripe price wired.
func (c *Client) Configured(p plan.Plan) bool {
	switch p {
	case plan.Starter:
		return c.priceStarter != ""
	case plan.Pro:
		return c.pricePro != ""
	default:
		return false
	}
}

// EnsureCustomer creates a Stripe customer tagged with the org id. The domain only
// calls this when the org has no stored customer, so this always creates one.
func (c *Client) EnsureCustomer(ctx context.Context, orgID uuid.UUID, email string) (string, error) {
	params := &stripe.CustomerParams{}
	params.Context = ctx
	if email != "" {
		params.Email = stripe.String(email)
	}
	params.AddMetadata("org_id", orgID.String())
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe create customer: %w", err)
	}
	return cust.ID, nil
}

// SubscriptionCheckoutURL creates a recurring Checkout session for a tier.
func (c *Client) SubscriptionCheckoutURL(ctx context.Context, in billing.CheckoutInput) (string, error) {
	price, err := c.priceFor(in.Plan)
	if err != nil {
		return "", err
	}
	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:   stripe.String(in.CustomerID),
		SuccessURL: stripe.String(c.successURL),
		CancelURL:  stripe.String(c.cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(price), Quantity: stripe.Int64(1)},
		},
	}
	params.Context = ctx
	params.AddMetadata("org_id", in.OrgID.String())
	// Stamp the org id on the subscription too, so subscription.* webhooks resolve
	// the org without a customer→org lookup.
	params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{}
	params.SubscriptionData.AddMetadata("org_id", in.OrgID.String())
	sess, err := session.New(params)
	if err != nil {
		if customerInvalid(err) {
			return "", fmt.Errorf("stripe subscription checkout: %w", billing.ErrCustomerInvalid)
		}
		return "", fmt.Errorf("stripe subscription checkout: %w", err)
	}
	return sess.URL, nil
}

// TopUpCheckoutURL creates a one-time Checkout session for a custom amount.
func (c *Client) TopUpCheckoutURL(ctx context.Context, in billing.TopUpInput) (string, error) {
	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		Customer:   stripe.String(in.CustomerID),
		SuccessURL: stripe.String(c.successURL),
		CancelURL:  stripe.String(c.cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("usd"),
					UnitAmount: stripe.Int64(in.AmountCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Beacon Pulse monitoring credit"),
					},
				},
			},
		},
	}
	params.Context = ctx
	params.AddMetadata("org_id", in.OrgID.String())
	params.AddMetadata("kind", "topup")
	sess, err := session.New(params)
	if err != nil {
		if customerInvalid(err) {
			return "", fmt.Errorf("stripe top-up checkout: %w", billing.ErrCustomerInvalid)
		}
		return "", fmt.Errorf("stripe top-up checkout: %w", err)
	}
	return sess.URL, nil
}

// ParseWebhook verifies the signature and normalizes the event. Unhandled event
// types return a KindIgnore event (never an error), so the endpoint can 200 them.
func (c *Client) ParseWebhook(payload []byte, sigHeader string) (billing.WebhookEvent, error) {
	ev, err := webhook.ConstructEvent(payload, sigHeader, c.webhookSecret)
	if err != nil {
		// ConstructEvent rejects more than bad signatures — notably an API version
		// mismatch between the endpoint's version and the one this SDK is pinned to.
		// Distinguish them: reporting a version mismatch as "invalid signature" sends
		// you hunting a secret that was never wrong.
		if !errors.Is(err, webhook.ErrNotSigned) && !errors.Is(err, webhook.ErrInvalidHeader) &&
			!errors.Is(err, webhook.ErrNoValidSignature) && !errors.Is(err, webhook.ErrTooOld) {
			return billing.WebhookEvent{}, fmt.Errorf("%w: %w", billing.ErrWebhookNotSignature, err)
		}
		return billing.WebhookEvent{}, fmt.Errorf("verify stripe signature: %w", err)
	}
	return c.normalize(ev)
}

// normalize turns a Stripe event into the domain's provider-neutral shape. It is
// deliberately separate from signature checking: an event we fetched from the Stripe
// API ourselves is already authentic (we asked, over TLS, with our secret key), and
// it must be interpreted the SAME way as one that arrived by webhook — otherwise the
// reconciler and the endpoint could disagree about what a payment meant.
func (c *Client) normalize(ev stripe.Event) (billing.WebhookEvent, error) {
	out := billing.WebhookEvent{ID: ev.ID, Kind: billing.KindIgnore}

	switch ev.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(ev.Data.Raw, &sess); err != nil {
			return out, fmt.Errorf("decode checkout session: %w", err)
		}
		// One-time pay-as-you-go top-ups are settled here. Subscriptions are handled
		// via the customer.subscription.* events (more complete lifecycle).
		if sess.Mode == stripe.CheckoutSessionModePayment && sess.Metadata["kind"] == "topup" {
			out.Kind = billing.KindTopUp
			out.OrgID = parseUUID(sess.Metadata["org_id"])
			out.AmountCents = sess.AmountTotal
		}

	case "customer.subscription.created",
		"customer.subscription.updated",
		"customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(ev.Data.Raw, &sub); err != nil {
			return out, fmt.Errorf("decode subscription: %w", err)
		}
		out.Kind = billing.KindSubscription
		out.OrgID = parseUUID(sub.Metadata["org_id"])
		out.Status = string(sub.Status)
		if ev.Type == "customer.subscription.deleted" {
			out.Status = "canceled"
		}
		out.PeriodEnd = periodEnd(&sub)
		if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
			out.Plan = c.planForPrice(sub.Items.Data[0].Price.ID)
		}
	}
	return out, nil
}

// RecentTopUps lists the completed top-up payments Stripe recorded since `since`,
// normalized exactly as the webhook path would produce them.
//
// This is the safety net under the webhook. A webhook is best-effort: if the endpoint
// is down, misconfigured, or — as actually happened — rejecting events over an API
// version mismatch, Stripe eventually stops retrying and the money is simply gone
// from the customer's card with nothing credited. Nothing in the system notices,
// because the system never heard about it. Asking Stripe what it *knows* happened,
// rather than waiting to be told, is the only way a payment cannot be silently lost.
//
// Replay is safe because the event id is the idempotency key that the webhook path
// already uses: anything Stripe did deliver is a no-op here.
func (c *Client) RecentTopUps(ctx context.Context, since time.Time) ([]billing.WebhookEvent, error) {
	params := &stripe.EventListParams{
		Type: stripe.String("checkout.session.completed"),
		// Ask only for what Stripe could NOT deliver — pending, or failed every
		// attempt. That is exactly the money-losing set, and it keeps this to a few
		// rows instead of every payment in the window.
		//
		// Sound because the endpoint returns 200 only AFTER the credit transaction
		// commits, so "delivered" really does imply "credited". An event still in
		// flight may also appear here; replaying it early is harmless, since the
		// event id already guards against crediting twice.
		DeliverySuccess: stripe.Bool(false),
	}
	params.Context = ctx
	// Stripe keeps events for 30 days, which is the real ceiling on how far back
	// this can ever look.
	params.CreatedRange = &stripe.RangeQueryParams{GreaterThanOrEqual: since.Unix()}

	var out []billing.WebhookEvent
	iter := event.List(params)
	for iter.Next() {
		ev, err := c.normalize(*iter.Event())
		if err != nil {
			// One malformed event must not stall reconciliation for every other
			// payment; skip it and keep going.
			continue
		}
		if ev.Kind == billing.KindTopUp {
			out = append(out, ev)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe list events: %w", err)
	}
	return out, nil
}

func (c *Client) priceFor(p plan.Plan) (string, error) {
	switch p {
	case plan.Starter:
		if c.priceStarter == "" {
			return "", fmt.Errorf("starter price id not configured")
		}
		return c.priceStarter, nil
	case plan.Pro:
		if c.pricePro == "" {
			return "", fmt.Errorf("pro price id not configured")
		}
		return c.pricePro, nil
	default:
		return "", fmt.Errorf("plan %q is not subscribable", p)
	}
}

func (c *Client) planForPrice(priceID string) plan.Plan {
	switch priceID {
	case c.pricePro:
		return plan.Pro
	case c.priceStarter:
		return plan.Starter
	default:
		return plan.Free
	}
}

func parseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// periodEnd reads the subscription's current period end (Unix seconds → UTC).
// Stripe moved current_period_end off the subscription and onto each item as of
// API version 2025-03-31, so it is read from the items here. Our subscriptions are
// always single-item (one tier price), and items of one subscription share a
// billing period, so the first item carrying a period is authoritative.
func periodEnd(sub *stripe.Subscription) time.Time {
	if sub.Items == nil {
		return time.Time{}
	}
	for _, it := range sub.Items.Data {
		if it != nil && it.CurrentPeriodEnd > 0 {
			return time.Unix(it.CurrentPeriodEnd, 0).UTC()
		}
	}
	return time.Time{}
}
