package rest

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/billing"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// StripeWebhook verifies and normalizes a Stripe webhook. Implemented by the
// stripe adapter; nil when billing is not configured.
type StripeWebhook interface {
	ParseWebhook(payload []byte, sigHeader string) (billing.WebhookEvent, error)
}

// BillingHandler exposes the billing overview, Stripe Checkout (subscription and
// pay-as-you-go), and the Stripe webhook.
type BillingHandler struct {
	svc       *billing.Service
	stripe    StripeWebhook
	validator *validate.Validator
	auth      *middleware.Authenticator
	// diagnosisCostSeconds is shown so the billing page can price the button before
	// it is pressed, rather than after.
	diagnosisCostSeconds int64
}

// NewBillingHandler builds a BillingHandler. stripe may be nil (billing disabled).
func NewBillingHandler(svc *billing.Service, stripe StripeWebhook, v *validate.Validator, a *middleware.Authenticator, diagnosisCostSeconds int64) *BillingHandler {
	return &BillingHandler{svc: svc, stripe: stripe, validator: v, auth: a, diagnosisCostSeconds: diagnosisCostSeconds}
}

// Routes returns the AUTHENTICATED billing routes. The webhook is mounted
// separately (unauthenticated, signature-verified) in server.go.
func (h *BillingHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.Require)
	r.Get("/", h.get)
	r.With(h.auth.RequireWriter).Post("/checkout/subscription", h.subscribe)
	r.With(h.auth.RequireWriter).Post("/checkout/topup", h.topup)
	return r
}

type planInfoResponse struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	PriceMonthly       int      `json:"price_monthly"`
	MaxMonitors        int      `json:"max_monitors"`
	MinIntervalSeconds int      `json:"min_interval_seconds"`
	Features           []string `json:"features"`
	// Subscribable is true only when this tier can be purchased right now (Stripe
	// configured and a price set). The UI disables the button otherwise.
	Subscribable bool `json:"subscribable"`
}

type billingResponse struct {
	// SubscribedPlan is the tier the org subscribed to; EffectivePlan is what
	// actually applies right now (may be payg or free even while subscribed==pro
	// if the subscription lapsed).
	SubscribedPlan        string             `json:"subscribed_plan"`
	EffectivePlan         string             `json:"effective_plan"`
	SubscriptionStatus    string             `json:"subscription_status"`
	PeriodEnd             *time.Time         `json:"period_end,omitempty"`
	CreditSeconds         int64              `json:"credit_seconds"`
	// Granted/Consumed answer "how long have I had, and how long is left?" — the
	// question a bare balance leaves people reconstructing from Stripe receipts.
	GrantedCreditSeconds  int64              `json:"granted_credit_seconds"`
	ConsumedCreditSeconds int64              `json:"consumed_credit_seconds"`
	MonthlyDiagnoses       int               `json:"monthly_diagnoses"`
	DiagnosesUsedThisMonth int               `json:"diagnoses_used_this_month"`
	DiagnosisCostSeconds  int64              `json:"diagnosis_cost_seconds"`
	MaxMonitors           int                `json:"max_monitors"`
	MonitorHoursPerDollar int                `json:"monitor_hours_per_dollar"`
	BillingEnabled        bool               `json:"billing_enabled"`
	Plans                 []planInfoResponse `json:"plans"`
}

func presentCatalog(items []plan.Info, subscribable func(plan.Plan) bool) []planInfoResponse {
	out := make([]planInfoResponse, 0, len(items))
	for _, p := range items {
		out = append(out, planInfoResponse{
			ID:                 string(p.Plan),
			Name:               p.Name,
			PriceMonthly:       p.PriceMonthly,
			MaxMonitors:        p.Limits.MaxMonitors,
			MinIntervalSeconds: p.Limits.MinIntervalSeconds,
			Features:           p.Features,
			Subscribable:       subscribable(p.Plan),
		})
	}
	return out
}

func billingActor(r *http.Request) billing.Actor {
	p := mustPrincipal(r)
	return billing.Actor{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role}
}

func (h *BillingHandler) get(w http.ResponseWriter, r *http.Request) {
	ov, err := h.svc.Overview(r.Context(), billingActor(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	resp := billingResponse{
		SubscribedPlan:        string(ov.SubscribedPlan),
		EffectivePlan:         string(ov.EffectivePlan),
		SubscriptionStatus:    ov.SubscriptionStatus,
		CreditSeconds:         ov.CreditSeconds,
		GrantedCreditSeconds:  ov.GrantedCreditSeconds,
		ConsumedCreditSeconds: ov.ConsumedCreditSeconds,
		MonthlyDiagnoses:       ov.Limits.MonthlyDiagnoses,
		DiagnosesUsedThisMonth: ov.DiagnosesUsedThisMonth,
		DiagnosisCostSeconds:  h.diagnosisCostSeconds,
		MaxMonitors:           ov.Limits.MaxMonitors,
		MonitorHoursPerDollar: h.svc.MonitorHoursPerDollar(),
		BillingEnabled:        h.svc.Enabled(),
		Plans:                 presentCatalog(h.svc.Catalog(), h.svc.Subscribable),
	}
	if !ov.PeriodEnd.IsZero() {
		resp.PeriodEnd = &ov.PeriodEnd
	}
	httpx.OK(w, resp)
}

type subscribeRequest struct {
	Plan string `json:"plan" validate:"required,oneof=starter pro"`
}

func (h *BillingHandler) subscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	url, err := h.svc.StartSubscription(r.Context(), billingActor(r), plan.Plan(req.Plan))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, map[string]any{"checkout_url": url})
}

type topUpRequest struct {
	// AmountCents is the top-up amount in US cents (min $1).
	AmountCents int64 `json:"amount_cents" validate:"required,gte=100,lte=1000000"`
}

func (h *BillingHandler) topup(w http.ResponseWriter, r *http.Request) {
	var req topUpRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	url, err := h.svc.StartTopUp(r.Context(), billingActor(r), req.AmountCents)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, map[string]any{"checkout_url": url})
}

// Webhook receives Stripe events. Unauthenticated (Stripe can't present a JWT):
// authenticity comes from the signature the adapter verifies against the webhook
// secret. Mounted at /api/v1/billing/webhook in server.go.
func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	if h.stripe == nil {
		httpx.Error(w, r, apperror.NotFound("billing is not configured"))
		return
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.Error(w, r, apperror.Validation("could not read request body"))
		return
	}
	ev, err := h.stripe.ParseWebhook(payload, r.Header.Get("Stripe-Signature"))
	if err != nil {
		// A bad signature is a 400 and stays opaque: the caller is unauthenticated, so
		// detail would only help someone forge one. A rejection that is NOT about the
		// signature (an API-version mismatch) is an operator misconfiguration we own —
		// name it, so it is not mistaken for a wrong secret and sent chasing a rotation.
		if errors.Is(err, billing.ErrWebhookNotSignature) {
			httpx.Error(w, r, apperror.Validation("webhook rejected: the endpoint's Stripe API version does not match this server's Stripe SDK"))
			return
		}
		httpx.Error(w, r, apperror.Validation("invalid webhook signature"))
		return
	}
	// 200 only once the credit has committed, never before: Stripe treats the status
	// as the record of whether we took the event, and the reconciler trusts that same
	// signal to decide what still needs repairing.
	if _, err := h.svc.ApplyWebhook(r.Context(), ev); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, map[string]any{"received": true})
}
