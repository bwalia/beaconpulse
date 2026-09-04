package rest

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/domain/plan"
	"beacon/internal/domain/settings"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// userLookup resolves the caller's email for the platform-admin check. The JWT does
// not carry email (and API keys have no user at all), so it is read from the store.
type userLookup interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*auth.User, error)
}

// SettingsHandler exposes the platform-GLOBAL settings — pricing, limits and the
// premium allowlist. Restricted to platform operators: these values affect every
// tenant, so a tenant org-owner must not be able to reach them.
type SettingsHandler struct {
	svc       *settings.Service
	users     userLookup
	validator *validate.Validator
	auth      *middleware.Authenticator
}

// NewSettingsHandler builds a SettingsHandler.
func NewSettingsHandler(svc *settings.Service, users userLookup, v *validate.Validator, a *middleware.Authenticator) *SettingsHandler {
	return &SettingsHandler{svc: svc, users: users, validator: v, auth: a}
}

// Routes returns the platform-settings routes. Session-only (an API key must not
// reprice the platform) and platform-admin-gated inside each handler.
func (h *SettingsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.RequireSession)
	r.Get("/", h.get)
	r.Put("/", h.update)
	return r
}

var planDisplayName = map[string]string{"free": "Free", "starter": "Starter", "pro": "Pro"}

// cleanFeatures trims each bullet and drops blank ones, so a stray empty line in the
// admin textarea never becomes an empty bullet on the card.
func cleanFeatures(in []string) []string {
	out := make([]string, 0, len(in))
	for _, f := range in {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

type planSettingResponse struct {
	Plan               string   `json:"plan"`
	Name               string   `json:"name"`
	PriceMonthly       int      `json:"price_monthly"`
	MaxMonitors        int      `json:"max_monitors"`
	MinIntervalSeconds int      `json:"min_interval_seconds"`
	MonthlyDiagnoses   int      `json:"monthly_diagnoses"`
	// Tagline/Features are the raw stored copy (empty = using the built-in default),
	// so the admin form shows blanks with the default as placeholder.
	Tagline  string   `json:"tagline"`
	Features []string `json:"features"`
}

type settingsResponse struct {
	MonitorHoursPerDollar int                   `json:"monitor_hours_per_dollar"`
	Plans                 []planSettingResponse `json:"plans"`
	PremiumGrants         []string              `json:"premium_grants"`
	UpdatedAt             *time.Time            `json:"updated_at,omitempty"`
}

func presentSettings(s settings.Settings) settingsResponse {
	resp := settingsResponse{
		MonitorHoursPerDollar: s.MonitorHoursPerDollar,
		PremiumGrants:         s.PremiumGrants,
	}
	if resp.PremiumGrants == nil {
		resp.PremiumGrants = []string{}
	}
	for _, p := range s.Plans {
		feats := p.Features
		if feats == nil {
			feats = []string{}
		}
		resp.Plans = append(resp.Plans, planSettingResponse{
			Plan:               string(p.Plan),
			Name:               planDisplayName[string(p.Plan)],
			PriceMonthly:       p.PriceMonthly,
			MaxMonitors:        p.MaxMonitors,
			MinIntervalSeconds: p.MinIntervalSeconds,
			MonthlyDiagnoses:   p.MonthlyDiagnoses,
			Tagline:            p.Tagline,
			Features:           feats,
		})
	}
	if !s.UpdatedAt.IsZero() {
		resp.UpdatedAt = &s.UpdatedAt
	}
	return resp
}

// requirePlatformAdmin resolves the caller's email and confirms they are a platform
// operator. Any failure is a flat 403 so a tenant cannot probe who is on the list.
func (h *SettingsHandler) requirePlatformAdmin(r *http.Request) (string, error) {
	p := mustPrincipal(r)
	u, err := h.users.GetUserByID(r.Context(), p.UserID)
	if err != nil || !h.svc.IsPlatformAdmin(u.Email) {
		return "", apperror.Forbidden("only platform operators can view or change platform settings")
	}
	return u.Email, nil
}

func (h *SettingsHandler) get(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requirePlatformAdmin(r); err != nil {
		httpx.Error(w, r, err)
		return
	}
	s, err := h.svc.Get(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentSettings(s))
}

type planSettingRequest struct {
	Plan               string   `json:"plan" validate:"required,oneof=free starter pro"`
	PriceMonthly       int      `json:"price_monthly" validate:"gte=0,lte=1000000"`
	MaxMonitors        int      `json:"max_monitors" validate:"gte=1,lte=1000000"`
	MinIntervalSeconds int      `json:"min_interval_seconds" validate:"gte=5,lte=86400"`
	MonthlyDiagnoses   int      `json:"monthly_diagnoses" validate:"gte=0,lte=1000000"`
	Tagline            string   `json:"tagline" validate:"max=160"`
	Features           []string `json:"features" validate:"omitempty,max=8,dive,max=80"`
}

type updateSettingsRequest struct {
	MonitorHoursPerDollar int                  `json:"monitor_hours_per_dollar" validate:"gte=1,lte=1000000"`
	Plans                 []planSettingRequest `json:"plans" validate:"required,min=1,dive"`
	PremiumGrants         []string             `json:"premium_grants" validate:"omitempty,dive,max=254"`
}

func (h *SettingsHandler) update(w http.ResponseWriter, r *http.Request) {
	email, err := h.requirePlatformAdmin(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req updateSettingsRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	in := settings.Settings{
		MonitorHoursPerDollar: req.MonitorHoursPerDollar,
		PremiumGrants:         req.PremiumGrants,
	}
	for _, p := range req.Plans {
		in.Plans = append(in.Plans, settings.PlanConfig{
			Plan:               plan.Plan(p.Plan),
			PriceMonthly:       p.PriceMonthly,
			MaxMonitors:        p.MaxMonitors,
			MinIntervalSeconds: p.MinIntervalSeconds,
			MonthlyDiagnoses:   p.MonthlyDiagnoses,
			Tagline:            strings.TrimSpace(p.Tagline),
			Features:           cleanFeatures(p.Features),
		})
	}
	out, err := h.svc.Update(r.Context(), mustPrincipal(r).UserID, email, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentSettings(out))
}
