package notification

import (
	"time"

	"github.com/google/uuid"
)

// AlertStatus is whether an alert is firing or has resolved.
type AlertStatus string

const (
	StatusFiring   AlertStatus = "firing"
	StatusResolved AlertStatus = "resolved"
)

// AlertEvent is a single alert parsed from an Alertmanager webhook, normalized
// to the fields Beacon cares about. The transport layer builds these from the
// Alertmanager payload and hands them to the Dispatcher.
type AlertEvent struct {
	Status      AlertStatus
	AlertName   string
	Severity    string
	OrgID       uuid.UUID
	ProjectID   uuid.UUID
	MonitorID   string
	MonitorName string
	MonitorType string
	Target      string
	Summary     string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
}

// Duration returns how long the alert has been (or was) active.
func (e AlertEvent) Duration(now time.Time) time.Duration {
	end := now
	if e.Status == StatusResolved && !e.EndsAt.IsZero() {
		end = e.EndsAt
	}
	if e.StartsAt.IsZero() || end.Before(e.StartsAt) {
		return 0
	}
	return end.Sub(e.StartsAt)
}

// Message is the fully-rendered, channel-agnostic content a Notifier formats and
// sends. It carries every field the brief requires for a rich alert.
type Message struct {
	Status       AlertStatus
	Severity     string
	Title        string
	MonitorName  string
	MonitorType  string
	Target       string
	Project      string
	Environment  string
	Description  string
	Timestamp    time.Time
	Duration     time.Duration
	DashboardURL string
	// IsTest marks a "send test" message so notifiers can label it clearly.
	IsTest bool
	// Analysis is an optional AI triage of the alert (assessed severity, likely
	// cause, suggested fix). Nil when enrichment is disabled or unavailable.
	Analysis *AlertAnalysis
}
