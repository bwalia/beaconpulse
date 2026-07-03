package project

import (
	"context"

	"github.com/google/uuid"
)

// ListFilter narrows and paginates a project listing. All listings are implicitly
// scoped to the caller's organization by the service.
type ListFilter struct {
	Search      string
	Environment string
	Limit       int
	Offset      int
}

// Repository persists projects. All methods are org-scoped: an orgID mismatch
// must behave as "not found" so tenants cannot probe each other's data.
type Repository interface {
	Create(ctx context.Context, p *Project) error
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*Project, error)
	List(ctx context.Context, orgID uuid.UUID, f ListFilter) (items []Project, total int, err error)
	Update(ctx context.Context, p *Project) error
	SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error
	SlugExists(ctx context.Context, orgID uuid.UUID, slug string, excludeID uuid.UUID) (bool, error)
}
