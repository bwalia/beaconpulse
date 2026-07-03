package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"beacon/internal/platform/metrics"
)

// Metrics returns middleware that records request count, latency and in-flight
// count into the application's Prometheus registry. It uses chi's matched route
// pattern (e.g. /api/v1/monitors/{id}) as the "route" label to avoid unbounded
// cardinality from raw paths.
func Metrics(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.IncInflight()
			defer m.DecInflight()

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			m.ObserveRequest(r.Method, route, strconv.Itoa(rec.status), time.Since(start))
		})
	}
}
