package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/device"
	"beacon/internal/platform/apperror"
)

// DeviceRepository persists push-notification device tokens. It implements both
// device.Repository (registration) and device.TokenStore (alert fan-out).
type DeviceRepository struct{ pool *pgxpool.Pool }

func NewDeviceRepository(pool *pgxpool.Pool) *DeviceRepository {
	return &DeviceRepository{pool: pool}
}

var (
	_ device.Repository = (*DeviceRepository)(nil)
	_ device.TokenStore = (*DeviceRepository)(nil)
)

// Upsert registers a token or refreshes it in place. Conflicts key on the unique
// token: a device re-registering after a reinstall keeps one row, with its owner,
// org and last_seen updated to the latest registration.
func (r *DeviceRepository) Upsert(ctx context.Context, d *device.Device) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO device_tokens (id, org_id, user_id, platform, token, last_seen_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (token) DO UPDATE SET
		     org_id       = EXCLUDED.org_id,
		     user_id      = EXCLUDED.user_id,
		     platform     = EXCLUDED.platform,
		     last_seen_at = EXCLUDED.last_seen_at`,
		d.ID, d.OrgID, d.UserID, string(d.Platform), d.Token, d.LastSeenAt, d.CreatedAt)
	if err != nil {
		return apperror.Internal(fmt.Errorf("upsert device token: %w", err))
	}
	return nil
}

// DeleteByToken removes one token for an org. Idempotent: deleting an unknown
// token is a no-op, because the caller's intent — "this device no longer
// receives push" — is already satisfied.
func (r *DeviceRepository) DeleteByToken(ctx context.Context, orgID uuid.UUID, token string) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM device_tokens WHERE token = $1 AND org_id = $2`, token, orgID); err != nil {
		return apperror.Internal(fmt.Errorf("delete device token: %w", err))
	}
	return nil
}

// TokensByOrg returns every registered token for an org, for alert fan-out.
func (r *DeviceRepository) TokensByOrg(ctx context.Context, orgID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT token FROM device_tokens WHERE org_id = $1`, orgID)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("list device tokens: %w", err))
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, apperror.Internal(fmt.Errorf("scan device token: %w", err))
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Delete removes a token unconditionally, to prune one APNs has reported dead.
// Not org-scoped: a token APNs rejects as invalid is dead for everyone, and the
// prune runs from the notifier, which holds no actor.
func (r *DeviceRepository) Delete(ctx context.Context, token string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM device_tokens WHERE token = $1`, token); err != nil {
		return apperror.Internal(fmt.Errorf("prune device token: %w", err))
	}
	return nil
}
