package notifier

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html"
	"net"
	"net/smtp"
	"strings"
	"time"

	"beacon/internal/domain/notification"
)

// EmailNotifier delivers alerts over SMTP.
//
// SMTP config is per-channel (host/port/from/to/username, password as the secret)
// so a self-hosted operator can point at their own relay without any platform
// email service. Unlike the webhook path, the SMTP host is NOT run through
// safehttp's private-address block: internal mail relays (10.x, an in-cluster
// Postfix) are a legitimate and common setup, and SMTP is not a useful SSRF
// reflection primitive the way an HTTP fetch is. We still bound the dial and the
// whole exchange with a timeout.
type EmailNotifier struct {
	timeout time.Duration
	// dial is injectable so tests can run against an in-memory SMTP server.
	dial func(ctx context.Context, addr string) (net.Conn, error)
}

// NewEmailNotifier builds an EmailNotifier.
func NewEmailNotifier() *EmailNotifier {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return &EmailNotifier{
		timeout: 15 * time.Second,
		dial:    func(ctx context.Context, addr string) (net.Conn, error) { return d.DialContext(ctx, "tcp", addr) },
	}
}

// Type identifies this notifier.
func (e *EmailNotifier) Type() notification.ChannelType { return notification.TypeEmail }

// Send composes and delivers the message.
func (e *EmailNotifier) Send(ctx context.Context, ch notification.Decrypted, msg notification.Message) error {
	cfg, err := parseEmailConfig(ch)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	conn, err := e.dial(ctx, net.JoinHostPort(cfg.host, cfg.port))
	if err != nil {
		return fmt.Errorf("email: connect to %s: %w", cfg.host, err)
	}

	// Implicit TLS (port 465) wraps the socket immediately; STARTTLS (587) upgrades
	// after EHLO. "none" is plaintext, for a trusted internal relay only.
	if cfg.security == "tls" {
		conn = tls.Client(conn, &tls.Config{ServerName: cfg.host})
	}

	client, err := smtp.NewClient(conn, cfg.host)
	if err != nil {
		return fmt.Errorf("email: smtp handshake: %w", err)
	}
	defer func() { _ = client.Close() }()

	if cfg.security == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("email: server does not support STARTTLS (set security=tls or none)")
		}
		if err := client.StartTLS(&tls.Config{ServerName: cfg.host}); err != nil {
			return fmt.Errorf("email: STARTTLS: %w", err)
		}
	}

	if cfg.username != "" {
		auth := smtp.PlainAuth("", cfg.username, ch.Secret, cfg.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email: authentication failed: %w", err)
		}
	}

	if err := client.Mail(cfg.from); err != nil {
		return fmt.Errorf("email: MAIL FROM rejected: %w", err)
	}
	for _, rcpt := range cfg.to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("email: RCPT %s rejected: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email: DATA: %w", err)
	}
	if _, err := w.Write([]byte(buildMIME(cfg, msg))); err != nil {
		return fmt.Errorf("email: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: finalize body: %w", err)
	}
	return client.Quit()
}

type emailConfig struct {
	host, port, from, username, security string
	to                                   []string
}

func parseEmailConfig(ch notification.Decrypted) (emailConfig, error) {
	c := emailConfig{
		host:     strings.TrimSpace(ch.Config["host"]),
		port:     strings.TrimSpace(ch.Config["port"]),
		from:     strings.TrimSpace(ch.Config["from"]),
		username: strings.TrimSpace(ch.Config["username"]),
		security: strings.ToLower(strings.TrimSpace(ch.Config["security"])),
	}
	if c.host == "" {
		return c, fmt.Errorf("email: host is not configured")
	}
	if c.from == "" {
		return c, fmt.Errorf("email: from address is not configured")
	}
	// "to" is a comma-separated list in the plaintext config.
	for _, r := range strings.Split(ch.Config["to"], ",") {
		if r = strings.TrimSpace(r); r != "" {
			c.to = append(c.to, r)
		}
	}
	if len(c.to) == 0 {
		return c, fmt.Errorf("email: no recipients configured")
	}
	if c.port == "" {
		c.port = "587"
	}
	if c.security == "" {
		c.security = "starttls" // safe default
	}
	switch c.security {
	case "starttls", "tls", "none":
	default:
		return c, fmt.Errorf("email: security must be starttls, tls or none")
	}
	return c, nil
}

// buildMIME renders a multipart/alternative message: a plaintext part (always
// readable) and a minimal HTML part. CRLF line endings per RFC 5322.
func buildMIME(cfg emailConfig, msg notification.Message) string {
	subject := "[BEACON] " + statusHeadline(msg)
	boundary := "beacon-boundary-9d7f2a"

	var b strings.Builder
	crlf := func(s string) { b.WriteString(s); b.WriteString("\r\n") }

	crlf("From: " + cfg.from)
	crlf("To: " + strings.Join(cfg.to, ", "))
	crlf("Subject: " + mimeEncodeHeader(subject))
	crlf("MIME-Version: 1.0")
	crlf("Date: " + msg.Timestamp.UTC().Format(time.RFC1123Z))
	crlf(`Content-Type: multipart/alternative; boundary="` + boundary + `"`)
	crlf("")

	// text/plain
	crlf("--" + boundary)
	crlf("Content-Type: text/plain; charset=UTF-8")
	crlf("")
	b.WriteString(strings.ReplaceAll(plainBody(msg), "\n", "\r\n"))
	crlf("")

	// text/html
	crlf("--" + boundary)
	crlf("Content-Type: text/html; charset=UTF-8")
	crlf("")
	b.WriteString(htmlBody(msg))
	crlf("")

	crlf("--" + boundary + "--")
	return b.String()
}

func htmlBody(msg notification.Message) string {
	var rows strings.Builder
	for _, l := range detailLines(msg) {
		k, v, ok := strings.Cut(l, ": ")
		if !ok {
			continue
		}
		fmt.Fprintf(&rows,
			`<tr><td style="padding:4px 12px 4px 0;color:#64748b;white-space:nowrap">%s</td><td style="padding:4px 0;color:#0f172a">%s</td></tr>`,
			html.EscapeString(k), html.EscapeString(v))
	}
	button := ""
	if msg.DashboardURL != "" {
		button = fmt.Sprintf(
			`<p style="margin-top:16px"><a href="%s" style="background:#0f172a;color:#fff;padding:10px 18px;border-radius:8px;text-decoration:none;display:inline-block">Open in Beacon</a></p>`,
			html.EscapeString(msg.DashboardURL))
	}
	return fmt.Sprintf(`<!doctype html><html><body style="font-family:system-ui,-apple-system,sans-serif;color:#0f172a;max-width:560px;margin:0 auto;padding:16px">
<div style="border-left:4px solid %s;padding-left:12px;margin-bottom:16px"><h2 style="margin:0;font-size:18px">%s</h2></div>
<table style="border-collapse:collapse;font-size:14px">%s</table>%s
</body></html>`, severityHex(msg), html.EscapeString(statusHeadline(msg)), rows.String(), button)
}

// mimeEncodeHeader RFC 2047-encodes a header value only if it contains non-ASCII,
// so plain subjects stay readable in a raw message.
func mimeEncodeHeader(s string) string {
	for _, r := range s {
		if r > 127 {
			return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
		}
	}
	return s
}
