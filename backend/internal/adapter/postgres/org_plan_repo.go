package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

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

var _ monitor.OrgPlanReader = (*OrgPlanRepository)(nil)

// Plan returns the org's EFFECTIVE plan — the tier whose limits actually apply
// right now: the subscribed tier while its Stripe subscription is active, else
// pay-as-you-go while credit remains, else Free. Enforcement (monitor create,
// control-plane cap) reads this, so a depleted org automatically falls back to
// Free's limits. A missing org surfaces as not-found.
func (r *OrgPlanRepository) Plan(ctx context.Context, orgID uuid.UUID) (plan.Plan, error) {
	var (
		p      string
		status *string
		credit int64
	)
	err := r.pool.QueryRow(ctx,
		`SELECT plan, subscription_status, credit_seconds
		   FROM organizations WHERE id = $1 AND deleted_at IS NULL`, orgID,
	).Scan(&p, &status, &credit)
	if err != nil {
		if isNoRows(err) {
			return "", apperror.NotFound("organization not found")
		}
		return "", apperror.Internal(fmt.Errorf("read org plan: %w", err))
	}
	subActive := status != nil && (*status == "active" || *status == "trialing")
	return plan.Effective(plan.Plan(p), subActive, credit), nil
}
