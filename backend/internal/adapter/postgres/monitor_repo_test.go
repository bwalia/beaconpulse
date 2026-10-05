package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/monitor"
	"beacon/internal/platform/crypto"
	"beacon/internal/platform/database"
	"beacon/migrations"
)

func testCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.NewCipher(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestHeaderValuesAreEncryptedAtRest: what reaches the database holds no
// plaintext header value, and reading it back restores the original.
func TestHeaderValuesAreEncryptedAtRest(t *testing.T) {
	r := &MonitorRepository{cipher: testCipher(t)}
	in := monitor.Settings{Headers: map[string]string{"Authorization": "Bearer s3cret"}}

	raw, err := r.sealedConfig(in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("s3cret")) {
		t.Fatalf("stored config contains the plaintext value: %s", raw)
	}
	if in.Headers["Authorization"] != "Bearer s3cret" {
		t.Error("sealing mutated the caller's settings")
	}

	var out monitor.Settings
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	r.openHeaders(out.Headers)
	if got := out.Headers["Authorization"]; got != "Bearer s3cret" {
		t.Errorf("read back %q", got)
	}
}

// TestSealLegacyHeaders rewrites plaintext rows from before encryption at rest.
// It needs a scratch database: BEACON_TEST_DSN=postgres://… go test -run Legacy
func TestSealLegacyHeaders(t *testing.T) {
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

	org, proj, id := uuid.New(), uuid.New(), uuid.New()
	slug := "t-" + id.String()[:8]
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO organizations (id, name, slug) VALUES ($1, 't', $2)`, []any{org, slug}},
		{`INSERT INTO projects (id, org_id, name, slug) VALUES ($1, $2, 't', $3)`, []any{proj, org, slug}},
		{`INSERT INTO monitors (id, org_id, project_id, name, type, target, config)
		  VALUES ($1, $2, $3, 't', 'https', 'https://example.com', '{"headers":{"Authorization":"Bearer legacy"}}')`, []any{id, org, proj}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}

	r := NewMonitorRepository(pool, testCipher(t))
	if n, err := r.SealLegacyHeaders(ctx); err != nil || n != 1 {
		t.Fatalf("first run sealed %d (err %v), want 1", n, err)
	}
	var raw string
	if err := pool.QueryRow(ctx, `SELECT config::text FROM monitors WHERE id=$1`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains([]byte(raw), []byte("legacy")) {
		t.Fatalf("row still holds plaintext: %s", raw)
	}
	m, err := r.GetByID(ctx, org, id)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Settings.Headers["Authorization"]; got != "Bearer legacy" {
		t.Errorf("read back %q", got)
	}
	if n, err := r.SealLegacyHeaders(ctx); err != nil || n != 0 {
		t.Errorf("second run sealed %d (err %v), want 0", n, err)
	}
}
