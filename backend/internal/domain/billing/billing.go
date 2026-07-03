// Package billing manages an organization's subscription plan. In this build a
// plan change is applied directly (a self-serve switch); a production deployment
// would insert a payment provider (e.g. Stripe Checkout) before ChangePlan and
// drive it from a webhook. The domain is intentionally provider-agnostic: it only
// reads and writes the org's plan and records an audit entry.
package billing

import (
	"context"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

// OrgPlanStore reads and writes an organization's plan.
type OrgPlanStore interface {
	Plan(ctx context.Context, orgID uuid.UUID) (plan.Plan, error)
	SetPlan(ctx context.Context, orgID uuid.UUID, p plan.Plan) error
}

// Actor is the authenticated caller.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}

// Service implements plan viewing and changing.
type Service struct {
	store    OrgPlanStore
	auditlog audit.Recorder
}

// NewService wires the billing service.
func NewService(store OrgPlanStore, auditlog audit.Recorder) *Service {
	return &Service{store: store, auditlog: auditlog}
}

// Catalog returns the purchasable plans.
func (s *Service) Catalog() []plan.Info { return plan.Catalog() }

// Current returns the caller org's current plan.
func (s *Service) Current(ctx context.Context, actor Actor) (plan.Plan, error) {
	return s.store.Plan(ctx, actor.OrgID)
}

// ChangePlan switches the org to a new plan. Restricted to owners/admins (the
// people who would hold billing responsibility).
func (s *Service) ChangePlan(ctx context.Context, actor Actor, newPlan plan.Plan) (plan.Plan, error) {
	if !actor.Role.CanAdminister() {
		return "", apperror.Forbidden("only owners and admins can change the plan")
	}
	if !newPlan.Valid() {
		return "", apperror.Validation("unknown plan",
			apperror.FieldError{Field: "plan", Message: "must be free, starter or pro"})
	}
	if err := s.store.SetPlan(ctx, actor.OrgID, newPlan); err != nil {
		return "", err
	}
	org := actor.OrgID
	uid := actor.UserID
	_ = s.auditlog.Record(ctx, audit.Entry{
		OrgID: &org, UserID: &uid, Action: "billing.plan_changed",
		ResourceType: "organization", ResourceID: org.String(),
		Metadata: map[string]any{"plan": string(newPlan)},
	})
	return newPlan, nil
}
