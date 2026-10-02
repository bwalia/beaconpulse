package notifier

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"beacon/internal/domain/notification"
)

// AccountMailer sends transactional account email (password reset) over the platform
// SMTP relay — the same BEACON_DEFAULT_SMTP_* relay the alert fallback uses. It
// satisfies auth.Mailer.
type AccountMailer struct {
	cfg   DefaultEmailConfig
	email *EmailNotifier
}

// NewAccountMailer builds an AccountMailer. brand is the product name in the copy.
func NewAccountMailer(cfg DefaultEmailConfig, brand string) *AccountMailer {
	return &AccountMailer{cfg: cfg, email: NewEmailNotifier(brand)}
}

// SendPasswordReset emails a password-reset link to one recipient.
func (m *AccountMailer) SendPasswordReset(ctx context.Context, to, name, link string) error {
	// Validate and default the relay settings exactly as a configured channel would.
	cfg, err := parseEmailConfig(notification.Decrypted{Config: map[string]string{
		"host": m.cfg.Host, "port": m.cfg.Port, "from": m.cfg.From,
		"username": m.cfg.Username, "security": m.cfg.Security, "to": to,
	}})
	if err != nil {
		return err
	}
	return m.email.deliver(ctx, cfg, m.cfg.Password, passwordResetMIME(brandOr(m.email.brand), cfg, name, link, time.Now()))
}

// passwordResetMIME renders the reset email as plaintext + HTML. Only the brand (our
// config) reaches a header; the user's name and the link go in the body, escaped.
func passwordResetMIME(brand string, cfg emailConfig, name, link string, now time.Time) string {
	const boundary = "beacon-boundary-reset-41c8"
	greeting := "Hi,"
	if name = strings.TrimSpace(name); name != "" {
		greeting = "Hi " + name + ","
	}

	plain := fmt.Sprintf("%s\n\nSomeone (hopefully you) asked to reset the password for your %s account.\n"+
		"Open this link to choose a new one. It works once and expires in 1 hour:\n\n%s\n\n"+
		"If you didn't ask for this, ignore this email — your password won't change.\n", greeting, brand, link)

	htmlPart := fmt.Sprintf(`<!doctype html><html><body style="font-family:system-ui,-apple-system,sans-serif;color:#0f172a;max-width:560px;margin:0 auto;padding:16px">
<p>%s</p>
<p>Someone (hopefully you) asked to reset the password for your %s account. Choose a new one below — the link works once and expires in 1 hour.</p>
<p style="margin:24px 0"><a href="%s" style="background:#0f172a;color:#fff;padding:12px 20px;border-radius:8px;text-decoration:none;display:inline-block">Reset password</a></p>
<p style="color:#64748b;font-size:13px">Or paste this link into your browser:<br><span style="word-break:break-all">%s</span></p>
<p style="color:#64748b;font-size:13px">If you didn't ask for this, ignore this email — your password won't change.</p>
</body></html>`, html.EscapeString(greeting), html.EscapeString(brand), html.EscapeString(link), html.EscapeString(link))

	var b strings.Builder
	crlf := func(s string) { b.WriteString(s); b.WriteString("\r\n") }
	crlf("From: " + cfg.from)
	crlf("To: " + strings.Join(cfg.to, ", "))
	crlf("Subject: " + mimeEncodeHeader("Reset your "+brand+" password"))
	crlf("MIME-Version: 1.0")
	crlf("Date: " + now.UTC().Format(time.RFC1123Z))
	crlf(`Content-Type: multipart/alternative; boundary="` + boundary + `"`)
	crlf("")
	crlf("--" + boundary)
	crlf("Content-Type: text/plain; charset=UTF-8")
	crlf("")
	b.WriteString(strings.ReplaceAll(plain, "\n", "\r\n"))
	crlf("")
	crlf("--" + boundary)
	crlf("Content-Type: text/html; charset=UTF-8")
	crlf("")
	b.WriteString(htmlPart)
	crlf("")
	crlf("--" + boundary + "--")
	return b.String()
}
