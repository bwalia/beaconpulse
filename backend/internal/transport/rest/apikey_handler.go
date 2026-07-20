package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"beacon/internal/domain/apikey"
	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// APIKeyHandler manages the credentials machines authenticate with.
type APIKeyHandler struct {
	svc       *apikey.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
}

func NewAPIKeyHandler(svc *apikey.Service, v *validate.Validator, a *middleware.Authenticator) *APIKeyHandler {
	return &APIKeyHandler{svc: svc, validator: v, auth: a}
}

// Routes mounts key management.
//
// RequireSession, not Require: every route here is deliberately unreachable with an API
// key. Otherwise a leaked key could mint a replacement for itself, and revoking the
// original would achieve nothing.
func (h *APIKeyHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.RequireSession)
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Delete("/{id}", h.revoke)
	return r
}

func apikeyActor(r *http.Request) apikey.Actor {
	p := mustPrincipal(r)
	return apikey.Actor{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role}
}

type createKeyRequest struct {
	Name string `json:"name" validate:"required,max=80"`
	// Role the key holds; empty means the creator's own. Capped at the creator's role
	// by the service.
	Role string `json:"role" validate:"omitempty,oneof=owner admin member viewer"`
	// ExpiresInDays is optional. Expressed as a duration rather than a date because
	// that is how the decision is actually made — "this key is for a 90-day contract"
	// — and it saves every client computing a timestamp.
	ExpiresInDays int `json:"expires_in_days" validate:"omitempty,min=1,max=3650"`
}

// createKeyResponse carries the secret. It is the only time it will ever be returned:
// only its hash is stored, so this response cannot be reproduced, and the UI has to
// say so plainly.
type createKeyResponse struct {
	Key apikey.Key `json:"key"`
	// Secret is the credential itself. Show it once, tell the user to store it.
	Secret string `json:"secret"`
}

func (h *APIKeyHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	in := apikey.CreateInput{Name: req.Name, Role: auth.Role(req.Role)}
	if req.ExpiresInDays > 0 {
		exp := time.Now().UTC().AddDate(0, 0, req.ExpiresInDays)
		in.ExpiresAt = &exp
	}

	k, secret, err := h.svc.Issue(r.Context(), apikeyActor(r), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Created(w, createKeyResponse{Key: *k, Secret: secret})
}

func (h *APIKeyHandler) list(w http.ResponseWriter, r *http.Request) {
	keys, err := h.svc.List(r.Context(), apikeyActor(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, map[string]any{"data": keys})
}

func (h *APIKeyHandler) revoke(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperror.Validation("invalid key id"))
		return
	}
	if err := h.svc.Revoke(r.Context(), apikeyActor(r), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}
