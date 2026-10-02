package notifier

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"beacon/internal/domain/notification"
)

// AccountMailer sends transactional account email (password reset, email
// confirmation) over the platform SMTP relay — the same BEACON_DEFAULT_SMTP_* relay
// the alert fallback uses. It satisfies auth.Mailer.
type AccountMailer struct {
	cfg   DefaultEmailConfig
	email *EmailNotifier
}

// NewAccountMailer builds an AccountMailer. brand is the product name in the copy.
func NewAccountMailer(cfg DefaultEmailConfig, brand string) *AccountMailer {
	return &AccountMailer{cfg: cfg, email: NewEmailNotifier(brand)}
}

// accountEmail is one single-action account email: a greeting, an explanation, a
// button, and what to do if it wasn't you.
type accountEmail struct {
	subject, intro, button, ignore string
}

// SendPasswordReset emails a password-reset link to one recipient.
func (m *AccountMailer) SendPasswordReset(ctx context.Context, to, name, link string) error {
	brand := brandOr(m.email.brand)
	return m.send(ctx, to, name, link, accountEmail{
		subject: "Reset your " + brand + " password",
		intro:   "Someone (hopefully you) asked to reset the password for your " + brand + " account. Choose a new one with the link below — it works once and expires in 1 hour.",
		button:  "Reset password",
		ignore:  "If you didn't ask for this, ignore this email — your password won't change.",
	})
}

// SendEmailVerification emails an address-confirmation link to one recipient.
func (m *AccountMailer) SendEmailVerification(ctx context.Context, to, name, link string) error {
	brand := brandOr(m.email.brand)
	return m.send(ctx, to, name, link, accountEmail{
		subject: "Confirm your email for " + brand,
		intro:   "Confirm this is your email address so " + brand + " can reach you with alerts and account messages. The link expires in 3 days.",
		button:  "Confirm email",
		ignore:  "If you didn't create a " + brand + " account, ignore this email.",
	})
}

func (m *AccountMailer) send(ctx context.Context, to, name, link string, e accountEmail) error {
	// Validate and default the relay settings exactly as a configured channel would.
	cfg, err := parseEmailConfig(notification.Decrypted{Config: map[string]string{
		"host": m.cfg.Host, "port": m.cfg.Port, "from": m.cfg.From,
		"username": m.cfg.Username, "security": m.cfg.Security, "to": to,
	}})
	if err != nil {
		return err
	}
	return m.email.deliver(ctx, cfg, m.cfg.Password, accountMIME(cfg, name, link, e, time.Now()))
}

// accountMIME renders an account email as plaintext + HTML. Only our own copy (built
// from the brand config) reaches a header; the user's name and the link go in the
// body, escaped.
func accountMIME(cfg emailConfig, name, link string, e accountEmail, now time.Time) string {
	const boundary = "beacon-boundary-account-41c8"
	greeting := "Hi,"
	if name = strings.TrimSpace(name); name != "" {
		greeting = "Hi " + name + ","
	}

	plain := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n", greeting, e.intro, link, e.ignore)

	htmlPart := fmt.Sprintf(`<!doctype html><html><body style="font-family:system-ui,-apple-system,sans-serif;color:#0f172a;max-width:560px;margin:0 auto;padding:16px">
<p>%s</p>
<p>%s</p>
<p style="margin:24px 0"><a href="%s" style="background:#0f172a;color:#fff;padding:12px 20px;border-radius:8px;text-decoration:none;display:inline-block">%s</a></p>
<p style="color:#64748b;font-size:13px">Or paste this link into your browser:<br><span style="word-break:break-all">%s</span></p>
<p style="color:#64748b;font-size:13px">%s</p>
</body></html>`, html.EscapeString(greeting), html.EscapeString(e.intro), html.EscapeString(link),
		html.EscapeString(e.button), html.EscapeString(link), html.EscapeString(e.ignore))

	var b strings.Builder
	crlf := func(s string) { b.WriteString(s); b.WriteString("\r\n") }
	crlf("From: " + cfg.from)
	crlf("To: " + strings.Join(cfg.to, ", "))
	crlf("Subject: " + mimeEncodeHeader(e.subject))
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
