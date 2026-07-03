package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/billing"
	"beacon/internal/domain/monitor"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

// OrgPlanRepository reads an organization's subscription plan for quota
// enforcement.
type OrgPlanRepository struct {
	pool *pgxpool.Pool
}

// NewOrgPlanRepository builds an OrgPlanRepository.
func NewOrgPlanRepository(pool *pgxpool.Pool) *OrgPlanRepository {
	return &OrgPlanRepository{pool: pool}
}

var (
	_ monitor.OrgPlanReader = (*OrgPlanRepository)(nil)
	_ billing.OrgPlanStore  = (*OrgPlanRepository)(nil)
)

// Plan returns the org's plan. A missing org surfaces as not-found; an
// unexpected/empty value falls back to Free via plan.LimitsFor downstream.
func (r *OrgPlanRepository) Plan(ctx context.Context, orgID uuid.UUID) (plan.Plan, error) {
	var p string
	err := r.pool.QueryRow(ctx,
		`SELECT plan FROM organizations WHERE id = $1 AND deleted_at IS NULL`, orgID,
	).Scan(&p)
	if err != nil {
		if isNoRows(err) {
			return "", apperror.NotFound("organization not found")
		}
		return "", apperror.Internal(fmt.Errorf("read org plan: %w", err))
	}
	return plan.Plan(p), nil
}

// SetPlan updates the org's plan.
func (r *OrgPlanRepository) SetPlan(ctx context.Context, orgID uuid.UUID, p plan.Plan) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organizations SET plan = $2 WHERE id = $1 AND deleted_at IS NULL`, orgID, string(p))
	if err != nil {
		return apperror.Internal(fmt.Errorf("set org plan: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("organization not found")
	}
	return nil
}
