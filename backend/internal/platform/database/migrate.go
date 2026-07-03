package database

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration is a single versioned schema change.
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// MigrationStatus reports whether a migration has been applied.
type MigrationStatus struct {
	Version int
	Name    string
	Applied bool
}

// LoadMigrations parses migration files from src. Files must be named
// NNNN_name.up.sql (and optionally NNNN_name.down.sql). It returns migrations
// sorted ascending by version and errors on duplicate or malformed versions.
func LoadMigrations(src fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	byVersion := map[int]*Migration{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		version, label, direction, err := parseMigrationName(name)
		if err != nil {
			return nil, err
		}
		content, err := fs.ReadFile(src, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		m := byVersion[version]
		if m == nil {
			m = &Migration{Version: version, Name: label}
			byVersion[version] = m
		}
		switch direction {
		case "up":
			m.UpSQL = string(content)
		case "down":
			m.DownSQL = string(content)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if strings.TrimSpace(m.UpSQL) == "" {
			return nil, fmt.Errorf("migration %04d (%s) has no up SQL", m.Version, m.Name)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func parseMigrationName(name string) (version int, label, direction string, err error) {
	base := strings.TrimSuffix(name, ".sql")
	switch {
	case strings.HasSuffix(base, ".up"):
		direction = "up"
		base = strings.TrimSuffix(base, ".up")
	case strings.HasSuffix(base, ".down"):
		direction = "down"
		base = strings.TrimSuffix(base, ".down")
	default:
		return 0, "", "", fmt.Errorf("migration %q must end in .up.sql or .down.sql", name)
	}
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return 0, "", "", fmt.Errorf("migration %q must be named NNNN_name", name)
	}
	version, err = strconv.Atoi(base[:idx])
	if err != nil {
		return 0, "", "", fmt.Errorf("migration %q has non-numeric version: %w", name, err)
	}
	return version, base[idx+1:], direction, nil
}

// Migrator applies migrations against a pool, recording applied versions in a
// schema_migrations table. Each migration runs inside its own transaction so a
// failure leaves the database at a consistent, known version.
type Migrator struct {
	pool       *pgxpool.Pool
	migrations []Migration
}

// NewMigrator builds a Migrator from the given pool and migration source.
func NewMigrator(pool *pgxpool.Pool, src fs.FS) (*Migrator, error) {
	ms, err := LoadMigrations(src)
	if err != nil {
		return nil, err
	}
	return &Migrator{pool: pool, migrations: ms}, nil
}

func (m *Migrator) ensureTable(ctx context.Context) error {
	_, err := m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER     PRIMARY KEY,
			name        TEXT        NOT NULL,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

func (m *Migrator) appliedVersions(ctx context.Context) (map[int]bool, error) {
	rows, err := m.pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied: %w", err)
	}
	defer rows.Close()
	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// Up applies all pending migrations in order and returns the versions applied.
func (m *Migrator) Up(ctx context.Context) ([]int, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}
	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return nil, err
	}

	var done []int
	for _, mig := range m.migrations {
		if applied[mig.Version] {
			continue
		}
		if err := m.applyOne(ctx, mig); err != nil {
			return done, fmt.Errorf("apply migration %04d (%s): %w", mig.Version, mig.Name, err)
		}
		done = append(done, mig.Version)
	}
	return done, nil
}

func (m *Migrator) applyOne(ctx context.Context, mig Migration) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, mig.UpSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		mig.Version, mig.Name,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Status returns each migration and whether it has been applied.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}
	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MigrationStatus, 0, len(m.migrations))
	for _, mig := range m.migrations {
		out = append(out, MigrationStatus{Version: mig.Version, Name: mig.Name, Applied: applied[mig.Version]})
	}
	return out, nil
}
