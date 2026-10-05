package postgres

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/platform/crypto"
	"beacon/internal/platform/database"
	"beacon/migrations"
)

// TestPlatformSecretIsCreatedOnceAndEncrypted: the first call stores the
// generated value encrypted, and every later call (any replica) gets that same
// value back instead of minting a new one. Needs BEACON_TEST_DSN.
func TestPlatformSecretIsCreatedOnceAndEncrypted(t *testing.T) {
	dsn := os.Getenv("BEACON_TEST_DSN")
	if dsn == "" {
		t.Skip("BEACON_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	mig, err := database.NewMigrator(pool, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mig.Up(ctx); err != nil {
		t.Fatal(err)
	}
	c, err := crypto.NewCipher(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	r := NewPlatformSecretRepository(pool, c)

	n := 0
	gen := func() (string, error) { n++; return "secret-" + strings.Repeat("x", n), nil }
	first, err := r.GetOrCreate(ctx, "t", gen)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.GetOrCreate(ctx, "t", gen)
	if err != nil {
		t.Fatal(err)
	}
	if first != "secret-x" || second != first {
		t.Errorf("first=%q second=%q, want both secret-x", first, second)
	}
	var raw string
	if err := pool.QueryRow(ctx, `SELECT value FROM platform_secrets WHERE name='t'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "secret") {
		t.Errorf("stored in plaintext: %q", raw)
	}
}
