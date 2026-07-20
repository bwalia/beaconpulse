package middleware

import (
	"net/http"
	"strconv"
	"time"

	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/ratelimit"
)

// RateLimit refuses requests that arrive faster than a key is allowed to send them.
//
// This is the layer that makes an unauthenticated endpoint safe to expose. Without it,
// anything reachable without credentials is billable by strangers: signing up costs us
// a probe loop, logging in costs a bcrypt, and a readiness check costs a round trip to
// Postgres and Redis. None of those are expensive once; all of them are expensive ten
// thousand times a second, and nothing else in the stack says no.
//
// The limiter is in-memory, per pod, and that is a deliberate trade. A shared counter
// in Redis would be exact across replicas, but it puts a network hop and a dependency
// in front of every request including login — so a Redis blip would become an outage of
// the thing that is supposed to protect us during a blip. In-memory cannot fail, cannot
// add latency, and cannot be the reason the site is down. The cost is that with N
// replicas the effective ceiling is N times the configured rate, which bounds abuse to
// within a small constant factor rather than exactly. For deciding whether to serve a
// flood, a factor of two is not the difference that matters.
func RateLimit(l *ratelimit.KeyedLimiter, key func(*http.Request) string, retryAfter time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(key(r)) {
				// Retry-After turns a refusal into instructions. A well-behaved client
				// backs off by exactly the right amount instead of hammering, which is
				// the difference between shedding a burst and amplifying it.
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				httpx.Error(w, r, apperror.RateLimited("too many requests — slow down and try again shortly"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ByIP keys a limit on the caller's address.
//
// The right key for anything unauthenticated, because there is no account to key on
// yet — which is exactly what makes signup and login abusable.
//
// It reads X-Forwarded-For, which our own gateway sets. Keying on RemoteAddr instead
// would see every request as coming from the nginx pod, put the entire internet in one
// bucket, and turn the first burst into a self-inflicted outage for everyone else.
func ByIP(r *http.Request) string { return clientIP(r) }

// ByOrg keys a limit on the authenticated organization: the unit that pays, and the
// unit whose runaway script we are bounding. Falls back to the address for anything
// unauthenticated that reaches it, so the key is never empty — an empty key would put
// every such caller in one shared bucket.
func ByOrg(r *http.Request) string {
	if p, ok := PrincipalFromContext(r.Context()); ok {
		return p.OrgID.String()
	}
	return clientIP(r)
}
