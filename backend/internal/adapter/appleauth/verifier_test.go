package appleauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testKID = "test-key-1"
	testAud = "com.sysops247.app"
)

// newTestVerifier serves a JWKS containing key's public half and points a
// Verifier at it.
func newTestVerifier(t *testing.T, key *rsa.PrivateKey) *Verifier {
	t.Helper()
	jwks := jwksJSON(key.PublicKey)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	t.Cleanup(srv.Close)
	v := New([]string{testAud})
	v.jwksURL = srv.URL
	return v
}

func jwksJSON(pub rsa.PublicKey) []byte {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	doc := map[string]any{
		"keys": []map[string]string{
			{"kty": "RSA", "kid": testKID, "use": "sig", "alg": "RS256", "n": n, "e": e},
		},
	}
	b, _ := json.Marshal(doc)
	return b
}

func mint(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = testKID
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":            appleIssuer,
		"aud":            testAud,
		"sub":            "001234.abcdef",
		"email":          "user@privaterelay.appleid.com",
		"email_verified": "true", // Apple's string form
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Add(-time.Minute).Unix(),
	}
}

func TestVerifyAcceptsValidToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newTestVerifier(t, key)

	id, err := v.Verify(context.Background(), mint(t, key, validClaims()))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id.Subject != "001234.abcdef" {
		t.Errorf("subject=%q", id.Subject)
	}
	if id.Email != "user@privaterelay.appleid.com" {
		t.Errorf("email=%q", id.Email)
	}
	if !id.EmailVerified {
		t.Error("expected email_verified true from the string form")
	}
}

func TestVerifyRejectsWrongAudience(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newTestVerifier(t, key)
	claims := validClaims()
	claims["aud"] = "com.someone.else"
	if _, err := v.Verify(context.Background(), mint(t, key, claims)); err == nil {
		t.Fatal("expected an audience-mismatch rejection")
	}
}

func TestVerifyRejectsWrongIssuer(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newTestVerifier(t, key)
	claims := validClaims()
	claims["iss"] = "https://evil.example.com"
	if _, err := v.Verify(context.Background(), mint(t, key, claims)); err == nil {
		t.Fatal("expected an issuer rejection")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newTestVerifier(t, key)
	claims := validClaims()
	claims["exp"] = time.Now().Add(-time.Hour).Unix()
	if _, err := v.Verify(context.Background(), mint(t, key, claims)); err == nil {
		t.Fatal("expected an expiry rejection")
	}
}

func TestVerifyRejectsWrongSigningKey(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newTestVerifier(t, key) // JWKS exposes `key`

	// Signed by a different key but carrying the advertised kid.
	if _, err := v.Verify(context.Background(), mint(t, other, validClaims())); err == nil {
		t.Fatal("expected a signature rejection")
	}
}
