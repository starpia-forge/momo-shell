package sqlite

import (
	"database/sql"
	"fmt"
)

// schemaVersion tracks applied migrations via PRAGMA user_version so Open
// is idempotent across app restarts.
const schemaVersion = 1

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("sqlite: read schema version: %w", err)
	}

	if version < 1 {
		if err := migrateV1(db); err != nil {
			return err
		}
	}

	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("sqlite: write schema version: %w", err)
	}
	return nil
}

// migrateV1 creates the initial schema (01-architecture.md §6), with
// hosts.last_connected_at added for the sidebar's "recent" sort. Statements
// run individually rather than as one batched Exec since not every driver
// supports multi-statement execution.
func migrateV1(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS hosts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 22,
			labels TEXT NOT NULL DEFAULT '[]',
			username TEXT NOT NULL,
			auth_type TEXT NOT NULL,
			key_path TEXT,
			source TEXT NOT NULL DEFAULT 'local',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_connected_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS command_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			host_id TEXT,
			command TEXT NOT NULL,
			executed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_history_recent ON command_history(executed_at DESC)`,
		`CREATE TABLE IF NOT EXISTS known_hosts (
			address TEXT NOT NULL,
			port INTEGER NOT NULL,
			algo TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			PRIMARY KEY (address, port, algo)
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite: migrate v1: %w", err)
		}
	}
	return nil
}
