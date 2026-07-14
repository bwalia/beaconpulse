package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"beacon/internal/domain/maintenance"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// MaintenanceHandler exposes maintenance-window CRUD. Reads are open to any
// authenticated member; writes require a writer role (a window silently blinds
// alerting, so it is treated like a security change — writer-gated and audited).
type MaintenanceHandler struct {
	svc       *maintenance.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
}

// NewMaintenanceHandler builds a MaintenanceHandler.
func NewMaintenanceHandler(svc *maintenance.Service, v *validate.Validator, a *middleware.Authenticator) *MaintenanceHandler {
	return &MaintenanceHandler{svc: svc, validator: v, auth: a}
}

// Routes returns the authenticated maintenance-window routes.
func (h *MaintenanceHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.Require)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.With(h.auth.RequireWriter).Post("/", h.create)
	r.With(h.auth.RequireWriter).Patch("/{id}", h.update)
	r.With(h.auth.RequireWriter).Delete("/{id}", h.delete)
	return r
}

// ---- DTOs ----

type createWindowRequest struct {
	Title       string      `json:"title" validate:"required,min=1,max=200"`
	Description string      `json:"description" validate:"omitempty,max=2000"`
	StartsAt    time.Time   `json:"starts_at" validate:"required"`
	EndsAt      time.Time   `json:"ends_at" validate:"required"`
	Scope       string      `json:"scope" validate:"required,oneof=org project monitor"`
	ScopeIDs    []uuid.UUID `json:"scope_ids"`
}

type updateWindowRequest struct {
	Title       *string      `json:"title" validate:"omitempty,min=1,max=200"`
	Description *string      `json:"description" validate:"omitempty,max=2000"`
	StartsAt    *time.Time   `json:"starts_at"`
	EndsAt      *time.Time   `json:"ends_at"`
	Scope       *string      `json:"scope" validate:"omitempty,oneof=org project monitor"`
	ScopeIDs    *[]uuid.UUID `json:"scope_ids"`
}

type windowResponse struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Scope       string    `json:"scope"`
	ScopeIDs    []string  `json:"scope_ids"`
	// Active is whether the window covers "now" — lets the UI badge live windows
	// without recomputing the start/end comparison client-side.
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func presentWindow(w *maintenance.Window) windowResponse {
	ids := make([]string, len(w.ScopeIDs))
	for i, id := range w.ScopeIDs {
		ids[i] = id.String()
	}
	return windowResponse{
		ID:          w.ID.String(),
		OrgID:       w.OrgID.String(),
		Title:       w.Title,
		Description: w.Description,
		StartsAt:    w.StartsAt,
		EndsAt:      w.EndsAt,
		Scope:       string(w.Scope),
		ScopeIDs:    ids,
		Active:      w.Active(time.Now().UTC()),
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}
}

func maintenanceActor(r *http.Request) maintenance.Actor {
	p := mustPrincipal(r)
	return maintenance.Actor{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role}
}

// ---- handlers ----

func (h *MaintenanceHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginationParams(r, 50, 200)
	items, total, err := h.svc.List(r.Context(), maintenanceActor(r), maintenance.ListFilter{Limit: limit, Offset: offset})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]windowResponse, 0, len(items))
	for i := range items {
		out = append(out, presentWindow(&items[i]))
	}
	httpx.OK(w, newListResponse(out, total, limit, offset))
}

func (h *MaintenanceHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	win, err := h.svc.Get(r.Context(), maintenanceActor(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentWindow(win))
}

func (h *MaintenanceHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createWindowRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	win, err := h.svc.Create(r.Context(), maintenanceActor(r), maintenance.CreateInput{
		Title:       req.Title,
		Description: req.Description,
		StartsAt:    req.StartsAt,
		EndsAt:      req.EndsAt,
		Scope:       maintenance.Scope(req.Scope),
		ScopeIDs:    req.ScopeIDs,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Created(w, presentWindow(win))
}

func (h *MaintenanceHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req updateWindowRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	var scope *maintenance.Scope
	if req.Scope != nil {
		s := maintenance.Scope(*req.Scope)
		scope = &s
	}
	win, err := h.svc.Update(r.Context(), maintenanceActor(r), id, maintenance.UpdateInput{
		Title:       req.Title,
		Description: req.Description,
		StartsAt:    req.StartsAt,
		EndsAt:      req.EndsAt,
		Scope:       scope,
		ScopeIDs:    req.ScopeIDs,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentWindow(win))
}

func (h *MaintenanceHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), maintenanceActor(r), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}
