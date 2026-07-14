package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/heartbeat"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/ratelimit"
)

// HeartbeatHandler serves the PUBLIC ping-ingest endpoint for heartbeat monitors.
//
// It is UNAUTHENTICATED by design — the token in the URL is the credential (a
// "capability URL", same model as Healthchecks.io / Cronitor / GitHub webhook
// URLs). It accepts GET and POST because cron/curl/wget/PowerShell all differ,
// returns 200 fast on a valid token, and is rate-limited per token so a leaked URL
// cannot be turned into a database hammer.
type HeartbeatHandler struct {
	svc     *heartbeat.Service
	limiter *ratelimit.KeyedLimiter
}

// NewHeartbeatHandler builds a HeartbeatHandler. A heartbeat legitimately pings at
// most every ~10s, so 1 req/s sustained with a small burst is generous for real
// use and still caps abuse hard.
func NewHeartbeatHandler(svc *heartbeat.Service) *HeartbeatHandler {
	return &HeartbeatHandler{
		svc:     svc,
		limiter: ratelimit.New(1, 5, 50000),
	}
}

// Routes returns the PUBLIC (unauthenticated) heartbeat routes.
//
// Deliberately not behind Authenticator.Require and mounted under an explicit
// /public prefix so "this needs no token" is obvious in logs and proxy rules.
func (h *HeartbeatHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{token}", h.ping)
	r.Post("/{token}", h.ping)
	return r
}

func (h *HeartbeatHandler) ping(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	// Rate-limit per token. A limited request is refused with 429 without touching
	// the database, so a flood of a single leaked URL cannot amplify into DB load.
	if !h.limiter.Allow(token) {
		httpx.Error(w, r, apperror.RateLimited("too many pings; slow down"))
		return
	}

	if err := h.svc.Ping(r.Context(), token); err != nil {
		httpx.Error(w, r, err)
		return
	}
	// Tiny, fast body. Some cron wrappers check for a 2xx; give them one cheaply.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
