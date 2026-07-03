// Package logger provides structured JSON logging built on the standard
// library's log/slog. All application logs flow through a single *slog.Logger
// so that every line carries consistent metadata (timestamp, level, request id,
// user id, correlation id) and is machine-parseable.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// contextKey is unexported to avoid collisions in context.Value.
type contextKey int

const (
	loggerKey contextKey = iota
)

// Fields carried through context and attached to every log line by the HTTP
// middleware. They are exported as slog attribute keys for consistency.
const (
	KeyRequestID     = "request_id"
	KeyCorrelationID = "correlation_id"
	KeyUserID        = "user_id"
	KeyLatencyMS     = "latency_ms"
)

// New builds a *slog.Logger. format is "json" or "text"; level is
// debug|info|warn|error. Unknown values fall back to safe defaults (json/info).
func New(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     parseLevel(level),
		AddSource: false,
	}
	var h slog.Handler
	switch strings.ToLower(format) {
	case "text":
		h = slog.NewTextHandler(os.Stdout, opts)
	default:
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithContext stores a logger in the context so downstream code can retrieve a
// request-scoped logger already enriched with request metadata.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext returns the request-scoped logger, or the default logger if none
// is present. It never returns nil.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
