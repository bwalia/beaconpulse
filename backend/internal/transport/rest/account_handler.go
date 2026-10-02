package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/account"
	"beacon/internal/platform/httpx"
	"beacon/internal/transport/rest/middleware"
)

// AccountHandler exposes permanent account deletion.
type AccountHandler struct {
	svc  *account.Service
	auth *middleware.Authenticator
}

// NewAccountHandler builds an AccountHandler.
func NewAccountHandler(svc *account.Service, a *middleware.Authenticator) *AccountHandler {
	return &AccountHandler{svc: svc, auth: a}
}

// Routes returns the account routes. Deletion is session-only: an API key acts for
// the organization, and a leaked key must never be able to erase it.
func (h *AccountHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.With(h.auth.RequireSession).Post("/delete", h.delete)
	return r
}

type deleteAccountRequest struct {
	ConfirmEmail string `json:"confirm_email"` // checked against the account by the service
}

func (h *AccountHandler) delete(w http.ResponseWriter, r *http.Request) {
	var req deleteAccountRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	p := mustPrincipal(r)
	if err := h.svc.Delete(r.Context(), p.UserID, p.OrgID, req.ConfirmEmail); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}
