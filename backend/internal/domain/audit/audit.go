// Package audit records security- and change-relevant actions to an append-only
// log. Services depend on the Recorder interface so audit writing never blocks
// or fails a business operation: a recording error is logged but not returned to
// the caller. Querying the log (for the Audit Logs dashboard page) uses the
// Repository interface.
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Action enumerates the audited operations. Kept as typed constants so call
// sites are consistent and greppable.
type Action string

const (
	ActionUserRegistered Action = "user.registered"
	ActionUserLogin      Action = "user.login"
	ActionUserLogout     Action = "user.logout"
	ActionTokenRefreshed Action = "token.refreshed"

	ActionProjectCreated Action = "project.created"
	ActionProjectUpdated Action = "project.updated"
	// ActionStatusPageUpdated — the org published/unpublished its public status
	// page, or renamed it. Security-relevant: it changes what anonymous visitors
	// can see.
	ActionStatusPageUpdated Action = "status_page.updated"
	ActionProjectDeleted    Action = "project.deleted"

	ActionMonitorCreated Action = "monitor.created"
	ActionMonitorUpdated Action = "monitor.updated"
	ActionMonitorDeleted Action = "monitor.deleted"
	ActionMonitorEnabled Action = "monitor.enabled"
	ActionMonitorPaused  Action = "monitor.paused"
)

// Entry is a single audit record.
type Entry struct {
	ID           uuid.UUID
	OrgID        *uuid.UUID
	UserID       *uuid.UUID
	Action       Action
	ResourceType string
	ResourceID   string
	Metadata     map[string]any
	IP           string
	UserAgent    string
	RequestID    string
	CreatedAt    time.Time
}

// Recorder appends entries to the audit log. Implementations should be
// best-effort: a failure to record must not abort the underlying operation.
type Recorder interface {
	Record(ctx context.Context, e Entry) error
}

// Repository reads audit entries for presentation.
type Repository interface {
	Insert(ctx context.Context, e *Entry) error
	List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]Entry, int, error)
}
