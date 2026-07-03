// Package rest implements the versioned HTTP API. Handlers are thin: they
// decode and validate input, invoke a domain service, and present the result
// using the shared httpx envelope. All business logic lives in the domain
// layer.
package rest

import (
	"net"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/domain/monitor"
	"beacon/internal/domain/project"
	"beacon/internal/platform/apperror"
	"beacon/internal/transport/rest/middleware"
)

// maxBodyBytes bounds request bodies decoded by handlers (defense against
// memory-exhaustion). 1 MiB is generous for JSON control-plane payloads.
const maxBodyBytes = 1 << 20

// requestMeta extracts client IP and user agent for audit and token records.
func requestMeta(r *http.Request) auth.RequestMeta {
	return auth.RequestMeta{IP: clientIP(r), UserAgent: r.UserAgent()}
}

// mustPrincipal returns the authenticated principal. It must only be called from
// handlers mounted behind the auth middleware, which guarantees its presence.
func mustPrincipal(r *http.Request) middleware.Principal {
	p, _ := middleware.PrincipalFromContext(r.Context())
	return p
}

// projectActor builds a project.Actor from the authenticated principal.
func projectActor(r *http.Request) project.Actor {
	p := mustPrincipal(r)
	return project.Actor{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role}
}

// monitorActor builds a monitor.Actor from the authenticated principal.
func monitorActor(r *http.Request) monitor.Actor {
	p := mustPrincipal(r)
	return monitor.Actor{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role}
}

// parseIDParam parses a UUID path parameter, returning a validation error on a
// malformed value.
func parseIDParam(r *http.Request, name string) (uuid.UUID, error) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperror.Validation("invalid id in path",
			apperror.FieldError{Field: name, Message: "must be a valid UUID"})
	}
	return id, nil
}

// pagination is the standard pagination metadata returned with list responses.
type pagination struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// listResponse is the standard envelope for paginated collections.
type listResponse struct {
	Data       any        `json:"data"`
	Pagination pagination `json:"pagination"`
}

func newListResponse(data any, total, limit, offset int) listResponse {
	return listResponse{Data: data, Pagination: pagination{Total: total, Limit: limit, Offset: offset}}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := indexByte(xff, ','); i >= 0 {
			return trim(xff[:i])
		}
		return trim(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// paginationParams parses ?limit= and ?offset= with sane defaults and caps.
func paginationParams(r *http.Request, defLimit, maxLimit int) (limit, offset int) {
	limit, offset = defLimit, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
