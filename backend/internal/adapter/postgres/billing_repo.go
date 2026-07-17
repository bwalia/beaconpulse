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

// ---- AI diagnosis metering ----
//
// These live on the billing repository because credit_seconds does. One owner for the
// money means the diagnosis charge and the per-minute meter cannot drift into two
// different ideas of what a balance is.

// ChargeCredit debits costSeconds if — and only if — the balance covers it, in a
// single statement.
//
// The WHERE clause is the whole safety property. Reading the balance and then writing
// it back would let two simultaneous clicks both see enough credit and both spend it,
// so an org with one diagnosis left could take several. Here the second UPDATE simply
// matches no row.
func (r *BillingRepository) ChargeCredit(ctx context.Context, orgID uuid.UUID, costSeconds int64) (bool, error) {
	if costSeconds <= 0 {
		return true, nil
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE organizations
		    SET credit_seconds = credit_seconds - $2
		  WHERE id = $1 AND deleted_at IS NULL AND credit_seconds >= $2`,
		orgID, costSeconds)
	if err != nil {
		return false, apperror.Internal(fmt.Errorf("charge diagnosis credit: %w", err))
	}
	return tag.RowsAffected() == 1, nil
}

// RefundCredit returns a charge for a diagnosis that was never delivered.
func (r *BillingRepository) RefundCredit(ctx context.Context, orgID uuid.UUID, costSeconds int64) error {
	if costSeconds <= 0 {
		return nil
	}
	if _, err := r.pool.Exec(ctx,
		`UPDATE organizations SET credit_seconds = credit_seconds + $2
		  WHERE id = $1 AND deleted_at IS NULL`, orgID, costSeconds); err != nil {
		return apperror.Internal(fmt.Errorf("refund diagnosis credit: %w", err))
	}
	return nil
}

// CountRunsSince counts delivered diagnoses in the current allowance window.
func (r *BillingRepository) CountRunsSince(ctx context.Context, orgID uuid.UUID, since time.Time) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM diagnose_runs WHERE org_id = $1 AND created_at >= $2`,
		orgID, since).Scan(&n); err != nil {
		return 0, apperror.Internal(fmt.Errorf("count diagnoses: %w", err))
	}
	return n, nil
}

// RecordRun writes a delivered diagnosis to the ledger.
func (r *BillingRepository) RecordRun(ctx context.Context, orgID, monitorID uuid.UUID, costSeconds int64) error {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO diagnose_runs (id, org_id, monitor_id, credit_seconds)
		 VALUES ($1,$2,$3,$4)`,
		uuid.New(), orgID, monitorID, costSeconds); err != nil {
		return apperror.Internal(fmt.Errorf("record diagnosis: %w", err))
	}
	return nil
}

// CreditTotals reports how much credit an org has ever been given and how much it has
// spent, so the billing page can say "you have monitored X hours, Y left" instead of
// only naming a balance.
//
// Consumed is DERIVED (granted − remaining) rather than tracked in its own column:
// every grant is already an immutable row in billing_events, and the balance is
// authoritative, so a separate counter could only ever disagree with them. It also
// means the number is correct for credit spent before this was written.
func (r *BillingRepository) CreditTotals(ctx context.Context, orgID uuid.UUID) (granted, remaining int64, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT COALESCE((SELECT sum(credit_added_seconds) FROM billing_events
		                   WHERE org_id = $1 AND type = 'topup'), 0),
		        COALESCE((SELECT credit_seconds FROM organizations
		                   WHERE id = $1 AND deleted_at IS NULL), 0)`,
		orgID).Scan(&granted, &remaining)
	if err != nil {
		return 0, 0, apperror.Internal(fmt.Errorf("credit totals: %w", err))
	}
	return granted, remaining, nil
}
