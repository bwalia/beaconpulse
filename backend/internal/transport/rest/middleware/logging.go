package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"beacon/internal/platform/httpx"
	"beacon/internal/platform/logger"
)

// statusRecorder captures the response status code and byte count for logging
// without altering the response.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	n, err := sr.ResponseWriter.Write(b)
	sr.bytes += n
	return n, err
}

// Logging returns middleware that stores a request-scoped logger in the context
// (enriched with request/correlation ids) and emits exactly one structured line
// per request on completion, including method, path, status, latency and size.
func Logging(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqID := httpx.RequestIDFromContext(r.Context())
			corrID := httpx.CorrelationIDFromContext(r.Context())
			reqLogger := base.With(
				slog.String(logger.KeyRequestID, reqID),
				slog.String(logger.KeyCorrelationID, corrID),
			)
			ctx := logger.WithContext(r.Context(), reqLogger)

			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r.WithContext(ctx))

			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			reqLogger.Info("http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Int64(logger.KeyLatencyMS, time.Since(start).Milliseconds()),
				slog.String("remote", clientIP(r)),
			)
		})
	}
}

// clientIP extracts the best-effort client IP, honoring X-Forwarded-For when
// present (the API is expected to run behind a trusted proxy in production).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return trimSpace(xff[:i])
			}
		}
		return trimSpace(xff)
	}
	return r.RemoteAddr
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}
