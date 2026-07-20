package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"beacon/internal/domain/apikey"
	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
)

type principalKey struct{}

// Principal is the authenticated caller, however they authenticated.
//
// A dashboard session and an API key both produce one of these, and that is the point:
// authorization, org scoping, plan limits and billing all read the Principal, so
// machine access inherits every rule the UI already obeys instead of growing a second
// set that drifts. Adding a credential type is a change here and nowhere else.
type Principal struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
	// APIKeyID is set when the caller presented an API key rather than a session.
	// Endpoints that must never be reachable by machine — minting further keys, most
	// of all — check this; everything else is deliberately indifferent to it.
	APIKeyID *uuid.UUID
}

// IsAPIKey reports whether this request was authenticated by an API key.
func (p Principal) IsAPIKey() bool { return p.APIKeyID != nil }

// KeyVerifier resolves an API key to the org and role it grants. Implemented by the
// apikey service; an interface so this package does not depend on it.
type KeyVerifier interface {
	Verify(ctx context.Context, secret string) (*apikey.Verified, error)
}

// Authenticator builds authentication middleware bound to a TokenManager, and
// optionally to an API-key verifier.
type Authenticator struct {
	tm   *auth.TokenManager
	keys KeyVerifier // nil = API keys not enabled; sessions still work
}

// NewAuthenticator constructs an Authenticator.
func NewAuthenticator(tm *auth.TokenManager) *Authenticator {
	return &Authenticator{tm: tm}
}

// WithKeys returns an Authenticator that also accepts API keys.
func (a *Authenticator) WithKeys(keys KeyVerifier) *Authenticator {
	return &Authenticator{tm: a.tm, keys: keys}
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

		// An API key is recognised by its prefix, so a session token is never sent
		// through a database lookup and a key is never run through JWT parsing —
		// each credential fails as itself, with its own error.
		if apikey.Looks(raw) {
			if a.keys == nil {
				httpx.Error(w, r, apperror.Unauthorized("API keys are not enabled on this deployment"))
				return
			}
			v, err := a.keys.Verify(r.Context(), raw)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			// UserID is the key id. A key acts for the ORGANIZATION, not for the
			// person who minted it: attributing its writes to that user would keep
			// naming them long after they left, and would survive their account being
			// deleted. Ownership everywhere is org-scoped, so nothing downstream cares.
			p := Principal{UserID: v.KeyID, OrgID: v.OrgID, Role: v.Role, APIKeyID: &v.KeyID}
			ctx := context.WithValue(r.Context(), principalKey{}, p)
			next.ServeHTTP(w, r.WithContext(ctx))
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

// RequireSession is like Require but refuses API keys, for the few actions that must
// stay human. Minting keys is the one that matters: a key that can mint keys turns one
// leaked credential into permanent self-renewing access that outlives revoking the
// original, so the escalation is removed rather than managed.
func (a *Authenticator) RequireSession(next http.Handler) http.Handler {
	return a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := PrincipalFromContext(r.Context())
		if p.IsAPIKey() {
			httpx.Error(w, r, apperror.Forbidden(
				"this action requires signing in — API keys cannot manage API keys"))
			return
		}
		next.ServeHTTP(w, r)
	}))
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
