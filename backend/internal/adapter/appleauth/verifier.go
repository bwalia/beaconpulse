// Package appleauth verifies Apple-issued "Sign in with Apple" identity tokens.
// It is the infrastructure implementation of auth.AppleVerifier. Unlike Google,
// Apple ships no first-party Go token library, so this fetches Apple's public
// signing keys (JWKS) itself and verifies the RS256 signature and claims with the
// already-vendored golang-jwt. Keys are cached, so after the first call this is a
// local cryptographic check.
package appleauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"beacon/internal/domain/auth"
)

const (
	appleIssuer  = "https://appleid.apple.com"
	appleJWKSURL = "https://appleid.apple.com/auth/keys"
	// jwksCacheTTL bounds how long Apple's signing keys are reused before refetch.
	// Apple rotates keys rarely; an hour is safe and keeps this a local check.
	jwksCacheTTL = time.Hour
)

// Verifier validates Apple identity tokens against one or more accepted client
// ids (audiences) — the app bundle id, one per white-label brand.
type Verifier struct {
	clientIDs []string
	client    *http.Client
	jwksURL   string

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	now       func() time.Time
}

// New builds a Verifier accepting tokens whose audience matches any of clientIDs.
func New(clientIDs []string) *Verifier {
	return &Verifier{
		clientIDs: clientIDs,
		client:    &http.Client{Timeout: 10 * time.Second},
		jwksURL:   appleJWKSURL,
		now:       time.Now,
	}
}

// Verify checks the token's signature (against Apple's public keys), issuer,
// expiry and audience, returning the verified identity.
func (v *Verifier) Verify(ctx context.Context, identityToken string) (*auth.AppleIdentity, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(identityToken, claims,
		func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			if kid == "" {
				return nil, errors.New("appleauth: token has no kid")
			}
			return v.keyByID(ctx, kid)
		},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
	)
	if err != nil {
		return nil, fmt.Errorf("appleauth: %w", err)
	}
	// Audience is checked here rather than via a parser option because we accept
	// several client ids (one per brand) and the option validates only one.
	if !v.audienceOK(claims) {
		return nil, errors.New("appleauth: audience mismatch")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, errors.New("appleauth: token has no subject")
	}
	email, _ := claims["email"].(string)
	return &auth.AppleIdentity{
		Subject:       sub,
		Email:         email,
		EmailVerified: claimBool(claims, "email_verified"),
	}, nil
}

func (v *Verifier) audienceOK(claims jwt.MapClaims) bool {
	switch aud := claims["aud"].(type) {
	case string:
		return contains(v.clientIDs, aud)
	case []any:
		for _, a := range aud {
			if s, ok := a.(string); ok && contains(v.clientIDs, s) {
				return true
			}
		}
	}
	return false
}

// keyByID resolves a JWKS key id to Apple's RSA public key, fetching and caching
// the key set. On a fetch failure it falls back to a stale cached key so a
// transient outage at Apple does not break sign-in.
func (v *Verifier) keyByID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if key, ok := v.cachedKey(kid, false); ok {
		return key, nil
	}
	keys, err := v.fetchKeys(ctx)
	if err != nil {
		if key, ok := v.cachedKey(kid, true); ok {
			return key, nil
		}
		return nil, err
	}
	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = v.now()
	v.mu.Unlock()

	if key, ok := keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("appleauth: no signing key for kid %q", kid)
}

// cachedKey returns a cached key. When allowStale is false it ignores an expired
// cache; when true it returns whatever is cached regardless of age.
func (v *Verifier) cachedKey(kid string, allowStale bool) (*rsa.PublicKey, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keys == nil {
		return nil, false
	}
	if !allowStale && v.now().Sub(v.fetchedAt) >= jwksCacheTTL {
		return nil, false
	}
	key, ok := v.keys[kid]
	return key, ok
}

func (v *Verifier) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("appleauth: build jwks request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("appleauth: fetch jwks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("appleauth: jwks endpoint returned %d", resp.StatusCode)
	}

	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("appleauth: decode jwks: %w", err)
	}

	out := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublicKey(k.N, k.E)
		if err != nil {
			continue // skip a malformed key rather than failing the whole set
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("appleauth: no usable RSA keys in jwks")
	}
	return out, nil
}

// rsaPublicKey builds an RSA public key from a JWK's base64url modulus/exponent.
func rsaPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("exponent: %w", err)
	}
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() || e.Int64() > (1<<31-1) {
		return nil, errors.New("exponent out of range")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(e.Int64())}, nil
}

// claimBool tolerates "email_verified" arriving as a JSON bool or the string
// "true" — Apple, like Google, has used both.
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

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
