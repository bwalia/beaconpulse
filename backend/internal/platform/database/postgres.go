// Package database owns the Postgres connection pool. The rest of the
// application depends on the *pgxpool.Pool through repository implementations;
// no package constructs its own connections. Pool sizing and lifetimes are
// driven by configuration so the same code scales from a laptop to production.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/config"
)

// Connect opens and verifies a Postgres connection pool. It blocks until a ping
// succeeds or the context is cancelled, so callers can fail fast at startup.
func Connect(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("database: parse dsn: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return pool, nil
}
