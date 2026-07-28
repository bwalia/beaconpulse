// Package googleauth verifies Google-issued OpenID Connect ID tokens for the
// "Sign in with Google" flow. It is the infrastructure implementation of
// auth.GoogleVerifier and depends on Google's official token library, which fetches
// and caches Google's public signing keys.
package googleauth

import (
	"context"
	"errors"

	"google.golang.org/api/idtoken"

	"beacon/internal/domain/auth"
)

// Verifier validates Google ID tokens against one or more accepted client ids
// (audiences) — one per white-label brand the API serves.
type Verifier struct {
	clientIDs []string
}

// New builds a Verifier accepting tokens issued for any of clientIDs.
func New(clientIDs []string) *Verifier {
	return &Verifier{clientIDs: clientIDs}
}

// Verify checks the token's signature (against Google's public keys), issuer, expiry
// and audience, returning the verified identity. The token is accepted if its audience
// matches any configured client id. idtoken.Validate caches Google's certs, so after
// the first call this is a local cryptographic check with no network round-trip.
func (v *Verifier) Verify(ctx context.Context, idToken string) (*auth.GoogleIdentity, error) {
	var lastErr error = errors.New("no google client ids configured")
	for _, aud := range v.clientIDs {
		payload, err := idtoken.Validate(ctx, idToken, aud)
		if err != nil {
			lastErr = err
			continue
		}
		return &auth.GoogleIdentity{
			Subject:       payload.Subject,
			Email:         claimString(payload.Claims, "email"),
			EmailVerified: claimBool(payload.Claims, "email_verified"),
			Name:          claimString(payload.Claims, "name"),
		}, nil
	}
	return nil, lastErr
}

func claimString(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// claimBool tolerates the "email_verified" claim arriving as either a JSON bool or the
// string "true", both of which Google has used.
func claimBool(m map[string]any, key string) bool {
	switch val := m[key].(type) {
	case bool:
		return val
	case string:
		return val == "true"
	default:
		return false
	}
}
