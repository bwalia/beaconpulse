package maintenance

import (
	"context"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/platform/apperror"
)

// maxTitleLen / maxDescriptionLen mirror the DB CHECK constraints so a bad input
// fails as a 422 with a field message rather than a 500 from Postgres.
const (
	maxTitleLen       = 200
	maxDescriptionLen = 2000
)

// CreateInput is the data required to open a maintenance window.
type CreateInput struct {
	Title       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	Scope       Scope
	ScopeIDs    []uuid.UUID
}

// UpdateInput is a partial update; nil fields are left unchanged. Scope and
// ScopeIDs are validated together against the resulting state so the pair can
// never drift out of agreement.
type UpdateInput struct {
	Title       *string
	Description *string
	StartsAt    *time.Time
	EndsAt      *time.Time
	Scope       *Scope
	ScopeIDs    *[]uuid.UUID
}

// Service is the maintenance-window use-case layer.
type Service struct {
	repo     Repository
	auditlog audit.Recorder
	now      func() time.Time
}

// NewService wires the window service. auditlog records writes; it is required.
func NewService(repo Repository, auditlog audit.Recorder) *Service {
	return &Service{repo: repo, auditlog: auditlog, now: time.Now}
}

// Create opens a new window. Writer role required.
func (s *Service) Create(ctx context.Context, actor Actor, in CreateInput) (*Window, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit creating maintenance windows")
	}
	w := &Window{
		ID:          uuid.New(),
		OrgID:       actor.OrgID,
		Title:       in.Title,
		Description: in.Description,
		StartsAt:    in.StartsAt.UTC(),
		EndsAt:      in.EndsAt.UTC(),
		Scope:       in.Scope,
		ScopeIDs:    dedupeIDs(in.ScopeIDs),
	}
	if err := validate(w); err != nil {
		return nil, err
	}
	uid := actor.UserID
	now := s.now().UTC()
	w.CreatedBy, w.UpdatedBy = &uid, &uid
	w.CreatedAt, w.UpdatedAt = now, now
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, audit.ActionMaintenanceCreated, w.ID, map[string]any{
		"title": w.Title, "scope": string(w.Scope), "ids": len(w.ScopeIDs),
	})
	return w, nil
}

// List returns the org's windows, newest-starting first, with a total count.
func (s *Service) List(ctx context.Context, actor Actor, f ListFilter) ([]Window, int, error) {
	return s.repo.List(ctx, actor.OrgID, f)
}

// Get returns one window, scoped to the actor's org.
func (s *Service) Get(ctx context.Context, actor Actor, id uuid.UUID) (*Window, error) {
	return s.repo.GetByID(ctx, actor.OrgID, id)
}

// Update mutates a window. Writer role required. The scope/scope-id pair is
// re-validated against the merged result.
func (s *Service) Update(ctx context.Context, actor Actor, id uuid.UUID, in UpdateInput) (*Window, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit modifying maintenance windows")
	}
	w, err := s.repo.GetByID(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if in.Title != nil {
		w.Title = *in.Title
	}
	if in.Description != nil {
		w.Description = *in.Description
	}
	if in.StartsAt != nil {
		w.StartsAt = in.StartsAt.UTC()
	}
	if in.EndsAt != nil {
		w.EndsAt = in.EndsAt.UTC()
	}
	if in.Scope != nil {
		w.Scope = *in.Scope
	}
	if in.ScopeIDs != nil {
		w.ScopeIDs = dedupeIDs(*in.ScopeIDs)
	}
	if err := validate(w); err != nil {
		return nil, err
	}
	uid := actor.UserID
	w.UpdatedBy = &uid
	w.UpdatedAt = s.now().UTC()
	if err := s.repo.Update(ctx, w); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, audit.ActionMaintenanceUpdated, w.ID, map[string]any{
		"title": w.Title, "scope": string(w.Scope), "ids": len(w.ScopeIDs),
	})
	return w, nil
}

// Delete soft-deletes a window. Writer role required. Re-reads first so a
// cross-org delete 404s rather than silently succeeding.
func (s *Service) Delete(ctx context.Context, actor Actor, id uuid.UUID) error {
	if !actor.Role.CanWrite() {
		return apperror.Forbidden("your role does not permit deleting maintenance windows")
	}
	if _, err := s.repo.GetByID(ctx, actor.OrgID, id); err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, actor.OrgID, id, actor.UserID); err != nil {
		return err
	}
	s.audit(ctx, actor, audit.ActionMaintenanceDeleted, id, nil)
	return nil
}

// IsSuppressed reports whether alerts for the monitor are currently suppressed by
// an active window at instant `at`. This satisfies notification.MaintenanceChecker
// so the Dispatcher can consult it directly.
func (s *Service) IsSuppressed(ctx context.Context, orgID, monitorID uuid.UUID, at time.Time) (bool, error) {
	return s.repo.ActiveForMonitor(ctx, orgID, monitorID, at)
}

// ActiveMonitorIDs returns the set of monitor ids in the org currently under an
// active window — used to badge the monitor and alert lists in one query.
func (s *Service) ActiveMonitorIDs(ctx context.Context, orgID uuid.UUID, at time.Time) (map[uuid.UUID]bool, error) {
	return s.repo.ActiveMonitorIDs(ctx, orgID, at)
}

func (s *Service) audit(ctx context.Context, actor Actor, action audit.Action, resourceID uuid.UUID, md map[string]any) {
	org := actor.OrgID
	uid := actor.UserID
	_ = s.auditlog.Record(ctx, audit.Entry{
		OrgID: &org, UserID: &uid, Action: action,
		ResourceType: "maintenance_window", ResourceID: resourceID.String(), Metadata: md,
	})
}

// validate enforces the window invariants that the API guarantees, mirroring the
// DB CHECK constraints so violations surface as clean 422s.
func validate(w *Window) error {
	var fields []apperror.FieldError
	if l := len(w.Title); l < 1 || l > maxTitleLen {
		fields = append(fields, apperror.FieldError{Field: "title", Message: "must be 1–200 characters"})
	}
	if len(w.Description) > maxDescriptionLen {
		fields = append(fields, apperror.FieldError{Field: "description", Message: "must be at most 2000 characters"})
	}
	if w.StartsAt.IsZero() || w.EndsAt.IsZero() {
		fields = append(fields, apperror.FieldError{Field: "starts_at", Message: "start and end are required"})
	} else if !w.EndsAt.After(w.StartsAt) {
		fields = append(fields, apperror.FieldError{Field: "ends_at", Message: "must be after starts_at"})
	}
	if !ValidScope(w.Scope) {
		fields = append(fields, apperror.FieldError{Field: "scope", Message: "must be one of org, project, monitor"})
	} else if w.Scope == ScopeOrg && len(w.ScopeIDs) > 0 {
		fields = append(fields, apperror.FieldError{Field: "scope_ids", Message: "org-scoped windows take no ids"})
	} else if w.Scope != ScopeOrg && len(w.ScopeIDs) == 0 {
		fields = append(fields, apperror.FieldError{Field: "scope_ids", Message: "at least one id is required for project/monitor scope"})
	}
	if len(fields) > 0 {
		return apperror.Validation("invalid maintenance window", fields...)
	}
	return nil
}

// dedupeIDs drops duplicates (and nil UUIDs) while preserving first-seen order, so
// a window's coverage list is canonical.
func dedupeIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return []uuid.UUID{}
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
