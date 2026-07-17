package rest

import (
	"context"
	"net/http"
	"time"

	"beacon/internal/platform/httpx"
)

// Checker is a named readiness probe for a dependency (database, cache, ...).
type Checker struct {
	Name  string
	Check func(ctx context.Context) error
}

// HealthHandler serves liveness and readiness endpoints used by orchestrators
// and load balancers. Liveness reflects "the process is up"; readiness reflects
// "the process can serve traffic" (all critical dependencies reachable).
type HealthHandler struct {
	version  string
	env      string
	started  time.Time
	checkers []Checker
	now      func() time.Time
}

// NewHealthHandler builds a HealthHandler with the given dependency checkers.
//
// env comes from config at RUNTIME rather than being compiled in, and that is the
// whole point of reporting it: one image is promoted from int to test to prod, so
// anything baked into it can only ever name the environment it was BUILT for. The
// footer's job is to say which environment you are looking at now.
func NewHealthHandler(version, env string, started time.Time, checkers ...Checker) *HealthHandler {
	return &HealthHandler{version: version, env: env, started: started, checkers: checkers, now: time.Now}
}

// Live always returns 200 while the process is running.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	httpx.OK(w, map[string]any{"status": "ok"})
}

// Health returns basic build/runtime info: which build is running, in which
// environment, and since when.
//
// started_at is sent as well as uptime_seconds because uptime is a number that was
// true when the response left. The dashboard footer ticks "deployed 2h ago" off its
// own clock, and it can only do that from an instant, not a duration.
//
// "Deployed" is honest but not exact: this is when the PROCESS started, which is the
// deploy in every ordinary case, since a rollout replaces the pods. A crash-restart
// or a node drain also resets it, and it is worth knowing that before treating this
// as an audit trail.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	httpx.OK(w, map[string]any{
		"status":         "ok",
		"version":        h.version,
		"env":            h.env,
		"started_at":     h.started.UTC().Format(time.RFC3339),
		"uptime_seconds": int64(h.now().Sub(h.started).Seconds()),
	})
}

// Ready runs every dependency checker and returns 200 only if all pass,
// otherwise 503 with the per-dependency results.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	results := make(map[string]string, len(h.checkers))
	allOK := true
	for _, c := range h.checkers {
		if err := c.Check(ctx); err != nil {
			results[c.Name] = "error: " + err.Error()
			allOK = false
		} else {
			results[c.Name] = "ok"
		}
	}

	status := http.StatusOK
	overall := "ready"
	if !allOK {
		status = http.StatusServiceUnavailable
		overall = "not_ready"
	}
	httpx.JSON(w, status, map[string]any{"status": overall, "checks": results})
}
