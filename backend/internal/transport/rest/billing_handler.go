package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/billing"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// BillingHandler exposes the plan catalog and plan-change endpoint.
type BillingHandler struct {
	svc       *billing.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
}

// NewBillingHandler builds a BillingHandler.
func NewBillingHandler(svc *billing.Service, v *validate.Validator, a *middleware.Authenticator) *BillingHandler {
	return &BillingHandler{svc: svc, validator: v, auth: a}
}

// Routes returns the authenticated billing routes.
func (h *BillingHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.Require)
	r.Get("/", h.get)
	r.With(h.auth.RequireWriter).Post("/plan", h.changePlan)
	return r
}

type planInfoResponse struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	PriceMonthly       int      `json:"price_monthly"`
	MaxMonitors        int      `json:"max_monitors"`
	MinIntervalSeconds int      `json:"min_interval_seconds"`
	Features           []string `json:"features"`
}

type billingResponse struct {
	CurrentPlan string             `json:"current_plan"`
	Plans       []planInfoResponse `json:"plans"`
}

type changePlanRequest struct {
	Plan string `json:"plan" validate:"required,oneof=free starter pro"`
}

func presentCatalog(items []plan.Info) []planInfoResponse {
	out := make([]planInfoResponse, 0, len(items))
	for _, p := range items {
		out = append(out, planInfoResponse{
			ID:                 string(p.Plan),
			Name:               p.Name,
			PriceMonthly:       p.PriceMonthly,
			MaxMonitors:        p.Limits.MaxMonitors,
			MinIntervalSeconds: p.Limits.MinIntervalSeconds,
			Features:           p.Features,
		})
	}
	return out
}

func billingActor(r *http.Request) billing.Actor {
	p := mustPrincipal(r)
	return billing.Actor{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role}
}

func (h *BillingHandler) get(w http.ResponseWriter, r *http.Request) {
	current, err := h.svc.Current(r.Context(), billingActor(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, billingResponse{
		CurrentPlan: string(current),
		Plans:       presentCatalog(h.svc.Catalog()),
	})
}

func (h *BillingHandler) changePlan(w http.ResponseWriter, r *http.Request) {
	var req changePlanRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	newPlan, err := h.svc.ChangePlan(r.Context(), billingActor(r), plan.Plan(req.Plan))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, map[string]any{"current_plan": string(newPlan)})
}
