package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/statuspage"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
)

// StatusPageHandler serves the PUBLIC status page.
//
// This is the only unauthenticated read in the API, so it is written defensively:
// no auth middleware to lean on, no org from a token, and the slug is attacker-
// controlled. Everything it can return is the narrow statuspage projection —
// there is no code path here that can reach a target, an IP or a check config.
type StatusPageHandler struct {
	svc *statuspage.Service
}

// NewStatusPageHandler builds a StatusPageHandler.
func NewStatusPageHandler(svc *statuspage.Service) *StatusPageHandler {
	return &StatusPageHandler{svc: svc}
}

// Routes returns the PUBLIC (unauthenticated) status routes.
//
// Deliberately not mounted behind Authenticator.Require — that is the entire
// point of the feature. Kept in its own router so the absence of auth is a
// visible, reviewable decision rather than an omission someone has to notice.
func (h *StatusPageHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{slug}", h.get)
	return r
}

// ---- DTOs ----
//
// These mirror the domain projection exactly. They exist so the JSON shape is
// pinned by a type: adding a field to the domain cannot leak into the public
// response without someone editing this file.

type statusMonitorResponse struct {
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
}

type statusGroupResponse struct {
	Name        string                  `json:"name"`
	Environment string                  `json:"environment"`
	Monitors    []statusMonitorResponse `json:"monitors"`
}

type statusPageResponse struct {
	OrgName   string                `json:"org_name"`
	Title     string                `json:"title"`
	Overall   string                `json:"overall"`
	Groups    []statusGroupResponse `json:"groups"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// get serves GET /api/v1/public/status/{slug}.
func (h *StatusPageHandler) get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		httpx.Error(w, r, apperror.NotFound("no status page at that address"))
		return
	}

	page, err := h.svc.Get(r.Context(), slug)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	// An unpublished org and a non-existent one return the SAME 404. Anything
	// else would let a stranger enumerate which organizations exist by watching
	// for a different error.
	if page == nil {
		httpx.Error(w, r, apperror.NotFound("no status page at that address"))
		return
	}

	groups := make([]statusGroupResponse, 0, len(page.Groups))
	for _, g := range page.Groups {
		monitors := make([]statusMonitorResponse, 0, len(g.Monitors))
		for _, m := range g.Monitors {
			monitors = append(monitors, statusMonitorResponse{
				Name:          m.Name,
				Status:        string(m.Status),
				LastCheckedAt: m.LastCheckedAt,
			})
		}
		groups = append(groups, statusGroupResponse{
			Name:        g.Name,
			Environment: g.Environment,
			Monitors:    monitors,
		})
	}

	// Let the CDN/proxy absorb repeat traffic. The page is cheap to produce but
	// it is unauthenticated, so the best request is one that never reaches us.
	// 30s matches the default probe interval: caching longer would show stale
	// "operational" during a real outage, which is the one thing a status page
	// must never do.
	w.Header().Set("Cache-Control", "public, max-age=30")

	httpx.OK(w, statusPageResponse{
		OrgName:   page.OrgName,
		Title:     page.Title,
		Overall:   string(page.Overall),
		Groups:    groups,
		UpdatedAt: page.UpdatedAt,
	})
}
