package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/apikey"
	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
)

// APIKeyRepository persists API keys.
type APIKeyRepository struct{ pool *pgxpool.Pool }

func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

var _ apikey.Repository = (*APIKeyRepository)(nil)

func (r *APIKeyRepository) Create(ctx context.Context, k *apikey.Key, hash string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO api_keys (id, org_id, created_by, name, key_hash, key_prefix, role, expires_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		k.ID, k.OrgID, nullUUID(k.CreatedBy), k.Name, hash, k.Prefix, string(k.Role), k.ExpiresAt, k.CreatedAt)
	if err != nil {
		return apperror.Internal(fmt.Errorf("create api key: %w", err))
	}
	return nil
}

// ByHash resolves a presented key in one indexed lookup on the unique hash.
//
// Deliberately returns the key whatever its state — revoked, expired — and leaves that
// judgement to the domain. Filtering here would put the rule in two places, and the
// half that lives in SQL is the half nobody tests.
func (r *APIKeyRepository) ByHash(ctx context.Context, hash string) (*apikey.Key, error) {
	var (
		k    apikey.Key
		role string
	)
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, key_prefix, role, created_at, expires_at, revoked_at, last_used_at
		   FROM api_keys WHERE key_hash = $1`, hash).
		Scan(&k.ID, &k.OrgID, &k.Name, &k.Prefix, &role, &k.CreatedAt, &k.ExpiresAt, &k.RevokedAt, &k.LastUsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // unknown key; the domain reports one opaque failure
		}
		return nil, apperror.Internal(fmt.Errorf("lookup api key: %w", err))
	}
	k.Role = auth.Role(role)
	return &k, nil
}

func (r *APIKeyRepository) List(ctx context.Context, orgID uuid.UUID) ([]apikey.Key, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, key_prefix, role, created_at, expires_at, revoked_at, last_used_at
		   FROM api_keys WHERE org_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("list api keys: %w", err))
	}
	defer rows.Close()

	out := []apikey.Key{}
	for rows.Next() {
		var (
			k    apikey.Key
			role string
		)
		if err := rows.Scan(&k.ID, &k.OrgID, &k.Name, &k.Prefix, &role,
			&k.CreatedAt, &k.ExpiresAt, &k.RevokedAt, &k.LastUsedAt); err != nil {
			return nil, apperror.Internal(fmt.Errorf("scan api key: %w", err))
		}
		k.Role = auth.Role(role)
		out = append(out, k)
	}
	return out, rows.Err()
}

// Revoke tombstones a key. Idempotent: revoking twice is not an error, because the
// caller's intent — "this key must not work" — is satisfied either way.
func (r *APIKeyRepository) Revoke(ctx context.Context, orgID, id uuid.UUID, at time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = COALESCE(revoked_at, $3)
		  WHERE id = $1 AND org_id = $2`, id, orgID, at)
	if err != nil {
		return apperror.Internal(fmt.Errorf("revoke api key: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("API key not found")
	}
	return nil
}

// lastUsedResolution is how stale "last used" is allowed to be.
//
// Recording it exactly would mean a write on every authenticated request: a CI job
// looping over 200 domains would take Beacon's most-contended row and update it 200
// times, serialising the whole batch behind one lock for a timestamp nobody reads to
// the minute. The question this column answers is "is this key still in use, and can I
// revoke it?", and an hour's resolution answers that completely.
const lastUsedResolution = time.Hour

// TouchLastUsed records a key's use, at most once per lastUsedResolution.
//
// The threshold is in the WHERE clause rather than in Go: two concurrent requests would
// both read a stale value and both decide to write, so the decision has to be made
// where the row is locked.
func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET last_used_at = $2
		  WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < $2 - $3::interval)`,
		id, at, lastUsedResolution.String())
	if err != nil {
		return apperror.Internal(fmt.Errorf("touch api key: %w", err))
	}
	return nil
}

func nullUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
