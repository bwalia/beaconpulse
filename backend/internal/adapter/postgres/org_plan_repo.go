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
		p          string
		status     *string
		credit     int64
		ownerEmail string
	)
	err := r.pool.QueryRow(ctx,
		`SELECT o.plan, o.subscription_status, o.credit_seconds, COALESCE(ow.email, '')
		   FROM organizations o
		   LEFT JOIN LATERAL (
		       SELECT email FROM users
		        WHERE org_id = o.id AND role = 'owner' AND deleted_at IS NULL
		        ORDER BY created_at LIMIT 1
		   ) ow ON true
		  WHERE o.id = $1 AND o.deleted_at IS NULL`, orgID,
	).Scan(&p, &status, &credit, &ownerEmail)
	if err != nil {
		if isNoRows(err) {
			return "", apperror.NotFound("organization not found")
		}
		return "", apperror.Internal(fmt.Errorf("read org plan: %w", err))
	}
	subActive := status != nil && (*status == "active" || *status == "trialing")
	return plan.Resolve(plan.Plan(p), subActive, credit, ownerEmail), nil
}
