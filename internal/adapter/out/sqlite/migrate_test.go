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

// TestMigrate_V2ToV3AddsAppSettings simulates an existing v2 database
// (schema_version=2, no app_settings table) being opened by the current
// code, asserting the v3 migration runs and the new table is queryable.
func TestMigrate_V2ToV3AddsAppSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatalf("reset schema version: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS app_settings"); err != nil {
		t.Fatalf("drop app_settings: %v", err)
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

	if _, err := reopened.Exec("SELECT * FROM app_settings LIMIT 1"); err != nil {
		t.Fatalf("table app_settings not queryable after migration: %v", err)
	}
}

// TestMigrate_V3ToV4AddsMCPClients simulates an existing v3 database
// (schema_version=3, no mcp_clients table) being opened by the current
// code, asserting the v4 migration runs and the new table is queryable.
func TestMigrate_V3ToV4AddsMCPClients(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 3"); err != nil {
		t.Fatalf("reset schema version: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS mcp_clients"); err != nil {
		t.Fatalf("drop mcp_clients: %v", err)
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

	if _, err := reopened.Exec("SELECT * FROM mcp_clients LIMIT 1"); err != nil {
		t.Fatalf("table mcp_clients not queryable after migration: %v", err)
	}
}
