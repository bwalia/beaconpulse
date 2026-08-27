package apns

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"testing"
	"time"
)

func testKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return k
}

func TestParseP8Key(t *testing.T) {
	k := testKey(t)
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	got, err := ParseP8Key(pemBytes)
	if err != nil {
		t.Fatalf("ParseP8Key: %v", err)
	}
	if !got.Equal(k) {
		t.Fatal("parsed key does not match original")
	}

	if _, err := ParseP8Key([]byte("not a pem")); err == nil {
		t.Fatal("expected an error for non-PEM input")
	}
}

func TestResultTokenIsDead(t *testing.T) {
	cases := []struct {
		name string
		res  Result
		dead bool
	}{
		{"ok", Result{StatusCode: 200}, false},
		{"gone unregistered", Result{StatusCode: 410, Reason: "Unregistered"}, true},
		{"bad device token", Result{StatusCode: 400, Reason: "BadDeviceToken"}, true},
		{"wrong topic", Result{StatusCode: 400, Reason: "DeviceTokenNotForTopic"}, true},
		{"server error", Result{StatusCode: 500, Reason: "InternalServerError"}, false},
		{"rate limited", Result{StatusCode: 429, Reason: "TooManyRequests"}, false},
	}
	for _, c := range cases {
		if got := c.res.TokenIsDead(); got != c.dead {
			t.Errorf("%s: TokenIsDead()=%v want %v", c.name, got, c.dead)
		}
	}
}

func TestProviderTokenCachesAndRefreshes(t *testing.T) {
	c, err := New(Config{
		PrivateKey: testKey(t),
		KeyID:      "KID123",
		TeamID:     "TEAM123",
		Topic:      "com.example.app",
		HTTPClient: &http.Client{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return now }

	first, err := c.providerToken()
	if err != nil {
		t.Fatalf("providerToken: %v", err)
	}
	if first == "" {
		t.Fatal("expected a signed token")
	}
	// Within the TTL the same token is reused (no re-sign).
	if again, _ := c.providerToken(); again != first {
		t.Fatal("expected the cached token to be reused within the TTL")
	}
	// Past the TTL a fresh token is minted.
	now = now.Add(providerTokenTTL + time.Minute)
	if refreshed, _ := c.providerToken(); refreshed == first {
		t.Fatal("expected a new token once the TTL elapsed")
	}
}
