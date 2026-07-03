package project

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/monitor"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/logger"
	"beacon/internal/platform/slug"
)

// CreateInput is the validated payload for creating a project.
type CreateInput struct {
	Name        string
	Slug        string // optional; derived from Name when empty
	Description string
	Environment Environment
	IsActive    *bool
}

// UpdateInput is the payload for a partial update. Nil fields are left unchanged.
type UpdateInput struct {
	Name        *string
	Description *string
	Environment *Environment
	IsActive    *bool
}

// Service implements project use cases.
type Service struct {
	repo     Repository
	syncer   monitor.Syncer
	auditlog audit.Recorder
	now      func() time.Time
}

// NewService wires the project service. syncer may be nil in contexts that do
// not manage the control plane (e.g. read-only tests).
func NewService(repo Repository, syncer monitor.Syncer, auditlog audit.Recorder) *Service {
	return &Service{repo: repo, syncer: syncer, auditlog: auditlog, now: time.Now}
}

// Create adds a project to the actor's organization.
func (s *Service) Create(ctx context.Context, actor Actor, in CreateInput) (*Project, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit creating projects")
	}
	env := in.Environment
	if env == "" {
		env = EnvProduction
	}
	if !env.Valid() {
		return nil, apperror.Validation("invalid environment",
			apperror.FieldError{Field: "environment", Message: "must be production, staging or development"})
	}

	desired := in.Slug
	if desired == "" {
		desired = in.Name
	}
	uniqueSlug, err := s.uniqueSlug(ctx, actor.OrgID, desired, uuid.Nil)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	p := &Project{
		ID:          uuid.New(),
		OrgID:       actor.OrgID,
		Name:        strings.TrimSpace(in.Name),
		Slug:        uniqueSlug,
		Description: strings.TrimSpace(in.Description),
		Environment: env,
		IsActive:    active,
		CreatedBy:   &actor.UserID,
		UpdatedBy:   &actor.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	s.record(ctx, actor, audit.ActionProjectCreated, p.ID, map[string]any{"name": p.Name, "slug": p.Slug})
	return p, nil
}

// Get returns a single project scoped to the actor's org.
func (s *Service) Get(ctx context.Context, actor Actor, id uuid.UUID) (*Project, error) {
	return s.repo.GetByID(ctx, actor.OrgID, id)
}

// List returns a page of projects for the actor's org.
func (s *Service) List(ctx context.Context, actor Actor, f ListFilter) ([]Project, int, error) {
	normalizeFilter(&f)
	return s.repo.List(ctx, actor.OrgID, f)
}

// Update applies a partial update to a project.
func (s *Service) Update(ctx context.Context, actor Actor, id uuid.UUID, in UpdateInput) (*Project, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit updating projects")
	}
	p, err := s.repo.GetByID(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}

	changed := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, apperror.Validation("name must not be empty",
				apperror.FieldError{Field: "name", Message: "is required"})
		}
		p.Name = name
		changed["name"] = name
	}
	if in.Description != nil {
		p.Description = strings.TrimSpace(*in.Description)
		changed["description"] = p.Description
	}
	if in.Environment != nil {
		if !in.Environment.Valid() {
			return nil, apperror.Validation("invalid environment",
				apperror.FieldError{Field: "environment", Message: "must be production, staging or development"})
		}
		p.Environment = *in.Environment
		changed["environment"] = string(*in.Environment)
	}
	if in.IsActive != nil {
		p.IsActive = *in.IsActive
		changed["is_active"] = *in.IsActive
	}

	p.UpdatedBy = &actor.UserID
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	s.record(ctx, actor, audit.ActionProjectUpdated, p.ID, changed)
	return p, nil
}

// Delete soft-deletes a project (and, by FK cascade at the query level, is
// expected to cascade to its monitors in the repository implementation).
func (s *Service) Delete(ctx context.Context, actor Actor, id uuid.UUID) error {
	if !actor.Role.CanWrite() {
		return apperror.Forbidden("your role does not permit deleting projects")
	}
	// Ensure it exists and belongs to the org before deleting.
	if _, err := s.repo.GetByID(ctx, actor.OrgID, id); err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, actor.OrgID, id, actor.UserID); err != nil {
		return err
	}
	// Deleting a project cascades a soft-delete to its monitors, so the control
	// plane must be reconciled to stop probing them.
	if s.syncer != nil {
		if err := s.syncer.Sync(ctx); err != nil {
			logger.FromContext(ctx).Warn("control-plane sync after project delete failed; will reconcile later",
				slog.String("error", err.Error()))
		}
	}
	s.record(ctx, actor, audit.ActionProjectDeleted, id, nil)
	return nil
}

// ---- internals ----

func (s *Service) uniqueSlug(ctx context.Context, orgID uuid.UUID, desired string, excludeID uuid.UUID) (string, error) {
	base := slug.Make(desired, "project")
	candidate := base
	for attempt := 0; attempt < 5; attempt++ {
		exists, err := s.repo.SlugExists(ctx, orgID, candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		suffix := uuid.NewString()[:6]
		trimmed := base
		if max := 63 - len(suffix) - 1; len(trimmed) > max {
			trimmed = strings.Trim(trimmed[:max], "-")
		}
		candidate = trimmed + "-" + suffix
	}
	return "", apperror.Conflict("could not allocate a unique project slug")
}

func (s *Service) record(ctx context.Context, actor Actor, action audit.Action, resourceID uuid.UUID, md map[string]any) {
	org := actor.OrgID
	uid := actor.UserID
	_ = s.auditlog.Record(ctx, audit.Entry{
		OrgID:        &org,
		UserID:       &uid,
		Action:       action,
		ResourceType: "project",
		ResourceID:   resourceID.String(),
		Metadata:     md,
	})
}

func normalizeFilter(f *ListFilter) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	f.Search = strings.TrimSpace(f.Search)
}
