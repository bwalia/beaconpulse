package notifier

import (
	"strings"
	"testing"
	"time"

	"beacon/internal/domain/notification"
)

func TestFormatMessageFiring(t *testing.T) {
	msg := notification.Message{
		Status:       notification.StatusFiring,
		Severity:     "critical",
		Title:        "Marketing site is down",
		MonitorName:  "Marketing site",
		MonitorType:  "https",
		Target:       "https://example.com",
		Project:      "Production",
		Environment:  "production",
		Description:  "Failed health check",
		Timestamp:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Duration:     3 * time.Minute,
		DashboardURL: "http://localhost:3400",
	}
	out := formatMessage(msg)

	for _, want := range []string{
		"FIRING — CRITICAL", "Marketing site", "https://example.com",
		"Production", "Duration:", "Open dashboard",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("formatted message missing %q\n%s", want, out)
		}
	}
}

func TestFormatMessageResolvedAndEscaping(t *testing.T) {
	msg := notification.Message{
		Status:      notification.StatusResolved,
		MonitorName: "a<b>&c",
		Title:       "resolved",
	}
	out := formatMessage(msg)
	if !strings.Contains(out, "✅ <b>RESOLVED</b>") {
		t.Errorf("expected resolved header, got:\n%s", out)
	}
	// The angle brackets in the monitor name must be HTML-escaped.
	if !strings.Contains(out, "a&lt;b&gt;&amp;c") {
		t.Errorf("expected escaped monitor name, got:\n%s", out)
	}
}

func TestHumanizeDuration(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Second:             "30s",
		90 * time.Second:             "1m",
		time.Hour:                    "1h",
		time.Hour + 5*time.Minute:    "1h5m",
		2*time.Hour + 30*time.Minute: "2h30m",
	}
	for d, want := range cases {
		if got := humanizeDuration(d); got != want {
			t.Errorf("humanizeDuration(%s) = %q, want %q", d, got, want)
		}
	}
}
