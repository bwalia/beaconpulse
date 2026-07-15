// Package stripe adapts the Stripe SDK to the billing domain's Payments interface
// and normalizes Stripe webhooks into billing.WebhookEvent. It is the ONLY place
// the SDK is imported, so the domain stays provider-agnostic and testable.
package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/checkout/session"
	"github.com/stripe/stripe-go/v79/customer"
	"github.com/stripe/stripe-go/v79/webhook"

	"beacon/internal/domain/billing"
	"beacon/internal/domain/plan"
)

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
		return "", fmt.Errorf("stripe top-up checkout: %w", err)
	}
	return sess.URL, nil
}

// ParseWebhook verifies the signature and normalizes the event. Unhandled event
// types return a KindIgnore event (never an error), so the endpoint can 200 them.
func (c *Client) ParseWebhook(payload []byte, sigHeader string) (billing.WebhookEvent, error) {
	ev, err := webhook.ConstructEvent(payload, sigHeader, c.webhookSecret)
	if err != nil {
		return billing.WebhookEvent{}, fmt.Errorf("verify stripe signature: %w", err)
	}
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
func periodEnd(sub *stripe.Subscription) time.Time {
	if sub.CurrentPeriodEnd > 0 {
		return time.Unix(sub.CurrentPeriodEnd, 0).UTC()
	}
	return time.Time{}
}
