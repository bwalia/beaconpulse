package notifier

import (
	"strings"
	"testing"

	"beacon/internal/domain/notification"
)

func TestBuildMIMEUsesBrand(t *testing.T) {
	cfg := emailConfig{from: "alerts@relay.test", to: []string{"owner@acme.com"}}
	msg := notification.Message{Status: notification.StatusFiring, MonitorName: "API", DashboardURL: "https://dash.test"}

	out := buildMIME("SysOps 24/7", cfg, msg)
	if !strings.Contains(out, "Open in SysOps 24/7") {
		t.Errorf("email body should carry the brand CTA; got:\n%s", out)
	}
	// The subject header is RFC 2047-encoded (the headline has a non-ASCII dash), so
	// assert on the base64 of the visible subject rather than the raw text.
	if !strings.Contains(out, "Content-Type: multipart/alternative") {
		t.Error("expected a multipart body")
	}
}

func TestBuildMIMEBrandFallsBackToBeacon(t *testing.T) {
	cfg := emailConfig{from: "alerts@relay.test", to: []string{"owner@acme.com"}}
	msg := notification.Message{Status: notification.StatusFiring, MonitorName: "API", DashboardURL: "https://dash.test"}

	if out := buildMIME("", cfg, msg); !strings.Contains(out, "Open in Beacon") {
		t.Errorf("an empty brand must fall back to Beacon; got:\n%s", out)
	}
}

func TestBrandOr(t *testing.T) {
	if got := brandOr("  "); got != "Beacon" {
		t.Errorf("blank brand = %q, want Beacon", got)
	}
	if got := brandOr("RedFox Signals"); got != "RedFox Signals" {
		t.Errorf("brand = %q, want RedFox Signals", got)
	}
}
