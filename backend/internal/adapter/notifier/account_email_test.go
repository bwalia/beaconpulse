package notifier

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordResetMIME(t *testing.T) {
	cfg := emailConfig{from: "no-reply@sysops247.com", to: []string{"jane@example.com"}}
	link := "https://sysops247.com/reset-password?token=abc&x=1"
	out := passwordResetMIME("SysOps 24/7", cfg, "<Jane>", link, time.Unix(0, 0))

	if !strings.Contains(out, "Subject: Reset your SysOps 24/7 password\r\n") {
		t.Fatal("subject should name the brand")
	}
	if !strings.Contains(out, link) {
		t.Fatal("plaintext part must carry the raw link")
	}
	if !strings.Contains(out, "token=abc&amp;x=1") {
		t.Fatal("HTML part must escape the link")
	}
	if strings.Contains(out, "<Jane>") && !strings.Contains(out, "Hi <Jane>,\r\n") {
		t.Fatal("the name may appear raw only in the plaintext part")
	}
	if !strings.Contains(out, "&lt;Jane&gt;") {
		t.Fatal("HTML part must escape the user's name")
	}
}
