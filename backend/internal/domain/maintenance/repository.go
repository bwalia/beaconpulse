package maintenance

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ListFilter narrows and paginates a window listing. All windows are returned by
// default (newest-starting first); the caller pages with Limit/Offset.
type ListFilter struct {
	Limit  int
	Offset int
}

// Repository is the persistence port for maintenance windows. Every method is
// org-scoped and ignores soft-deleted rows.
type Repository interface {
	Create(ctx context.Context, w *Window) error
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*Window, error)
	List(ctx context.Context, orgID uuid.UUID, f ListFilter) (items []Window, total int, err error)
	Update(ctx context.Context, w *Window) error
	SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error

	// ActiveForMonitor reports whether an active window at instant `at` covers the
	// given monitor. It resolves the monitor's project internally, so callers need
	// only the org and monitor ids. This is the suppression hot path.
	ActiveForMonitor(ctx context.Context, orgID, monitorID uuid.UUID, at time.Time) (bool, error)
}
