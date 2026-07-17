// Package ai contains adapters that enrich alerts with LLM analysis. The Ollama
// adapter implements notification.Analyzer by prompting an Ollama-compatible
// chat endpoint and mapping the JSON reply onto a notification.AlertAnalysis.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"beacon/internal/domain/notification"
)

// maxField clamps each free-text field so a runaway model can't produce a
// message that blows past Telegram's 4096-char limit.
const maxField = 600

// tokenTTL is how long each minted x-api-key bearer token is valid. Tokens are
// minted fresh per request (HMAC signing is microseconds), so this only needs to
// comfortably exceed one request's round-trip.
const tokenTTL = 5 * time.Minute

// OllamaAnalyzer prompts an Ollama-compatible /api/chat endpoint. It asks for a
// strict JSON object (Ollama's `format: json` guarantees valid JSON) describing
// the alert's assessed severity, likely cause, and a suggested fix.
//
// Auth: the endpoint sits behind a proxy that expects a short-lived JWT (a
// "bearer token") signed with a shared secret, presented in the x-api-key
// header — not the raw secret. We mint that token per request.
type OllamaAnalyzer struct {
	baseURL string // no trailing slash, no /api
	model   string
	secret  []byte // HMAC secret used to sign the x-api-key bearer token; empty = no auth header
	client  *http.Client
	now     func() time.Time // injectable clock for deterministic token tests
}

// NewOllamaAnalyzer builds an analyzer. signingSecret is the shared secret used
// to sign the x-api-key bearer token (empty to send no auth header). timeout
// bounds the whole HTTP call; the caller may pass a shorter context deadline to
// tighten it further.
func NewOllamaAnalyzer(baseURL, model, signingSecret string, timeout time.Duration) *OllamaAnalyzer {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &OllamaAnalyzer{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		secret:  []byte(signingSecret),
		client:  &http.Client{Timeout: timeout},
		now:     time.Now,
	}
}

// mintToken creates a short-lived HS256 JWT signed with the shared secret, to be
// sent in the x-api-key header. Claims are the standard registered set
// (issued-at, not-before, expiry); adjust here if the endpoint requires more.
func (a *OllamaAnalyzer) mintToken() (string, error) {
	now := a.now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(a.secret)
}

// ---- Ollama chat wire types ----

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []chatMessage  `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   string         `json:"format"`
	Options  map[string]any `json:"options,omitempty"`
	// Think disables the chain-of-thought channel on hybrid reasoning models
	// (e.g. qwen3). With format:"json" the JSON grammar is applied to the whole
	// output, so an enabled thinking channel collapses the answer to an empty
	// string. Ollama ignores this flag for non-reasoning models.
	Think *bool `json:"think,omitempty"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
	Done    bool        `json:"done"`
	Error   string      `json:"error"`
}

// analysisJSON is the shape we instruct the model to return.
type analysisJSON struct {
	Severity     string `json:"severity"`
	Summary      string `json:"summary"`
	LikelyCause  string `json:"likely_cause"`
	SuggestedFix string `json:"suggested_fix"`
}

const systemPrompt = `You are a senior Site Reliability Engineer triaging infrastructure monitoring alerts.
You will be given the details of a single alert from an uptime/health monitor.
Respond with ONLY a JSON object, no prose, using exactly these keys:
  "severity":      one of "high", "medium", or "low" — your assessment of business impact.
                   high  = a user-facing service is fully down or a security/data risk.
                   medium= degraded performance, partial failure, or a warning that will escalate.
                   low   = minor, transient, or non-user-facing.
  "summary":       one short sentence describing what is happening, in plain English.
  "likely_cause":  the single most probable root cause, concise.
  "suggested_fix": one concrete, actionable next step an on-call engineer should take.
Be specific and practical. Do not repeat the raw alert text verbatim.`

// Analyze prompts the model and maps its reply. Any transport, HTTP, or parse
// failure is returned as an error so the dispatcher can deliver the alert
// unenriched.
func (a *OllamaAnalyzer) Analyze(ctx context.Context, ev notification.AlertEvent) (*notification.AlertAnalysis, error) {
	content, err := a.chat(ctx, systemPrompt, buildUserPrompt(ev), 400)
	if err != nil {
		return nil, err
	}

	var parsed analysisJSON
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("ai: model did not return valid JSON: %w", err)
	}

	return &notification.AlertAnalysis{
		Severity:     normalizeSeverity(parsed.Severity),
		Summary:      clamp(parsed.Summary),
		LikelyCause:  clamp(parsed.LikelyCause),
		SuggestedFix: clamp(parsed.SuggestedFix),
		Model:        a.model,
	}, nil
}

// chat sends one system+user exchange and returns the model's raw content. Shared by
// every prompt in this package so auth, the JSON grammar, the disabled thinking
// channel and error handling are settled in exactly one place — the parts that are
// easy to get subtly wrong and expensive to debug through a model.
//
// numPredict is the output budget: enough for the answer, low enough that a model
// which starts rambling is cut off rather than paid for.
func (a *OllamaAnalyzer) chat(ctx context.Context, system, user string, numPredict int) (string, error) {
	noThink := false
	reqBody := chatRequest{
		Model:  a.model,
		Stream: false,
		Format: "json",
		Think:  &noThink,
		Options: map[string]any{
			// Low, not zero: this is diagnosis, and we want the most probable
			// reading of the evidence rather than a creative one.
			"temperature": 0.2,
			"num_predict": numPredict,
		},
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if len(a.secret) > 0 {
		token, err := a.mintToken()
		if err != nil {
			return "", fmt.Errorf("ai: sign auth token: %w", err)
		}
		req.Header.Set("x-api-key", token)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("ai: endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("ai: decode response: %w", err)
	}
	if cr.Error != "" {
		return "", fmt.Errorf("ai: model error: %s", cr.Error)
	}

	content := strings.TrimSpace(cr.Message.Content)
	if content == "" {
		return "", fmt.Errorf("ai: empty model response")
	}
	return content, nil
}

// buildUserPrompt renders the alert as a compact, labelled block for the model.
func buildUserPrompt(ev notification.AlertEvent) string {
	var b strings.Builder
	b.WriteString("A monitoring alert is firing. Analyze it.\n\n")
	writeLine(&b, "Alert", ev.AlertName)
	writeLine(&b, "Rule severity", ev.Severity)
	writeLine(&b, "Monitor", ev.MonitorName)
	writeLine(&b, "Monitor type", ev.MonitorType)
	writeLine(&b, "Target", ev.Target)
	writeLine(&b, "Summary", ev.Summary)
	writeLine(&b, "Description", ev.Description)
	return strings.TrimRight(b.String(), "\n")
}

func writeLine(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", label, value)
}

// normalizeSeverity coerces the model's severity into one of our three buckets,
// defaulting to medium for anything unexpected.
func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case notification.AISeverityHigh, "critical", "urgent", "p1":
		return notification.AISeverityHigh
	case notification.AISeverityLow, "info", "minor", "p3", "p4":
		return notification.AISeverityLow
	case notification.AISeverityMedium, "warning", "moderate", "p2":
		return notification.AISeverityMedium
	default:
		return notification.AISeverityMedium
	}
}

// clamp trims whitespace and caps length so a verbose model can't overflow the
// downstream message.
func clamp(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxField {
		return s
	}
	return strings.TrimSpace(s[:maxField]) + "…"
}
