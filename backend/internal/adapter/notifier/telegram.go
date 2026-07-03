// Package notifier contains the concrete channel integrations that implement
// notification.Notifier. Telegram is the first (and highest-priority) channel:
// it delivers rich, HTML-formatted alert and recovery messages via the Telegram
// Bot API.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"beacon/internal/domain/notification"
)

// TelegramNotifier delivers messages through the Telegram Bot API.
type TelegramNotifier struct {
	client  *http.Client
	baseURL string // overridable for tests; defaults to https://api.telegram.org
}

// NewTelegramNotifier builds a TelegramNotifier.
func NewTelegramNotifier() *TelegramNotifier {
	return &TelegramNotifier{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api.telegram.org",
	}
}

// Type identifies this notifier.
func (t *TelegramNotifier) Type() notification.ChannelType { return notification.TypeTelegram }

type telegramRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
}

// Send formats and delivers a message. Errors from the Telegram API (bad token,
// wrong chat id, …) are surfaced so the caller can show them to the user.
func (t *TelegramNotifier) Send(ctx context.Context, ch notification.Decrypted, msg notification.Message) error {
	chatID := strings.TrimSpace(ch.Config["chat_id"])
	if chatID == "" {
		return fmt.Errorf("telegram: chat_id is not configured")
	}
	if ch.Secret == "" {
		return fmt.Errorf("telegram: bot token is not configured")
	}

	body, err := json.Marshal(telegramRequest{
		ChatID:                chatID,
		Text:                  formatMessage(msg),
		ParseMode:             "HTML",
		DisableWebPagePreview: true,
	})
	if err != nil {
		return fmt.Errorf("telegram: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, ch.Secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	var tr telegramResponse
	_ = json.NewDecoder(resp.Body).Decode(&tr)
	if resp.StatusCode != http.StatusOK || !tr.OK {
		desc := tr.Description
		if desc == "" {
			desc = resp.Status
		}
		return fmt.Errorf("telegram API error: %s", desc)
	}
	return nil
}

// formatMessage renders a Message as Telegram-flavored HTML.
func formatMessage(msg notification.Message) string {
	var b strings.Builder

	b.WriteString(header(msg))
	b.WriteString("\n\n")
	if msg.Title != "" {
		fmt.Fprintf(&b, "<b>%s</b>\n", esc(msg.Title))
	}

	writeField(&b, "Severity", strings.ToUpper(msg.Severity))
	writeField(&b, "Monitor", msg.MonitorName)
	writeField(&b, "Type", strings.ToUpper(msg.MonitorType))
	writeField(&b, "Target", msg.Target)
	writeField(&b, "Project", msg.Project)
	writeField(&b, "Environment", msg.Environment)
	if !msg.Timestamp.IsZero() {
		writeField(&b, "Time", msg.Timestamp.Format("2006-01-02 15:04:05 MST"))
	}
	if msg.Duration > 0 {
		writeField(&b, "Duration", humanizeDuration(msg.Duration))
	}
	if msg.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", esc(msg.Description))
	}
	if msg.DashboardURL != "" {
		fmt.Fprintf(&b, "\n<a href=\"%s\">Open dashboard</a>", esc(msg.DashboardURL))
	}
	return b.String()
}

func header(msg notification.Message) string {
	if msg.IsTest {
		return "🔔 <b>Test notification</b>"
	}
	switch msg.Status {
	case notification.StatusResolved:
		return "✅ <b>RESOLVED</b>"
	default:
		if strings.EqualFold(msg.Severity, "critical") {
			return "🔴 <b>FIRING — CRITICAL</b>"
		}
		return "🟠 <b>FIRING — " + strings.ToUpper(esc(msg.Severity)) + "</b>"
	}
}

func writeField(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(b, "<b>%s:</b> %s\n", label, esc(value))
}

func esc(s string) string { return html.EscapeString(s) }

// humanizeDuration renders a duration compactly (e.g. "2m", "1h5m", "3s").
func humanizeDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) - h*60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}
