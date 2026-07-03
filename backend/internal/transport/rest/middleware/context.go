// Package middleware provides the HTTP middleware chain used by the REST API:
// request identification, structured request logging, panic recovery, and (in
// the auth module) authentication. Middleware is ordered so that recovery wraps
// logging wraps everything else, guaranteeing every request is logged exactly
// once and no panic escapes to the client as a broken connection.
package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"beacon/internal/platform/httpx"
)

// Header names for request/correlation propagation.
const (
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"
)

// RequestID assigns each request a unique id (honoring an inbound X-Request-ID
// if present) and a correlation id (from X-Correlation-ID, defaulting to the
// request id). Both are stored in the context and echoed in response headers so
// clients and downstream services can trace a request end-to-end.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderRequestID)
		if reqID == "" {
			reqID = uuid.NewString()
		}
		corrID := r.Header.Get(HeaderCorrelationID)
		if corrID == "" {
			corrID = reqID
		}

		ctx := httpx.WithRequestID(r.Context(), reqID)
		ctx = httpx.WithCorrelationID(ctx, corrID)

		w.Header().Set(HeaderRequestID, reqID)
		w.Header().Set(HeaderCorrelationID, corrID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
