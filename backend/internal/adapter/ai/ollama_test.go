package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"beacon/internal/domain/notification"
)

// fakeOllama returns a server that records the last request and replies with the
// given assistant content (which the adapter expects to be a JSON string).
func fakeOllama(t *testing.T, assistantContent string, status int) (*httptest.Server, *[]byte) {
	t.Helper()
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if status != 0 && status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		resp := map[string]any{
			"model":   "test-model",
			"message": map[string]string{"role": "assistant", "content": assistantContent},
			"done":    true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, &gotBody
}

func sampleEvent() notification.AlertEvent {
	return notification.AlertEvent{
		Status:      notification.StatusFiring,
		AlertName:   "MonitorDown",
		Severity:    "critical",
		MonitorName: "API health",
		MonitorType: "https",
		Target:      "https://api.example.com/health",
		Summary:     "API health is down",
		Description: "probe_success == 0 for 2m",
	}
}

func TestAnalyzeParsesResponse(t *testing.T) {
	content := `{"severity":"high","summary":"The API is fully down.","likely_cause":"Upstream crashed.","suggested_fix":"Restart the API pod."}`
	srv, _ := fakeOllama(t, content, http.StatusOK)

	a := NewOllamaAnalyzer(srv.URL, "test-model", "secret-key", 5*time.Second)
	got, err := a.Analyze(context.Background(), sampleEvent())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got.Severity != notification.AISeverityHigh {
		t.Errorf("severity = %q, want high", got.Severity)
	}
	if got.Summary != "The API is fully down." {
		t.Errorf("summary = %q", got.Summary)
	}
	if got.SuggestedFix != "Restart the API pod." {
		t.Errorf("suggested_fix = %q", got.SuggestedFix)
	}
	if got.Model != "test-model" {
		t.Errorf("model = %q, want test-model", got.Model)
	}
}

func TestAnalyzeSendsAuthAndJSONFormat(t *testing.T) {
	content := `{"severity":"low","summary":"x","likely_cause":"y","suggested_fix":"z"}`
	srv, bodyp := fakeOllama(t, content, http.StatusOK)

	a := NewOllamaAnalyzer(srv.URL, "my-model", "abc123", 5*time.Second)
	if _, err := a.Analyze(context.Background(), sampleEvent()); err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	var req chatRequest
	if err := json.Unmarshal(*bodyp, &req); err != nil {
		t.Fatalf("request body not valid chatRequest JSON: %v", err)
	}
	if req.Model != "my-model" {
		t.Errorf("model = %q, want my-model", req.Model)
	}
	if req.Format != "json" {
		t.Errorf("format = %q, want json", req.Format)
	}
	if req.Stream {
		t.Error("stream should be false")
	}
	// think must be explicitly false so reasoning models (qwen3) don't collapse
	// their JSON answer to an empty string under format:json.
	if req.Think == nil || *req.Think {
		t.Errorf("think = %v, want explicit false", req.Think)
	}
	if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
		t.Errorf("messages = %+v, want [system,user]", req.Messages)
	}
	// The user prompt should carry the alert's key facts.
	if !strings.Contains(req.Messages[1].Content, "api.example.com") {
		t.Errorf("user prompt missing target: %s", req.Messages[1].Content)
	}
}

func TestAnalyzeAuthHeaderIsSignedJWT(t *testing.T) {
	const secret = "top-secret"
	content := `{"severity":"medium","summary":"x","likely_cause":"y","suggested_fix":"z"}`
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"role": "assistant", "content": content}, "done": true,
		})
	}))
	defer srv.Close()

	a := NewOllamaAnalyzer(srv.URL, "m", secret, 5*time.Second)
	if _, err := a.Analyze(context.Background(), sampleEvent()); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// The header must be the raw secret NEVER; it must be a JWT the secret verifies.
	if gotKey == secret || gotKey == "" {
		t.Fatalf("x-api-key should be a signed token, got %q", gotKey)
	}
	tok, err := jwt.Parse(gotKey, func(*jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !tok.Valid {
		t.Fatalf("x-api-key is not a valid HS256 JWT signed by the secret: %v", err)
	}
}

func TestAnalyzeNoSecretSendsNoHeader(t *testing.T) {
	content := `{"severity":"low","summary":"x","likely_cause":"y","suggested_fix":"z"}`
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["X-Api-Key"]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"role": "assistant", "content": content}, "done": true,
		})
	}))
	defer srv.Close()

	a := NewOllamaAnalyzer(srv.URL, "m", "", 5*time.Second)
	if _, err := a.Analyze(context.Background(), sampleEvent()); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if present {
		t.Error("no secret configured, but x-api-key header was sent")
	}
}

func TestAnalyzeErrorsOnNon200(t *testing.T) {
	srv, _ := fakeOllama(t, "", http.StatusInternalServerError)
	a := NewOllamaAnalyzer(srv.URL, "m", "", time.Second)
	if _, err := a.Analyze(context.Background(), sampleEvent()); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

func TestAnalyzeErrorsOnNonJSONContent(t *testing.T) {
	// format:json normally prevents this, but be defensive.
	srv, _ := fakeOllama(t, "not json at all", http.StatusOK)
	a := NewOllamaAnalyzer(srv.URL, "m", "", time.Second)
	if _, err := a.Analyze(context.Background(), sampleEvent()); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestNormalizeSeverity(t *testing.T) {
	cases := map[string]string{
		"high": "high", "HIGH": "high", "critical": "high", "p1": "high",
		"medium": "medium", "warning": "medium", "": "medium", "weird": "medium",
		"low": "low", "info": "low", "minor": "low",
	}
	for in, want := range cases {
		if got := normalizeSeverity(in); got != want {
			t.Errorf("normalizeSeverity(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClampTruncates(t *testing.T) {
	long := strings.Repeat("a", maxField+50)
	got := clamp(long)
	if len([]rune(got)) > maxField+1 { // +1 for the ellipsis rune
		t.Errorf("clamp did not truncate: len=%d", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Error("expected ellipsis suffix on truncated value")
	}
}
