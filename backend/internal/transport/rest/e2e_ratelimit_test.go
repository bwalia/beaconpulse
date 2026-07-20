package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"beacon/internal/transport/rest/middleware"
	"beacon/internal/platform/ratelimit"
)

// Middleware that is wired but never reached is the classic way a rate limit gets
// believed in without working. This drives it over real HTTP, through the real
// middleware, and checks the wire response — status AND the Retry-After that tells a
// client how to back off.
func TestRateLimitRefusesOverHTTPWithRetryAfter(t *testing.T) {
	l := ratelimit.New(0.001, 2, 100) // 2 through, then refuse
	h := middleware.RateLimit(l, middleware.ByIP, 30)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	call := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
		// Set by our gateway. Keying on RemoteAddr instead would lump the whole
		// internet into one bucket behind the proxy.
		req.Header.Set("X-Forwarded-For", ip)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < 2; i++ {
		if got := call("203.0.113.1").Code; got != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i+1, got)
		}
	}

	rec := call("203.0.113.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit request = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After: a client cannot know how long to wait, so it hammers")
	}

	// The forwarded address is genuinely what keys the bucket.
	if got := call("198.51.100.2").Code; got != http.StatusOK {
		t.Fatalf("a different client got %d — the limit is keyed on the proxy, not the caller", got)
	}
}
