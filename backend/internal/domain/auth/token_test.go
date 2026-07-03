package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func testTokenManager() *TokenManager {
	access := []byte("access-secret-at-least-32-bytes-long-xx")
	refresh := []byte("refresh-secret-at-least-32-bytes-long-x")
	return NewTokenManager(access, refresh, 15*time.Minute, 24*time.Hour)
}

func TestAccessTokenRoundTrip(t *testing.T) {
	tm := testTokenManager()
	u := &User{ID: uuid.New(), OrgID: uuid.New(), Role: RoleAdmin}

	token, err := tm.IssueAccessToken(u)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	claims, err := tm.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.Subject != u.ID.String() {
		t.Errorf("subject = %q, want %q", claims.Subject, u.ID.String())
	}
	if claims.OrgID != u.OrgID.String() {
		t.Errorf("org = %q, want %q", claims.OrgID, u.OrgID.String())
	}
	if claims.Role != RoleAdmin {
		t.Errorf("role = %q, want admin", claims.Role)
	}
}

func TestParseAccessTokenRejectsWrongSecret(t *testing.T) {
	tm := testTokenManager()
	u := &User{ID: uuid.New(), OrgID: uuid.New(), Role: RoleMember}
	token, _ := tm.IssueAccessToken(u)

	other := NewTokenManager([]byte("different-secret-at-least-32-bytes-long"), []byte("r-secret-at-least-32-bytes-long-padding"), time.Minute, time.Hour)
	if _, err := other.ParseAccessToken(token); err == nil {
		t.Fatal("expected parse to fail with a different signing secret")
	}
}

func TestParseAccessTokenRejectsExpired(t *testing.T) {
	tm := NewTokenManager([]byte("access-secret-at-least-32-bytes-long-xx"), []byte("refresh-secret-at-least-32-bytes-long-x"), -time.Minute, time.Hour)
	u := &User{ID: uuid.New(), OrgID: uuid.New(), Role: RoleViewer}
	token, _ := tm.IssueAccessToken(u)
	if _, err := tm.ParseAccessToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestRefreshTokenHashDeterministicAndUnique(t *testing.T) {
	tm := testTokenManager()
	g1, err := tm.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	g2, _ := tm.GenerateRefreshToken()

	if g1.Plaintext == g2.Plaintext {
		t.Fatal("expected unique refresh tokens")
	}
	if tm.HashRefreshToken(g1.Plaintext) != g1.Hash {
		t.Fatal("hash is not deterministic for the same plaintext")
	}
	if g1.Hash == g2.Hash {
		t.Fatal("expected different hashes for different tokens")
	}
}
