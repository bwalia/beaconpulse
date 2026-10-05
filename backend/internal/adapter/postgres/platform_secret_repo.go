package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/platform/crypto"
)

// PlatformSecretRepository keeps secrets the platform generates for itself (the
// VAPID key), encrypted at rest with the deployment's encryption key.
type PlatformSecretRepository struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
}

func NewPlatformSecretRepository(pool *pgxpool.Pool, cipher *crypto.Cipher) *PlatformSecretRepository {
	return &PlatformSecretRepository{pool: pool, cipher: cipher}
}

// GetOrCreate returns the named secret, generating and storing it on first use.
// Safe when several replicas start at once: each offers a candidate, the first
// insert wins, and everyone reads back the winner.
func (r *PlatformSecretRepository) GetOrCreate(ctx context.Context, name string, generate func() (string, error)) (string, error) {
	candidate, err := generate()
	if err != nil {
		return "", err
	}
	sealed, err := r.cipher.EncryptString(candidate)
	if err != nil {
		return "", err
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO platform_secrets (name, value) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, name, sealed); err != nil {
		return "", fmt.Errorf("store platform secret %s: %w", name, err)
	}
	var stored string
	if err := r.pool.QueryRow(ctx, `SELECT value FROM platform_secrets WHERE name = $1`, name).Scan(&stored); err != nil {
		return "", fmt.Errorf("load platform secret %s: %w", name, err)
	}
	return r.cipher.DecryptString(stored)
}
