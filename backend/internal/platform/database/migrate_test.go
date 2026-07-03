package database

import (
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsOrdersAndPairs(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_add_index.up.sql":   {Data: []byte("CREATE INDEX ...;")},
		"0002_add_index.down.sql": {Data: []byte("DROP INDEX ...;")},
		"0001_init.up.sql":        {Data: []byte("CREATE TABLE ...;")},
		"0001_init.down.sql":      {Data: []byte("DROP TABLE ...;")},
		"README.md":               {Data: []byte("ignored")},
	}
	ms, err := LoadMigrations(fsys)
	if err != nil {
		t.Fatalf("LoadMigrations: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(ms))
	}
	if ms[0].Version != 1 || ms[1].Version != 2 {
		t.Errorf("migrations not sorted ascending: %d, %d", ms[0].Version, ms[1].Version)
	}
	if ms[0].Name != "init" {
		t.Errorf("name = %q, want init", ms[0].Name)
	}
	if ms[0].DownSQL == "" {
		t.Error("expected down SQL to be loaded")
	}
}

func TestLoadMigrationsRejectsMissingUp(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_init.down.sql": {Data: []byte("DROP TABLE ...;")},
	}
	if _, err := LoadMigrations(fsys); err == nil {
		t.Fatal("expected error when up migration is missing")
	}
}

func TestLoadMigrationsRejectsBadName(t *testing.T) {
	fsys := fstest.MapFS{
		"bad-name.up.sql": {Data: []byte("SELECT 1;")},
	}
	if _, err := LoadMigrations(fsys); err == nil {
		t.Fatal("expected error for malformed migration name")
	}
}
