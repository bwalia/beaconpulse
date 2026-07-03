package httpx

import "context"

type ctxKey int

const (
	requestIDKey ctxKey = iota
	correlationIDKey
)

// WithRequestID stores the request id in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request id, or "" if absent.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithCorrelationID stores the client-supplied correlation id in the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext returns the correlation id, or "" if absent.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}
