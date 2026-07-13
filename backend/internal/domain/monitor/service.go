package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/logger"
)

// Default and boundary values for scheduling. Mirrors the DB CHECK constraints
// so violations are reported as friendly validation errors before hitting the
// database.
const (
	defaultInterval = 60
	minInterval     = 10
	maxInterval     = 86400
	defaultTimeout  = 10
	minTimeout      = 1
	maxTimeout      = 300
)

// CreateInput is the validated payload for creating a monitor.
type CreateInput struct {
	ProjectID uuid.UUID
	Name      string
	Type      Type
	Target    string
	Enabled   *bool
	// Public publishes the monitor on the org's status page. Nil => false: a new
	// monitor is never published by accident.
	Public          *bool
	IntervalSeconds int
	TimeoutSeconds  int
	Settings        Settings
}

// UpdateInput is a partial update; nil fields are unchanged.
type UpdateInput struct {
	Name            *string
	Target          *string
	Enabled         *bool
	Public          *bool
	IntervalSeconds *int
	TimeoutSeconds  *int
	Settings        *Settings
}

// Service implements monitor use cases and triggers control-plane syncs.
type Service struct {
	repo     Repository
	syncer   Syncer
	plans    OrgPlanReader
	auditlog audit.Recorder
	now      func() time.Time
}

// NewService wires the monitor service.
func NewService(repo Repository, syncer Syncer, plans OrgPlanReader, auditlog audit.Recorder) *Service {
	return &Service{repo: repo, syncer: syncer, plans: plans, auditlog: auditlog, now: time.Now}
}

// Usage summarizes an org's monitor usage against its plan limits.
type Usage struct {
	Plan               string
	MonitorsUsed       int
	MaxMonitors        int
	MinIntervalSeconds int
}

// Usage returns the caller org's current monitor usage and plan limits.
func (s *Service) Usage(ctx context.Context, actor Actor) (*Usage, error) {
	limits, planName, err := s.orgLimits(ctx, actor.OrgID)
	if err != nil {
		return nil, err
	}
	used, err := s.repo.CountByOrg(ctx, actor.OrgID)
	if err != nil {
		return nil, err
	}
	return &Usage{
		Plan:               string(planName),
		MonitorsUsed:       used,
		MaxMonitors:        limits.MaxMonitors,
		MinIntervalSeconds: limits.MinIntervalSeconds,
	}, nil
}

func (s *Service) orgLimits(ctx context.Context, orgID uuid.UUID) (plan.Limits, plan.Plan, error) {
	p, err := s.plans.Plan(ctx, orgID)
	if err != nil {
		return plan.Limits{}, "", err
	}
	return plan.LimitsFor(p), p, nil
}

// Create adds a monitor and schedules a control-plane sync so probing begins.
func (s *Service) Create(ctx context.Context, actor Actor, in CreateInput) (*Monitor, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit creating monitors")
	}
	if err := s.assertProject(ctx, actor.OrgID, in.ProjectID); err != nil {
		return nil, err
	}

	// Enforce the org's plan limits: monitor count and minimum interval.
	limits, _, err := s.orgLimits(ctx, actor.OrgID)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountByOrg(ctx, actor.OrgID)
	if err != nil {
		return nil, err
	}
	if count >= limits.MaxMonitors {
		return nil, apperror.QuotaExceeded(fmt.Sprintf(
			"your plan allows up to %d monitors (you already have %d). Upgrade your plan to add more.",
			limits.MaxMonitors, count))
	}

	interval := orDefault(in.IntervalSeconds, defaultInterval)
	timeout := orDefault(in.TimeoutSeconds, defaultTimeout)
	if err := validateSchedule(interval, timeout); err != nil {
		return nil, err
	}
	if err := enforceIntervalFloor(interval, limits.MinIntervalSeconds); err != nil {
		return nil, err
	}

	target, settings, err := normalizeAndValidate(in.Type, in.Target, in.Settings)
	if err != nil {
		return nil, err
	}

	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	// Safe by default: unpublished unless explicitly asked for.
	public := false
	if in.Public != nil {
		public = *in.Public
	}
	now := s.now().UTC()
	m := &Monitor{
		ID:              uuid.New(),
		OrgID:           actor.OrgID,
		ProjectID:       in.ProjectID,
		Name:            strings.TrimSpace(in.Name),
		Type:            in.Type,
		Target:          target,
		Enabled:         enabled,
		Public:          public,
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
		Settings:        settings,
		LastStatus:      StatusUnknown,
		CreatedBy:       &actor.UserID,
		UpdatedBy:       &actor.UserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	s.sync(ctx)
	s.record(ctx, actor, audit.ActionMonitorCreated, m.ID, map[string]any{
		"name": m.Name, "type": string(m.Type), "target": m.Target,
	})
	return m, nil
}

// Get returns a monitor scoped to the actor's org.
func (s *Service) Get(ctx context.Context, actor Actor, id uuid.UUID) (*Monitor, error) {
	return s.repo.GetByID(ctx, actor.OrgID, id)
}

// List returns a page of monitors for the actor's org.
func (s *Service) List(ctx context.Context, actor Actor, f ListFilter) ([]Monitor, int, error) {
	normalizeFilter(&f)
	return s.repo.List(ctx, actor.OrgID, f)
}

// Update applies a partial update and re-syncs the control plane.
func (s *Service) Update(ctx context.Context, actor Actor, id uuid.UUID, in UpdateInput) (*Monitor, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit updating monitors")
	}
	m, err := s.repo.GetByID(ctx, actor.OrgID, id)
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
		m.Name = name
		changed["name"] = name
	}
	if in.IntervalSeconds != nil {
		m.IntervalSeconds = *in.IntervalSeconds
		changed["interval_seconds"] = m.IntervalSeconds
	}
	if in.TimeoutSeconds != nil {
		m.TimeoutSeconds = *in.TimeoutSeconds
		changed["timeout_seconds"] = m.TimeoutSeconds
	}
	if err := validateSchedule(m.IntervalSeconds, m.TimeoutSeconds); err != nil {
		return nil, err
	}
	// Enforce the plan's interval floor only when the interval is being changed,
	// so unrelated edits to a grandfathered monitor are not blocked.
	if in.IntervalSeconds != nil {
		limits, _, err := s.orgLimits(ctx, actor.OrgID)
		if err != nil {
			return nil, err
		}
		if err := enforceIntervalFloor(m.IntervalSeconds, limits.MinIntervalSeconds); err != nil {
			return nil, err
		}
	}
	if in.Enabled != nil {
		m.Enabled = *in.Enabled
		changed["enabled"] = m.Enabled
	}
	// Publishing (or unpublishing) a monitor changes what anonymous visitors can
	// see, so it lands in the audit log like any other security-relevant change.
	if in.Public != nil {
		m.Public = *in.Public
		changed["public"] = m.Public
	}
	// Target/settings are re-validated together because settings defaults depend
	// on the (possibly new) target.
	if in.Target != nil || in.Settings != nil {
		target := m.Target
		if in.Target != nil {
			target = *in.Target
		}
		settings := m.Settings
		if in.Settings != nil {
			settings = *in.Settings
		}
		nt, ns, err := normalizeAndValidate(m.Type, target, settings)
		if err != nil {
			return nil, err
		}
		m.Target, m.Settings = nt, ns
		changed["target"] = nt
	}

	m.UpdatedBy = &actor.UserID
	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	s.sync(ctx)
	s.record(ctx, actor, audit.ActionMonitorUpdated, m.ID, changed)
	return m, nil
}

// SetEnabled enables or pauses a monitor and re-syncs.
func (s *Service) SetEnabled(ctx context.Context, actor Actor, id uuid.UUID, enabled bool) (*Monitor, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit changing monitors")
	}
	if _, err := s.repo.GetByID(ctx, actor.OrgID, id); err != nil {
		return nil, err
	}
	if err := s.repo.SetEnabled(ctx, actor.OrgID, id, enabled, actor.UserID); err != nil {
		return nil, err
	}
	s.sync(ctx)
	action := audit.ActionMonitorPaused
	if enabled {
		action = audit.ActionMonitorEnabled
	}
	s.record(ctx, actor, action, id, nil)
	return s.repo.GetByID(ctx, actor.OrgID, id)
}

// Delete soft-deletes a monitor and re-syncs so probing stops.
func (s *Service) Delete(ctx context.Context, actor Actor, id uuid.UUID) error {
	if !actor.Role.CanWrite() {
		return apperror.Forbidden("your role does not permit deleting monitors")
	}
	if _, err := s.repo.GetByID(ctx, actor.OrgID, id); err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, actor.OrgID, id, actor.UserID); err != nil {
		return err
	}
	s.sync(ctx)
	s.record(ctx, actor, audit.ActionMonitorDeleted, id, nil)
	return nil
}

// ---- internals ----

func (s *Service) assertProject(ctx context.Context, orgID, projectID uuid.UUID) error {
	ok, err := s.repo.ProjectExists(ctx, orgID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.Validation("project not found",
			apperror.FieldError{Field: "project_id", Message: "must reference an existing project"})
	}
	return nil
}

// sync requests a control-plane reconciliation. Failures are logged, not
// returned: the change is already persisted and a periodic full resync (plus
// queue retries) will converge the control plane. This keeps the API responsive
// and resilient to a temporarily-unavailable Prometheus.
func (s *Service) sync(ctx context.Context) {
	if s.syncer == nil {
		return
	}
	if err := s.syncer.Sync(ctx); err != nil {
		logger.FromContext(ctx).Warn("control-plane sync request failed; will reconcile later",
			slog.String("error", err.Error()))
	}
}

func (s *Service) record(ctx context.Context, actor Actor, action audit.Action, resourceID uuid.UUID, md map[string]any) {
	org := actor.OrgID
	uid := actor.UserID
	_ = s.auditlog.Record(ctx, audit.Entry{
		OrgID:        &org,
		UserID:       &uid,
		Action:       action,
		ResourceType: "monitor",
		ResourceID:   resourceID.String(),
		Metadata:     md,
	})
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// enforceIntervalFloor rejects intervals faster than the plan allows.
func enforceIntervalFloor(interval, minInterval int) error {
	if interval < minInterval {
		return apperror.QuotaExceeded(fmt.Sprintf(
			"your plan's minimum check interval is %ds; choose a slower interval or upgrade.", minInterval))
	}
	return nil
}

func validateSchedule(interval, timeout int) error {
	if interval < minInterval || interval > maxInterval {
		return apperror.Validation("interval out of range",
			apperror.FieldError{Field: "interval_seconds", Message: "must be between 10 and 86400"})
	}
	if timeout < minTimeout || timeout > maxTimeout {
		return apperror.Validation("timeout out of range",
			apperror.FieldError{Field: "timeout_seconds", Message: "must be between 1 and 300"})
	}
	if timeout > interval {
		return apperror.Validation("timeout must not exceed interval",
			apperror.FieldError{Field: "timeout_seconds", Message: "must be less than or equal to interval_seconds"})
	}
	return nil
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
