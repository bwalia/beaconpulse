package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"beacon/internal/domain/diagnose"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/ratelimit"
	"beacon/internal/transport/rest/middleware"
)

// DiagnoseHandler exposes on-demand AI diagnosis of a failing monitor.
type DiagnoseHandler struct {
	svc     *diagnose.Service
	auth    *middleware.Authenticator
	limiter *ratelimit.KeyedLimiter
}

// NewDiagnoseHandler builds a DiagnoseHandler.
//
// The limiter is not optional, and it is not about money. Billing caps what an org can
// SPEND; this caps what it can spend AT ONCE, and those are different failures. A
// diagnosis is the most expensive thing this API does — a handful of network probes
// and a model invocation on hardware we own — and the price deliberately does not
// discourage use, so a few dollars of credit or a Pro allowance buys enough runs to
// saturate the model if they all arrive together. Then nobody's diagnosis returns,
// including the one from the org that is actually on fire.
//
// One every six seconds sustained, bursting to three. A human diagnosing an outage
// presses this once and reads the answer for a minute; three back-to-back covers
// someone checking a few monitors after a bad deploy. Anything beyond that is a script.
//
// Keyed by ORG, not by user or IP: the org is what pays and what has an allowance, so
// it is the thing that must not be able to saturate the model — and a limit per user
// would just be a limit per invite.
func NewDiagnoseHandler(svc *diagnose.Service, a *middleware.Authenticator) *DiagnoseHandler {
	return &DiagnoseHandler{
		svc:     svc,
		auth:    a,
		limiter: ratelimit.New(1.0/6.0, 3, 10000),
	}
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
	// Before the plan gate and before any work: a refusal here must be the cheapest
	// thing the server can do, or the limiter becomes part of the load it exists to
	// shed.
	if !h.limiter.Allow(p.OrgID.String()) {
		httpx.Error(w, r, apperror.Validation(
			"too many diagnoses at once — wait a few seconds and try again"))
		return
	}

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
