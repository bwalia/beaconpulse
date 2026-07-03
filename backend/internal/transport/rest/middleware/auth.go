package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
)

type principalKey struct{}

// Principal is the authenticated caller derived from a valid access token.
type Principal struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}

// Authenticator builds authentication middleware bound to a TokenManager.
type Authenticator struct {
	tm *auth.TokenManager
}

// NewAuthenticator constructs an Authenticator.
func NewAuthenticator(tm *auth.TokenManager) *Authenticator {
	return &Authenticator{tm: tm}
}

// Require rejects requests without a valid Bearer access token. On success the
// Principal is stored in the request context for handlers to read.
func (a *Authenticator) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := bearerToken(r)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		claims, err := a.tm.ParseAccessToken(raw)
		if err != nil {
			httpx.Error(w, r, apperror.Unauthorized("invalid or expired access token"))
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			httpx.Error(w, r, apperror.Unauthorized("malformed access token subject"))
			return
		}
		orgID, err := uuid.Parse(claims.OrgID)
		if err != nil {
			httpx.Error(w, r, apperror.Unauthorized("malformed access token org"))
			return
		}

		p := Principal{UserID: userID, OrgID: orgID, Role: claims.Role}
		ctx := context.WithValue(r.Context(), principalKey{}, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireWriter is like Require but additionally rejects read-only (viewer)
// roles for mutating endpoints.
func (a *Authenticator) RequireWriter(next http.Handler) http.Handler {
	return a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := PrincipalFromContext(r.Context())
		if !p.Role.CanWrite() {
			httpx.Error(w, r, apperror.Forbidden("your role does not permit this action"))
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// PrincipalFromContext returns the authenticated principal, or ok=false if the
// request was not authenticated.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

func bearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", apperror.Unauthorized("authorization header is required")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", apperror.Unauthorized("authorization header must be a Bearer token")
	}
	token := strings.TrimSpace(h[len(prefix):])
	if token == "" {
		return "", apperror.Unauthorized("bearer token is empty")
	}
	return token, nil
}
