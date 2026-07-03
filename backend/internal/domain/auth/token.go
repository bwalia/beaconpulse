package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims are the custom JWT claims embedded in Beacon access tokens. Downstream
// middleware reads these to establish the authenticated principal without a
// database round-trip.
type Claims struct {
	OrgID string `json:"org"`
	Role  Role   `json:"role"`
	Type  string `json:"typ"`
	jwt.RegisteredClaims
}

const (
	accessTokenType = "access"
	// proxyTokenType marks the long-lived, org-scoped token stored in an httpOnly
	// cookie and used only by the gateway (nginx auth_request) to authorize
	// full-page navigations to the raw Prometheus/Alertmanager UIs and derive the
	// tenant's org_id for label enforcement.
	proxyTokenType = "proxy"
)

// TokenManager issues short-lived JWT access tokens and generates opaque
// refresh tokens. Access tokens are stateless (HS256-signed); refresh tokens
// are opaque random strings whose HMAC hash is persisted, allowing revocation
// and rotation. The plaintext refresh token is never stored.
type TokenManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	now           func() time.Time
}

// NewTokenManager builds a TokenManager from secrets and TTLs.
func NewTokenManager(accessSecret, refreshSecret []byte, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		accessSecret:  accessSecret,
		refreshSecret: refreshSecret,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		now:           time.Now,
	}
}

// AccessTTL exposes the configured access-token lifetime (for expires_in).
func (m *TokenManager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL exposes the configured refresh-token lifetime.
func (m *TokenManager) RefreshTTL() time.Duration { return m.refreshTTL }

// IssueAccessToken signs a JWT access token for the given user.
func (m *TokenManager) IssueAccessToken(u *User) (string, error) {
	now := m.now()
	claims := Claims{
		OrgID: u.OrgID.String(),
		Role:  u.Role,
		Type:  accessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			Issuer:    "beacon",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.accessSecret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// ParseAccessToken validates a JWT access token's signature, expiry and type,
// returning the embedded claims.
func (m *TokenManager) ParseAccessToken(raw string) (*Claims, error) {
	return m.parseTyped(raw, accessTokenType)
}

// IssueProxyToken signs a long-lived (refresh-TTL) token for the gateway cookie.
func (m *TokenManager) IssueProxyToken(u *User) (string, error) {
	now := m.now()
	claims := Claims{
		OrgID: u.OrgID.String(),
		Role:  u.Role,
		Type:  proxyTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
			Issuer:    "beacon",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.accessSecret)
	if err != nil {
		return "", fmt.Errorf("sign proxy token: %w", err)
	}
	return signed, nil
}

// ParseProxyToken validates a gateway proxy-session token.
func (m *TokenManager) ParseProxyToken(raw string) (*Claims, error) {
	return m.parseTyped(raw, proxyTokenType)
}

func (m *TokenManager) parseTyped(raw, wantType string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.accessSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("beacon"))
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.Type != wantType {
		return nil, fmt.Errorf("invalid %s token", wantType)
	}
	return claims, nil
}

// GeneratedRefreshToken bundles a plaintext refresh token with its stored hash
// and expiry.
type GeneratedRefreshToken struct {
	Plaintext string
	Hash      string
	ExpiresAt time.Time
}

// GenerateRefreshToken creates a cryptographically-random opaque token and its
// HMAC-SHA256 hash. Only the hash is persisted.
func (m *TokenManager) GenerateRefreshToken() (GeneratedRefreshToken, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return GeneratedRefreshToken{}, fmt.Errorf("generate refresh token: %w", err)
	}
	plaintext := base64.RawURLEncoding.EncodeToString(raw)
	return GeneratedRefreshToken{
		Plaintext: plaintext,
		Hash:      m.HashRefreshToken(plaintext),
		ExpiresAt: m.now().Add(m.refreshTTL),
	}, nil
}

// HashRefreshToken returns the deterministic HMAC-SHA256 hash of a plaintext
// refresh token, hex-encoded. Deterministic hashing allows an indexed lookup by
// hash while keeping the plaintext unrecoverable from the database.
func (m *TokenManager) HashRefreshToken(plaintext string) string {
	mac := hmac.New(sha256.New, m.refreshSecret)
	mac.Write([]byte(plaintext))
	return hex.EncodeToString(mac.Sum(nil))
}
