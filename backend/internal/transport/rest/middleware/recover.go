package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/logger"
)

// Recover is the outermost middleware. It converts any panic in a downstream
// handler into a logged internal error and a clean 500 response, so the process
// never crashes and clients never see a dropped connection. The stack trace is
// captured for diagnosis but never sent to the client.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.FromContext(r.Context()).Error("panic recovered",
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
					slog.String("path", r.URL.Path),
				)
				httpx.Error(w, r, apperror.New(apperror.CodeInternal, "an internal error occurred"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
