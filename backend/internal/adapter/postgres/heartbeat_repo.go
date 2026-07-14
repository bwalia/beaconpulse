package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/heartbeat"
	"beacon/internal/platform/apperror"
)

// HeartbeatRepository implements heartbeat.Repository — the single-statement,
// unauthenticated ping-ingest write. Kept apart from MonitorRepository so the
// unauth path has its own small, obvious surface.
type HeartbeatRepository struct {
	pool *pgxpool.Pool
}

// NewHeartbeatRepository builds a HeartbeatRepository.
func NewHeartbeatRepository(pool *pgxpool.Pool) *HeartbeatRepository {
	return &HeartbeatRepository{pool: pool}
}

var _ heartbeat.Repository = (*HeartbeatRepository)(nil)

// RecordPing stamps last_ping_at on the heartbeat owning this token.
//
// One statement, resolved through ux_monitors_ping_token (unique, partial), so it
// is O(1) regardless of how many monitors exist. A paused heartbeat still records
// the ping (harmless — the control plane emits no rule for a disabled monitor, so
// it cannot alert); only an unknown or deleted token fails to match.
func (r *HeartbeatRepository) RecordPing(ctx context.Context, token string, at time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE monitors
		    SET last_ping_at = $2
		  WHERE ping_token = $1
		    AND type = 'heartbeat'
		    AND deleted_at IS NULL`,
		token, at)
	if err != nil {
		return false, apperror.Internal(fmt.Errorf("record heartbeat ping: %w", err))
	}
	return tag.RowsAffected() > 0, nil
}
