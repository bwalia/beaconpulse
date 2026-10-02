package notifier

import (
	"strings"
	"testing"
	"time"
)

func TestAccountMIME(t *testing.T) {
	cfg := emailConfig{from: "no-reply@sysops247.com", to: []string{"jane@example.com"}}
	link := "https://sysops247.com/reset-password?token=abc&x=1"
	e := accountEmail{subject: "Reset your SysOps 24/7 password", intro: "Choose a new password.", button: "Reset password", ignore: "Ignore if not you."}
	out := accountMIME(cfg, "<Jane>", link, e, time.Unix(0, 0))

	if !strings.Contains(out, "Subject: Reset your SysOps 24/7 password\r\n") {
		t.Fatal("subject should be the email's own subject")
	}
	if !strings.Contains(out, link) {
		t.Fatal("plaintext part must carry the raw link")
	}
	if !strings.Contains(out, "token=abc&amp;x=1") {
		t.Fatal("HTML part must escape the link")
	}
	if !strings.Contains(out, "&lt;Jane&gt;") {
		t.Fatal("HTML part must escape the user's name")
	}
	if !strings.Contains(out, ">Reset password</a>") {
		t.Fatal("button label should render")
	}
}
