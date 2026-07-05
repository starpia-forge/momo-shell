package sqlite

import (
	"path/filepath"
	"testing"
)

// TestMigrate_V1ToV2AddsShareTables simulates an existing v1 database
// (schema_version=1, no share_* tables) being opened by the current code,
// asserting the v2 migration runs and every new table is queryable.
func TestMigrate_V1ToV2AddsShareTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	// Open once and roll the schema version back to 1 to simulate a
	// pre-Phase-5 database on disk.
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("reset schema version: %v", err)
	}
	for _, table := range []string{"share_peers", "share_clients", "share_settings"} {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open() (reopen) error = %v", err)
	}
	defer reopened.Close()

	var version int
	if err := reopened.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("schema version = %d, want %d", version, schemaVersion)
	}

	for _, table := range []string{"share_peers", "share_clients", "share_settings"} {
		if _, err := reopened.Exec("SELECT * FROM " + table + " LIMIT 1"); err != nil {
			t.Fatalf("table %s not queryable after migration: %v", table, err)
		}
	}
}
