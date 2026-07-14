package notifier

import (
	"fmt"
	"strings"

	"beacon/internal/domain/notification"
)

// Shared rendering helpers for the non-Telegram notifiers. Telegram keeps its own
// HTML formatter; Slack/Email/Webhook share these so their wording stays
// consistent and there is one place to change the copy.

// statusHeadline is the one-line summary, e.g. "DOWN — api.example.com".
func statusHeadline(msg notification.Message) string {
	verb := "DOWN"
	if msg.Status == notification.StatusResolved {
		verb = "RECOVERED"
	}
	if msg.IsTest {
		verb = "TEST"
	}
	name := msg.MonitorName
	if name == "" {
		name = msg.Title
	}
	return fmt.Sprintf("%s — %s", verb, name)
}

// severityHex maps a severity to a hex colour for the surfaces that support one
// (Slack attachment bar, HTML email). Recovery is always green regardless of the
// original severity: the useful signal is "it's back".
func severityHex(msg notification.Message) string {
	if msg.Status == notification.StatusResolved {
		return "#0ca30c" // VIZ.good
	}
	switch strings.ToLower(msg.Severity) {
	case "critical":
		return "#d03b3b" // VIZ.critical
	case "warning":
		return "#d97706" // VIZ.warning
	default:
		return "#6b7280" // slate
	}
}

// detailLines are the labelled facts shared by the plaintext renderers. Empty
// fields are skipped so a sparse message does not render blank rows.
func detailLines(msg notification.Message) []string {
	var out []string
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			out = append(out, fmt.Sprintf("%s: %s", k, v))
		}
	}
	add("Monitor", msg.MonitorName)
	add("Type", msg.MonitorType)
	add("Target", msg.Target)
	add("Project", msg.Project)
	add("Environment", msg.Environment)
	if msg.Severity != "" {
		add("Severity", strings.ToUpper(msg.Severity))
	}
	if msg.Status == notification.StatusResolved && msg.Duration > 0 {
		add("Downtime", msg.Duration.Round(1e9).String())
	}
	add("When", msg.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"))
	if msg.Description != "" {
		add("Details", msg.Description)
	}
	if msg.Analysis != nil {
		if msg.Analysis.LikelyCause != "" {
			add("Likely cause", msg.Analysis.LikelyCause)
		}
		if msg.Analysis.SuggestedFix != "" {
			add("Suggested fix", msg.Analysis.SuggestedFix)
		}
	}
	return out
}

// plainBody is the full plaintext rendering used by email's text part and as a
// webhook fallback: a headline, the details, and the dashboard link.
func plainBody(msg notification.Message) string {
	var b strings.Builder
	b.WriteString(statusHeadline(msg))
	b.WriteString("\n\n")
	for _, l := range detailLines(msg) {
		b.WriteString(l)
		b.WriteString("\n")
	}
	if msg.DashboardURL != "" {
		b.WriteString("\nDashboard: ")
		b.WriteString(msg.DashboardURL)
		b.WriteString("\n")
	}
	return b.String()
}
