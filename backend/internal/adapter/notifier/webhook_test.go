package notifier

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"beacon/internal/domain/notification"
	"beacon/internal/platform/safehttp"
)

func testMessage() notification.Message {
	return notification.Message{
		Status:      notification.StatusFiring,
		Severity:    "critical",
		Title:       "api is down",
		MonitorName: "api",
		MonitorType: "https",
		Target:      "https://api.example.com",
		Project:     "Prod",
		Environment: "production",
		Timestamp:   time.Unix(1_700_000_000, 0).UTC(),
	}
}

// TestWebhook_SignedDelivery locks the on-the-wire contract: the envelope shape,
// the event header, and — most importantly — the exact HMAC scheme, verified the
// way a real receiver would. If the signing format ever changes silently, this
// breaks, because customers will have written verification code against it.
func TestWebhook_SignedDelivery(t *testing.T) {
	const key = "whsec_test_key"
	var (
		gotSig   string
		gotEvent string
		gotBody  []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Beacon-Signature")
		gotEvent = r.Header.Get("X-Beacon-Event")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// AllowPrivate+AllowHTTP: the test server is on http loopback. This is the
	// "internal webhook opt-in" path; the block itself is covered in safehttp_test.
	wn := NewWebhookNotifier(safehttp.New(safehttp.Config{AllowPrivate: true, AllowHTTP: true}))
	wn.now = func() time.Time { return time.Unix(1_700_000_123, 0) } // deterministic t

	ch := notification.Decrypted{
		Type:   notification.TypeWebhook,
		Config: map[string]string{"url": srv.URL},
		Secret: key,
	}
	if err := wn.Send(context.Background(), ch, testMessage()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if gotEvent != "alert.firing" {
		t.Errorf("X-Beacon-Event = %q, want alert.firing", gotEvent)
	}

	// Reproduce the receiver's verification exactly: parse t and v1, recompute
	// HMAC-SHA256(key, "<t>." + body), constant-time compare.
	var ts, v1 string
	for _, part := range strings.Split(gotSig, ",") {
		if k, v, ok := strings.Cut(part, "="); ok {
			switch k {
			case "t":
				ts = v
			case "v1":
				v1 = v
			}
		}
	}
	if ts != "1700000123" {
		t.Errorf("signature t = %q, want 1700000123", ts)
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(ts + "."))
	mac.Write(gotBody)
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(v1), []byte(want)) {
		t.Errorf("signature v1 = %q, want %q — the HMAC scheme changed", v1, want)
	}

	// Envelope shape.
	var env map[string]any
	if err := json.Unmarshal(gotBody, &env); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if env["version"] != float64(1) {
		t.Errorf("version = %v, want 1", env["version"])
	}
	if env["event"] != "alert.firing" {
		t.Errorf("event = %v, want alert.firing", env["event"])
	}
	if mon, _ := env["monitor"].(map[string]any); mon["name"] != "api" {
		t.Errorf("monitor.name = %v, want api", mon["name"])
	}
}

// TestWebhook_UnsignedWhenNoKey: no key configured => no signature header. A
// customer who has not set a signing key must not receive a bogus one.
func TestWebhook_UnsignedWhenNoKey(t *testing.T) {
	var hadSig bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadSig = r.Header["X-Beacon-Signature"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	wn := NewWebhookNotifier(safehttp.New(safehttp.Config{AllowPrivate: true, AllowHTTP: true}))
	ch := notification.Decrypted{Type: notification.TypeWebhook, Config: map[string]string{"url": srv.URL}}
	if err := wn.Send(context.Background(), ch, testMessage()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if hadSig {
		t.Error("unsigned webhook sent an X-Beacon-Signature header")
	}
}

// TestWebhook_RefusesInternalTarget: the guard is wired in. A webhook aimed at
// loopback must fail (with the default, non-AllowPrivate client).
func TestWebhook_RefusesInternalTarget(t *testing.T) {
	wn := NewWebhookNotifier(safehttp.New(safehttp.Config{AllowHTTP: true})) // AllowPrivate false
	ch := notification.Decrypted{
		Type:   notification.TypeWebhook,
		Config: map[string]string{"url": "http://127.0.0.1:9/hook"},
	}
	err := wn.Send(context.Background(), ch, testMessage())
	if err == nil {
		t.Fatal("Send() to loopback succeeded, want blocked")
	}
}
