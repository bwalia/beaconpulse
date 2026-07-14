// Package maintenance models planned-downtime windows.
//
// A window declares "expect these monitors to be down, on purpose, from start to
// end." While active it does two things: it suppresses alert notifications for the
// covered monitors (checked once in the notification Dispatcher, so it covers
// probed monitors and heartbeats uniformly), and it relabels those monitors on the
// public status page as "under maintenance" rather than "down". A routine deploy no
// longer pages the on-call rotation or shows a false outage to the customer's users.
//
// A window is a loaded gun: a permanent org-wide window silently blinds the whole
// org. So writes require a writer role and are audited, and suppression is
// deliberately observable — a suppressed alert is recorded, never silently dropped.
package maintenance

import (
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
)

// Scope selects which monitors a window covers. One model, three grains.
type Scope string

const (
	// ScopeOrg covers every monitor in the org; ScopeIDs is empty.
	ScopeOrg Scope = "org"
	// ScopeProject covers the monitors in the listed projects (ScopeIDs = project ids).
	ScopeProject Scope = "project"
	// ScopeMonitor covers exactly the listed monitors (ScopeIDs = monitor ids).
	ScopeMonitor Scope = "monitor"
)

// ValidScope reports whether s is a scope Beacon understands.
func ValidScope(s Scope) bool {
	switch s {
	case ScopeOrg, ScopeProject, ScopeMonitor:
		return true
	default:
		return false
	}
}

// Window is a planned-downtime period for a set of monitors.
type Window struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Title       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	Scope       Scope
	// ScopeIDs holds project ids (ScopeProject) or monitor ids (ScopeMonitor);
	// empty for ScopeOrg. Never nil once persisted.
	ScopeIDs  []uuid.UUID
	CreatedBy *uuid.UUID
	UpdatedBy *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Active reports whether the window covers instant t (start inclusive, end
// exclusive — the monitor is back under normal alerting the moment the window ends).
func (w *Window) Active(t time.Time) bool {
	return !t.Before(w.StartsAt) && t.Before(w.EndsAt)
}

// Actor is the authenticated caller performing a window operation.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}
