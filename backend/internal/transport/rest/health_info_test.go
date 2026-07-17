package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The footer reads this endpoint, so its shape is a contract. env must be present
// (a promoted image cannot know it) and started_at must be an instant the browser
// can tick "deployed 2h ago" from — uptime_seconds alone was true only when it left.
func TestHealthReportsEnvAndStartedAt(t *testing.T) {
	started := time.Date(2026, 7, 17, 4, 0, 0, 0, time.UTC)
	h := NewHealthHandler("abc1234", "prod", started)
	h.now = func() time.Time { return started.Add(2 * time.Hour) }

	rec := httptest.NewRecorder()
	h.Health(rec, httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	// httpx.OK writes the value directly — no {data:...} envelope — and the browser
	// client does `return json as T`, so these keys ARE the footer's props.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rec.Body.String())
	}
	if body["version"] != "abc1234" {
		t.Errorf("version = %v, want abc1234", body["version"])
	}
	if body["env"] != "prod" {
		t.Errorf("env = %v, want prod — the footer cannot name the environment without it", body["env"])
	}
	if body["started_at"] != "2026-07-17T04:00:00Z" {
		t.Errorf("started_at = %v, want RFC3339 UTC", body["started_at"])
	}
	if up, _ := body["uptime_seconds"].(float64); up != 7200 {
		t.Errorf("uptime = %v, want 7200", body["uptime_seconds"])
	}
}
