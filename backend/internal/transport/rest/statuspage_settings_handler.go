package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/statuspage"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// StatusPageSettingsHandler exposes the AUTHENTICATED controls for an org's
// public status page: read the current settings, and publish/unpublish.
//
// Distinct from StatusPageHandler (which serves anonymous readers). Separate
// types, separate routes, separate services — so the write path can never be
// reached without a token by accident.
type StatusPageSettingsHandler struct {
	svc       *statuspage.SettingsService
	validator *validate.Validator
	auth      *middleware.Authenticator
}

// NewStatusPageSettingsHandler builds a StatusPageSettingsHandler.
func NewStatusPageSettingsHandler(svc *statuspage.SettingsService, v *validate.Validator, a *middleware.Authenticator) *StatusPageSettingsHandler {
	return &StatusPageSettingsHandler{svc: svc, validator: v, auth: a}
}

// Routes returns the authenticated status-page settings routes.
func (h *StatusPageSettingsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.Require)
	r.Get("/", h.get)
	// Publishing changes what the whole internet can see, so it needs a writer.
	r.With(h.auth.RequireWriter).Patch("/", h.update)
	return r
}

type statusPageSettingsRequest struct {
	Enabled *bool   `json:"enabled"`
	Title   *string `json:"title" validate:"omitempty,max=120"`
	// Slug is the custom public URL slug. Empty string clears it (back to the org
	// slug); omitted (null) leaves it unchanged. Format is enforced server-side
	// after normalisation, so we only bound the length here.
	Slug *string `json:"slug" validate:"omitempty,max=63"`
}

type statusPageSettingsResponse struct {
	Slug           string `json:"slug"`
	OrgName        string `json:"org_name"`
	Enabled        bool   `json:"enabled"`
	Title          string `json:"title"`
	PublishedCount int    `json:"published_count"`
	// OrgSlug is the default the page falls back to; CustomSlug is the override (empty
	// when using the default). The UI shows both so "reset to default" is possible.
	OrgSlug    string `json:"org_slug"`
	CustomSlug string `json:"custom_slug"`
	// URL is the public address of the page, so the UI never has to reconstruct
	// (and get subtly wrong) the route the server actually serves.
	URL string `json:"url"`
}

func toStatusPageSettingsResponse(s *statuspage.Settings) statusPageSettingsResponse {
	return statusPageSettingsResponse{
		Slug:           s.Slug,
		OrgName:        s.OrgName,
		Enabled:        s.Enabled,
		Title:          s.Title,
		PublishedCount: s.PublishedCount,
		OrgSlug:        s.OrgSlug,
		CustomSlug:     s.CustomSlug,
		URL:            "/status/" + s.Slug,
	}
}

func (h *StatusPageSettingsHandler) get(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	s, err := h.svc.Get(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, toStatusPageSettingsResponse(s))
}

func (h *StatusPageSettingsHandler) update(w http.ResponseWriter, r *http.Request) {
	var req statusPageSettingsRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	p := mustPrincipal(r)
	s, err := h.svc.Update(r.Context(), p.Role, p.OrgID, p.UserID, statuspage.UpdateInput{
		Enabled: req.Enabled,
		Title:   req.Title,
		Slug:    req.Slug,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, toStatusPageSettingsResponse(s))
}
