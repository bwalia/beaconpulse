package postgres

import (
	"context"

	"github.com/google/uuid"

	"beacon/internal/domain/notification"
)

// ProjectLookupAdapter adapts the ProjectRepository to notification.ProjectLookup
// so the dispatcher can enrich alert messages with a project's name and
// environment without the notification context depending on the project one.
type ProjectLookupAdapter struct {
	repo *ProjectRepository
}

// NewProjectLookupAdapter builds the adapter.
func NewProjectLookupAdapter(repo *ProjectRepository) *ProjectLookupAdapter {
	return &ProjectLookupAdapter{repo: repo}
}

var _ notification.ProjectLookup = (*ProjectLookupAdapter)(nil)

// Project returns the project's name and environment, or found=false if it
// cannot be resolved (best-effort; never blocks delivery).
func (a *ProjectLookupAdapter) Project(ctx context.Context, orgID, projectID uuid.UUID) (string, string, bool) {
	p, err := a.repo.GetByID(ctx, orgID, projectID)
	if err != nil {
		return "", "", false
	}
	return p.Name, string(p.Environment), true
}
