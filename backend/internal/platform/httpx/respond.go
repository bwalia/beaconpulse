// Package httpx contains transport-agnostic helpers for writing consistent JSON
// responses and decoding requests. Every success and error response across the
// API shares the same envelope shape so clients can rely on a single contract.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"beacon/internal/platform/apperror"
	"beacon/internal/platform/logger"
)

// ErrorBody is the standard error envelope returned to clients.
type ErrorBody struct {
	Error ErrorPayload `json:"error"`
}

// ErrorPayload carries the machine code, human message, optional field detail,
// and the request id for support/debugging correlation.
type ErrorPayload struct {
	Code      apperror.Code         `json:"code"`
	Message   string                `json:"message"`
	Fields    []apperror.FieldError `json:"fields,omitempty"`
	RequestID string                `json:"request_id,omitempty"`
}

// JSON writes v as JSON with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Default().Error("failed to encode response", "error", err)
	}
}

// Created writes a 201 with the given body.
func Created(w http.ResponseWriter, v any) { JSON(w, http.StatusCreated, v) }

// OK writes a 200 with the given body.
func OK(w http.ResponseWriter, v any) { JSON(w, http.StatusOK, v) }

// NoContent writes a 204.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Error translates any error into the standard error envelope. Internal errors
// are logged with their cause; the client only sees a generic message.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	ae := apperror.From(err)
	reqID := RequestIDFromContext(r.Context())

	if ae.HTTPStatus() >= 500 {
		logger.FromContext(r.Context()).Error("request failed",
			slog.String("error", ae.Error()),
			slog.String("code", string(ae.Code)),
		)
	}

	JSON(w, ae.HTTPStatus(), ErrorBody{Error: ErrorPayload{
		Code:      ae.Code,
		Message:   ae.Message,
		Fields:    ae.Fields,
		RequestID: reqID,
	}})
}

// DecodeJSON decodes the request body into dst, enforcing a size limit and
// rejecting unknown fields and trailing data. It returns an *apperror.Error with
// a validation code on malformed input.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !isJSON(ct) {
		return apperror.Validation("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return apperror.Validation("request body too large")
		case errors.Is(err, io.EOF):
			return apperror.Validation("request body must not be empty")
		default:
			return apperror.Validation("request body contains malformed JSON")
		}
	}
	if dec.More() {
		return apperror.Validation("request body must contain a single JSON object")
	}
	return nil
}

func isJSON(ct string) bool {
	// Accept "application/json" possibly followed by "; charset=..."
	for i := 0; i < len(ct); i++ {
		if ct[i] == ';' {
			ct = ct[:i]
			break
		}
	}
	return trimSpace(ct) == "application/json"
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
