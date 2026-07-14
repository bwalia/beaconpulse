package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"beacon/internal/domain/notification"
	"beacon/internal/platform/safehttp"
)

// SlackNotifier delivers alerts to a Slack Incoming Webhook.
//
// The webhook URL is the credential (anyone who has it can post to the channel),
// so it lives in the channel's ENCRYPTED secret, never in plaintext config. And
// because it is a tenant-supplied URL that our server fetches, every request goes
// through safehttp — a tenant must not be able to make Beacon POST to an internal
// address by setting a malicious "Slack" URL.
type SlackNotifier struct {
	client *safehttp.Client
}

// NewSlackNotifier builds a SlackNotifier over the SSRF-guarded client.
func NewSlackNotifier(client *safehttp.Client) *SlackNotifier {
	return &SlackNotifier{client: client}
}

// Type identifies this notifier.
func (s *SlackNotifier) Type() notification.ChannelType { return notification.TypeSlack }

// Slack Block Kit payload. We use a coloured attachment (the only way to get the
// severity bar) wrapping blocks for the structured content.
type slackPayload struct {
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color  string       `json:"color"`
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type     string        `json:"type"`
	Text     *slackText    `json:"text,omitempty"`
	Fields   []slackText   `json:"fields,omitempty"`
	Elements []slackButton `json:"elements,omitempty"`
}

type slackText struct {
	Type string `json:"type"` // "mrkdwn" | "plain_text"
	Text string `json:"text"`
}

type slackButton struct {
	Type string     `json:"type"` // "button"
	Text *slackText `json:"text"`
	URL  string     `json:"url,omitempty"`
}

// Send renders and delivers the message to the configured webhook URL.
func (s *SlackNotifier) Send(ctx context.Context, ch notification.Decrypted, msg notification.Message) error {
	webhookURL := strings.TrimSpace(ch.Secret)
	if webhookURL == "" {
		return fmt.Errorf("slack: webhook URL is not configured")
	}

	payload := slackPayload{Attachments: []slackAttachment{{
		Color:  severityHex(msg),
		Blocks: slackBlocks(msg),
	}}}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// Slack returns 200 with body "ok" on success, or 4xx with a text reason
	// ("invalid_payload", "no_service", …). Surface the reason to the user.
	if resp.StatusCode != http.StatusOK {
		reason, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack API error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(reason)))
	}
	return nil
}

func slackBlocks(msg notification.Message) []slackBlock {
	blocks := []slackBlock{{
		Type: "header",
		Text: &slackText{Type: "plain_text", Text: statusHeadline(msg)},
	}}

	// Two-column fields from the shared detail lines.
	var fields []slackText
	for _, l := range detailLines(msg) {
		k, v, ok := strings.Cut(l, ": ")
		if !ok {
			continue
		}
		fields = append(fields, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*%s*\n%s", k, v)})
	}
	// Slack caps a section at 10 fields; keep the most important ones.
	if len(fields) > 10 {
		fields = fields[:10]
	}
	if len(fields) > 0 {
		blocks = append(blocks, slackBlock{Type: "section", Fields: fields})
	}

	if msg.DashboardURL != "" {
		blocks = append(blocks, slackBlock{
			Type: "actions",
			Elements: []slackButton{{
				Type: "button",
				Text: &slackText{Type: "plain_text", Text: "Open in Beacon"},
				URL:  msg.DashboardURL,
			}},
		})
	}
	return blocks
}
