// Package apperror defines Beacon's centralized error model. Domain and service
// layers return *Error values with a stable machine-readable Code; the HTTP
// transport translates them into consistent JSON responses and status codes.
// This keeps error handling uniform across the codebase and prevents internal
// details from leaking to clients.
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable error classifier. Clients may switch on it.
type Code string

const (
	CodeValidation    Code = "validation"     // 400 — input failed validation
	CodeUnauthorized  Code = "unauthorized"   // 401 — missing/invalid credentials
	CodeForbidden     Code = "forbidden"      // 403 — authenticated but not allowed
	CodeNotFound      Code = "not_found"      // 404 — resource does not exist
	CodeConflict      Code = "conflict"       // 409 — uniqueness / state conflict
	CodeQuotaExceeded Code = "quota_exceeded" // 402 — plan limit reached
	CodeRateLimited   Code = "rate_limited"   // 429 — too many requests
	CodeInternal      Code = "internal"       // 500 — unexpected server error
	CodeUnavailable   Code = "unavailable"    // 503 — dependency unavailable
)

// FieldError describes a single field-level validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error is the canonical application error. It never contains data that would
// be unsafe to log; the Message is safe to return to clients.
type Error struct {
	Code    Code
	Message string
	Fields  []FieldError
	// err is the wrapped cause, retained for logging but never serialized.
	err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the wrapped cause for errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.err }

// HTTPStatus maps the error code to an HTTP status.
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case CodeValidation:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeQuotaExceeded:
		return http.StatusPaymentRequired
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// WithCause attaches an underlying cause and returns the same *Error for
// chaining.
func (e *Error) WithCause(cause error) *Error {
	e.err = cause
	return e
}

// New constructs an *Error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf constructs an *Error with a formatted message.
func Newf(code Code, format string, a ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// ---- constructors for common cases ----

func Validation(message string, fields ...FieldError) *Error {
	return &Error{Code: CodeValidation, Message: message, Fields: fields}
}

func Unauthorized(message string) *Error  { return New(CodeUnauthorized, message) }
func Forbidden(message string) *Error     { return New(CodeForbidden, message) }
func NotFound(message string) *Error      { return New(CodeNotFound, message) }
func Conflict(message string) *Error      { return New(CodeConflict, message) }
func QuotaExceeded(message string) *Error { return New(CodeQuotaExceeded, message) }
func RateLimited(message string) *Error   { return New(CodeRateLimited, message) }

// Internal wraps an unexpected error. The client-facing message is deliberately
// generic; the cause is preserved for server-side logs.
func Internal(cause error) *Error {
	return (&Error{Code: CodeInternal, Message: "an internal error occurred"}).WithCause(cause)
}

// From converts an arbitrary error into an *Error. If err already is (or wraps)
// an *Error, that value is returned; otherwise it is treated as internal.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	return Internal(err)
}

// IsCode reports whether err is an *Error with the given code.
func IsCode(err error, code Code) bool {
	var ae *Error
	return errors.As(err, &ae) && ae.Code == code
}
