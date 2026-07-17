package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"beacon/internal/domain/diagnose"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/transport/rest/middleware"
)

// DiagnoseHandler exposes on-demand AI diagnosis of a failing monitor.
type DiagnoseHandler struct {
	svc  *diagnose.Service
	auth *middleware.Authenticator
}

func NewDiagnoseHandler(svc *diagnose.Service, a *middleware.Authenticator) *DiagnoseHandler {
	return &DiagnoseHandler{svc: svc, auth: a}
}

// Routes mounts the diagnosis endpoint.
//
// POST, though it reads nothing and changes nothing in our database: it makes the
// server open sockets to a third party and spend a model's time, so it must never be
// something a browser can prefetch, a crawler can follow, or a proxy can cache.
func (h *DiagnoseHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.Require)
	r.Post("/", h.run)
	return r
}

func (h *DiagnoseHandler) run(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperror.Validation("invalid monitor id"))
		return
	}
	p := mustPrincipal(r)
	out, err := h.svc.Run(r.Context(), diagnose.Actor{
		UserID: p.UserID,
		OrgID:  p.OrgID,
		Role:   p.Role,
	}, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, out)
}
