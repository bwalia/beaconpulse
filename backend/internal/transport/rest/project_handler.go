package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/project"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// ProjectHandler exposes project CRUD endpoints.
type ProjectHandler struct {
	svc       *project.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
}

// NewProjectHandler builds a ProjectHandler.
func NewProjectHandler(svc *project.Service, v *validate.Validator, a *middleware.Authenticator) *ProjectHandler {
	return &ProjectHandler{svc: svc, validator: v, auth: a}
}

// Routes returns the authenticated project routes. Mutations additionally
// require a writer role.
func (h *ProjectHandler) Routes() chi.Router {
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

type createProjectRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=200"`
	Slug        string `json:"slug" validate:"omitempty,max=63"`
	Description string `json:"description" validate:"omitempty,max=2000"`
	Environment string `json:"environment" validate:"omitempty,oneof=production staging development"`
	IsActive    *bool  `json:"is_active"`
}

type updateProjectRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=200"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Environment *string `json:"environment" validate:"omitempty,oneof=production staging development"`
	IsActive    *bool   `json:"is_active"`
}

type projectResponse struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Environment string    `json:"environment"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func presentProject(p *project.Project) projectResponse {
	return projectResponse{
		ID:          p.ID.String(),
		OrgID:       p.OrgID.String(),
		Name:        p.Name,
		Slug:        p.Slug,
		Description: p.Description,
		Environment: string(p.Environment),
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// ---- handlers ----

func (h *ProjectHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginationParams(r, 50, 200)
	items, total, err := h.svc.List(r.Context(), projectActor(r), project.ListFilter{
		Search:      r.URL.Query().Get("search"),
		Environment: r.URL.Query().Get("environment"),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]projectResponse, 0, len(items))
	for i := range items {
		out = append(out, presentProject(&items[i]))
	}
	httpx.OK(w, newListResponse(out, total, limit, offset))
}

func (h *ProjectHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	p, err := h.svc.Get(r.Context(), projectActor(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentProject(p))
}

func (h *ProjectHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	p, err := h.svc.Create(r.Context(), projectActor(r), project.CreateInput{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Environment: project.Environment(req.Environment),
		IsActive:    req.IsActive,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Created(w, presentProject(p))
}

func (h *ProjectHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req updateProjectRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	in := project.UpdateInput{Name: req.Name, Description: req.Description, IsActive: req.IsActive}
	if req.Environment != nil {
		env := project.Environment(*req.Environment)
		in.Environment = &env
	}
	p, err := h.svc.Update(r.Context(), projectActor(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentProject(p))
}

func (h *ProjectHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), projectActor(r), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}
