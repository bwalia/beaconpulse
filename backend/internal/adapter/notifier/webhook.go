package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"beacon/internal/domain/notification"
	"beacon/internal/platform/safehttp"
)

// WebhookNotifier POSTs a stable JSON envelope to a tenant-supplied URL.
//
// This is the most dangerous channel type — an arbitrary URL that our server will
// fetch — so it goes through safehttp, which refuses internal/loopback/metadata
// addresses (including via DNS rebinding). When a signing key is configured, the
// body is HMAC-signed so the receiver can verify the request genuinely came from
// Beacon and was not tampered with or replayed.
type WebhookNotifier struct {
	client *safehttp.Client
	now    func() time.Time // injectable for deterministic signature tests
}

// NewWebhookNotifier builds a WebhookNotifier over the SSRF-guarded client.
func NewWebhookNotifier(client *safehttp.Client) *WebhookNotifier {
	return &WebhookNotifier{client: client, now: time.Now}
}

// Type identifies this notifier.
func (wn *WebhookNotifier) Type() notification.ChannelType { return notification.TypeWebhook }

// webhookEnvelope is the documented, VERSIONED payload. Consumers should switch on
// `version`; new fields are additive within a version.
type webhookEnvelope struct {
	Version   int                         `json:"version"`
	Event     string                      `json:"event"` // alert.firing | alert.resolved | test
	Status    string                      `json:"status"`
	Severity  string                      `json:"severity,omitempty"`
	Title     string                      `json:"title,omitempty"`
	Monitor   webhookMonitor              `json:"monitor"`
	Project   *webhookProject             `json:"project,omitempty"`
	Message   string                      `json:"message,omitempty"`
	Timestamp string                      `json:"timestamp"`
	Duration  float64                     `json:"duration_seconds,omitempty"`
	Dashboard string                      `json:"dashboard_url,omitempty"`
	Test      bool                        `json:"test,omitempty"`
	Analysis  *notification.AlertAnalysis `json:"analysis,omitempty"`
}

type webhookMonitor struct {
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Target string `json:"target,omitempty"`
}

type webhookProject struct {
	Name        string `json:"name"`
	Environment string `json:"environment,omitempty"`
}

// Send builds, signs and delivers the envelope.
func (wn *WebhookNotifier) Send(ctx context.Context, ch notification.Decrypted, msg notification.Message) error {
	rawURL := strings.TrimSpace(ch.Config["url"])
	if rawURL == "" {
		return fmt.Errorf("webhook: url is not configured")
	}
	method := strings.ToUpper(strings.TrimSpace(ch.Config["method"]))
	if method == "" {
		method = http.MethodPost
	}
	if method != http.MethodPost && method != http.MethodPut {
		return fmt.Errorf("webhook: method %q is not allowed (use POST or PUT)", method)
	}

	body, err := json.Marshal(buildEnvelope(msg))
	if err != nil {
		return fmt.Errorf("webhook: marshal envelope: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Beacon-Webhook/1")
	req.Header.Set("X-Beacon-Event", eventName(msg))

	// Sign the exact bytes if a key is present. Timestamp is inside the signed
	// payload so a captured request cannot be replayed against a later window —
	// the receiver rejects a stale t. This is the Stripe scheme.
	if key := strings.TrimSpace(ch.Secret); key != "" {
		t := wn.now().Unix()
		mac := hmac.New(sha256.New, []byte(key))
		fmt.Fprintf(mac, "%d.", t)
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Beacon-Signature", "t="+strconv.FormatInt(t, 10)+",v1="+sig)
	}

	resp, err := wn.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// Any 2xx is success. Surface the status on failure; the receiver's body is
	// their own, so we do not echo it.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func buildEnvelope(msg notification.Message) webhookEnvelope {
	env := webhookEnvelope{
		Version:   1,
		Event:     eventName(msg),
		Status:    string(msg.Status),
		Severity:  msg.Severity,
		Title:     msg.Title,
		Monitor:   webhookMonitor{Name: msg.MonitorName, Type: msg.MonitorType, Target: msg.Target},
		Message:   msg.Description,
		Timestamp: msg.Timestamp.UTC().Format(time.RFC3339),
		Dashboard: msg.DashboardURL,
		Test:      msg.IsTest,
		Analysis:  msg.Analysis,
	}
	if msg.Project != "" || msg.Environment != "" {
		env.Project = &webhookProject{Name: msg.Project, Environment: msg.Environment}
	}
	if msg.Duration > 0 {
		env.Duration = msg.Duration.Seconds()
	}
	return env
}

func eventName(msg notification.Message) string {
	if msg.IsTest {
		return "test"
	}
	if msg.Status == notification.StatusResolved {
		return "alert.resolved"
	}
	return "alert.firing"
}
