package monitor

import (
	"context"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/plan"
)

// StatusUpdate is an observed status for a monitor, produced by reading probe
// results from Prometheus and applied back to the cached last_status column.
type StatusUpdate struct {
	MonitorID uuid.UUID
	Status    Status
	CheckedAt time.Time
}

// GitHubRun is one recorded github_actions workflow run — the history behind a
// push monitor, which otherwise keeps only its latest status. Status is our
// classification; Conclusion is GitHub's raw word. The rest is context for the
// per-run tooltip and the link back to the run on github.com.
type GitHubRun struct {
	MonitorID  uuid.UUID
	Status     Status
	Conclusion string
	Workflow   string
	RunNumber  string
	RunAttempt string
	Branch     string
	SHA        string
	Actor      string
	EventName  string
	RunURL     string
	CreatedAt  time.Time
}

// ListFilter narrows and paginates a monitor listing within an organization.
type ListFilter struct {
	ProjectID *uuid.UUID
	Type      string
	Status    string
	Enabled   *bool
	Search    string
	Limit     int
	Offset    int
}

// Repository persists monitors. Tenant-scoped methods take an orgID; a mismatch
// must behave as "not found".
type Repository interface {
	Create(ctx context.Context, m *Monitor) error
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*Monitor, error)
	List(ctx context.Context, orgID uuid.UUID, f ListFilter) (items []Monitor, total int, err error)
	Update(ctx context.Context, m *Monitor) error
	SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error
	SetEnabled(ctx context.Context, orgID, id uuid.UUID, enabled bool, updatedBy uuid.UUID) error

	// ProjectExists verifies a project belongs to the org, so monitors cannot be
	// attached to another tenant's or a non-existent project.
	ProjectExists(ctx context.Context, orgID, projectID uuid.UUID) (bool, error)

	// CountByOrg returns the number of non-deleted monitors an org has, for
	// plan-limit enforcement.
	CountByOrg(ctx context.Context, orgID uuid.UUID) (int, error)

	// ListAllEnabled returns every enabled, non-deleted monitor across all
	// organizations. The control plane uses this to regenerate the full
	// Prometheus/Blackbox configuration from the source of truth.
	ListAllEnabled(ctx context.Context) ([]Monitor, error)

	// ApplyStatusUpdates writes observed statuses back to monitors (last_status
	// and last_checked_at). Paused monitors are left untouched. Returns the
	// number of rows updated.
	ApplyStatusUpdates(ctx context.Context, updates []StatusUpdate) (int64, error)

	// RecordGitHubRun appends one github_actions run to the history log. Shared
	// with the githubactions ingest context, which satisfies this same repository.
	RecordGitHubRun(ctx context.Context, run GitHubRun) error

	// ListGitHubRuns returns a monitor's recent runs, newest first (org-scoped: a
	// monitor in another org yields no rows). limit caps the result.
	ListGitHubRuns(ctx context.Context, orgID, monitorID uuid.UUID, limit int) ([]GitHubRun, error)
}

// OrgPlanReader resolves an organization's subscription plan. The service maps
// it to concrete limits via plan.LimitsFor. Implemented by an adapter over the
// organizations table.
type OrgPlanReader interface {
	Plan(ctx context.Context, orgID uuid.UUID) (plan.Plan, error)
}

// Syncer pushes the current desired monitor state into the control plane
// (regenerate config + reload Prometheus/Blackbox). The monitor and project
// services call Sync after any change. Implemented by the controlplane adapter.
type Syncer interface {
	Sync(ctx context.Context) error
}
