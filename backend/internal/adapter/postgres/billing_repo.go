package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/billing"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

// BillingRepository persists an org's billing state and the Stripe webhook
// idempotency ledger (billing_events).
type BillingRepository struct {
	pool *pgxpool.Pool
}

// NewBillingRepository builds a BillingRepository.
func NewBillingRepository(pool *pgxpool.Pool) *BillingRepository {
	return &BillingRepository{pool: pool}
}

var _ billing.Repository = (*BillingRepository)(nil)

// State returns the org's billing snapshot.
func (r *BillingRepository) State(ctx context.Context, orgID uuid.UUID) (billing.State, error) {
	var (
		st        billing.State
		planStr   string
		status    *string
		periodEnd *time.Time
		custID    *string
	)
	err := r.pool.QueryRow(ctx,
		`SELECT plan, subscription_status, subscription_current_period_end, stripe_customer_id, credit_seconds
		   FROM organizations WHERE id = $1 AND deleted_at IS NULL`, orgID).
		Scan(&planStr, &status, &periodEnd, &custID, &st.CreditSeconds)
	if err != nil {
		if isNoRows(err) {
			return billing.State{}, apperror.NotFound("organization not found")
		}
		return billing.State{}, apperror.Internal(fmt.Errorf("read billing state: %w", err))
	}
	st.Plan = plan.Plan(planStr)
	if status != nil {
		st.SubscriptionStatus = *status
	}
	if periodEnd != nil {
		st.PeriodEnd = *periodEnd
	}
	if custID != nil {
		st.StripeCustomerID = *custID
	}
	return st, nil
}

// SetCustomerID stores the org's Stripe customer id.
func (r *BillingRepository) SetCustomerID(ctx context.Context, orgID uuid.UUID, customerID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organizations SET stripe_customer_id = $2 WHERE id = $1 AND deleted_at IS NULL`,
		orgID, customerID)
	if err != nil {
		return apperror.Internal(fmt.Errorf("set stripe customer: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("organization not found")
	}
	return nil
}

// ApplyTopUp records the event and adds credit in one transaction. The unique
// stripe_event_id makes it idempotent: a duplicate webhook inserts nothing and
// credits nothing (returns applied=false).
func (r *BillingRepository) ApplyTopUp(ctx context.Context, orgID uuid.UUID, addSeconds, amountCents int64, eventID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, apperror.Internal(fmt.Errorf("begin top-up: %w", err))
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx,
		`INSERT INTO billing_events (id, stripe_event_id, org_id, type, amount_cents, credit_added_seconds)
		 VALUES ($1,$2,$3,'topup',$4,$5) ON CONFLICT (stripe_event_id) DO NOTHING`,
		uuid.New(), eventID, orgID, amountCents, addSeconds)
	if err != nil {
		return false, apperror.Internal(fmt.Errorf("record top-up event: %w", err))
	}
	if ct.RowsAffected() == 0 {
		return false, tx.Commit(ctx) // already processed
	}
	if _, err := tx.Exec(ctx,
		`UPDATE organizations SET credit_seconds = credit_seconds + $2 WHERE id = $1 AND deleted_at IS NULL`,
		orgID, addSeconds); err != nil {
		return false, apperror.Internal(fmt.Errorf("add credit: %w", err))
	}
	return true, tx.Commit(ctx)
}

// ApplySubscription records the event and syncs the org's subscribed tier/status,
// idempotently on the Stripe event id.
func (r *BillingRepository) ApplySubscription(ctx context.Context, orgID uuid.UUID, p plan.Plan, status string, periodEnd time.Time, eventID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, apperror.Internal(fmt.Errorf("begin subscription: %w", err))
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx,
		`INSERT INTO billing_events (id, stripe_event_id, org_id, type)
		 VALUES ($1,$2,$3,'subscription') ON CONFLICT (stripe_event_id) DO NOTHING`,
		uuid.New(), eventID, orgID)
	if err != nil {
		return false, apperror.Internal(fmt.Errorf("record subscription event: %w", err))
	}
	if ct.RowsAffected() == 0 {
		return false, tx.Commit(ctx)
	}
	// The subscribed tier is stored (starter/pro); whether it grants that tier is
	// gated on subscription_status by the effective-plan computation.
	tier := string(p)
	if !p.Subscribable() {
		tier = string(plan.Free)
	}
	var pe *time.Time
	if !periodEnd.IsZero() {
		pe = &periodEnd
	}
	if _, err := tx.Exec(ctx,
		`UPDATE organizations
		    SET plan = $2, subscription_status = $3, subscription_current_period_end = $4
		  WHERE id = $1 AND deleted_at IS NULL`,
		orgID, tier, status, pe); err != nil {
		return false, apperror.Internal(fmt.Errorf("sync subscription: %w", err))
	}
	return true, tx.Commit(ctx)
}

// DeductCredit burns elapsedSeconds × (enabled monitor count) from every org that
// has a positive balance, flooring at zero — the pay-as-you-go meter.
func (r *BillingRepository) DeductCredit(ctx context.Context, elapsedSeconds int64) error {
	if elapsedSeconds <= 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE organizations o
		    SET credit_seconds = GREATEST(0, o.credit_seconds - $1 * m.cnt)
		   FROM (
		       SELECT org_id, count(*)::bigint AS cnt
		         FROM monitors
		        WHERE enabled AND deleted_at IS NULL
		        GROUP BY org_id
		   ) m
		  WHERE o.id = m.org_id AND o.credit_seconds > 0 AND o.deleted_at IS NULL`,
		elapsedSeconds)
	if err != nil {
		return apperror.Internal(fmt.Errorf("deduct credit: %w", err))
	}
	return nil
}
