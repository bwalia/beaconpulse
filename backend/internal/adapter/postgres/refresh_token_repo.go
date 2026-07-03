package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
)

// RefreshTokenRepository implements auth.RefreshTokenRepository.
type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

// NewRefreshTokenRepository builds a RefreshTokenRepository.
func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

var _ auth.RefreshTokenRepository = (*RefreshTokenRepository)(nil)

// Create persists a hashed refresh token.
func (r *RefreshTokenRepository) Create(ctx context.Context, t *auth.RefreshToken) error {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, user_agent, ip, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.UserAgent, t.IP, t.CreatedAt,
	); err != nil {
		return apperror.Internal(fmt.Errorf("create refresh token: %w", err))
	}
	return nil
}

// GetByHash looks up a refresh token by its HMAC hash.
func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*auth.RefreshToken, error) {
	var t auth.RefreshToken
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked_at, user_agent, ip, created_at
		 FROM refresh_tokens WHERE token_hash = $1`, hash,
	).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.UserAgent, &t.IP, &t.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, apperror.NotFound("refresh token not found")
		}
		return nil, apperror.Internal(fmt.Errorf("get refresh token: %w", err))
	}
	return &t, nil
}

// Revoke marks a token revoked. Idempotent — revoking twice is a no-op.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id,
	); err != nil {
		return apperror.Internal(fmt.Errorf("revoke refresh token: %w", err))
	}
	return nil
}

// RevokeAllForUser revokes every active token for a user (e.g. on password
// change or forced logout).
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID,
	); err != nil {
		return apperror.Internal(fmt.Errorf("revoke all refresh tokens: %w", err))
	}
	return nil
}

// DeleteExpired removes expired tokens and returns the number deleted. Called
// periodically by the cleanup worker.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < now()`)
	if err != nil {
		return 0, apperror.Internal(fmt.Errorf("delete expired tokens: %w", err))
	}
	return tag.RowsAffected(), nil
}
