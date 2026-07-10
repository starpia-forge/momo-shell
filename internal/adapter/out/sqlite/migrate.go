package sqlite

import (
	"database/sql"
	"fmt"
)

// schemaVersion tracks applied migrations via PRAGMA user_version so Open
// is idempotent across app restarts.
const schemaVersion = 6

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
	if version < 2 {
		if err := migrateV2(db); err != nil {
			return err
		}
	}
	if version < 3 {
		if err := migrateV3(db); err != nil {
			return err
		}
	}
	if version < 4 {
		if err := migrateV4(db); err != nil {
			return err
		}
	}
	if version < 5 {
		if err := migrateV5(db); err != nil {
			return err
		}
	}
	if version < 6 {
		if err := migrateV6(db); err != nil {
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

// migrateV2 adds the tables backing LAN host sharing
// (docs/plan/06-phase5-host-sharing.md): share_peers caches peers this
// instance has paired with or discovered (consumer role, from M2 on),
// share_clients tracks peers this instance has issued a bearer token to
// (provider role), and share_settings is a small key-value store for this
// instance's identity and its shared-host selection.
func migrateV2(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS share_peers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			port INTEGER NOT NULL,
			cert_fingerprint TEXT NOT NULL,
			paired_at INTEGER NOT NULL,
			last_sync_at INTEGER,
			hosts_json TEXT NOT NULL DEFAULT '[]'
		)`,
		`CREATE TABLE IF NOT EXISTS share_clients (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			paired_at INTEGER NOT NULL,
			last_seen_at INTEGER
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_share_clients_token_hash ON share_clients(token_hash)`,
		`CREATE TABLE IF NOT EXISTS share_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite: migrate v2: %w", err)
		}
	}
	return nil
}

// migrateV3 adds app_settings, a flat key-value store for app-wide
// settings (docs/plan/09-settings.md) -- appearance theme and terminal
// defaults, distinct from share_settings which is share-domain identity.
func migrateV3(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite: migrate v3: %w", err)
		}
	}
	return nil
}

// migrateV4 adds mcp_clients, tracking MCP clients this instance has
// paired with (doc 20 D2 -- token = authentication only, no scope column;
// authorization is delegation's job). revoked is a soft-delete flag rather
// than row removal, distinct from every other repo in this package
// (host/history/share all hard-delete): audit history (E3) needs to
// resolve a clientID to a name even after revocation.
func migrateV4(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS mcp_clients (
			client_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			paired_at INTEGER NOT NULL,
			last_seen_at INTEGER,
			revoked INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_clients_token_hash ON mcp_clients(token_hash)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite: migrate v4: %w", err)
		}
	}
	return nil
}

// migrateV5 adds mcp_audit, the AI-control decision audit trail (doc 20 E3):
// every connect/command/control decision aicontrol makes, with its resolve
// verdict, approver, and timestamp. id is the replay order (US-5) rather
// than ts, since ts has only second granularity and a connect immediately
// followed by a command can otherwise tie. output_ref (doc 17 §10's
// encrypted, retention-bounded original-output capture) is deliberately
// absent -- it is a separate sub-system (E3-b) that lands as an additive
// column in a later migration.
func migrateV5(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS mcp_audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL,
			client_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			target TEXT NOT NULL DEFAULT '',
			original_cmd TEXT NOT NULL DEFAULT '',
			guarded_cmd TEXT NOT NULL DEFAULT '',
			resolve_json TEXT NOT NULL DEFAULT '',
			approver TEXT NOT NULL DEFAULT '',
			decision TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_session ON mcp_audit(session_id, id)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite: migrate v5: %w", err)
		}
	}
	return nil
}

// migrateV6 adds mcp_audit.output_ref (doc 17 §10, E3-b): the encrypted
// original terminal output produced during a command's lifecycle, captured
// by the capture tap and sealed by audit.Service's AES-256-GCM cipher. NULL
// means no capture is attached -- every connect/control event, and any
// command event whose capture hasn't been sealed yet or has aged out of
// audit.Service.PurgeOutputsBefore's retention window. Deliberately kept out
// of auditColumns/scanAuditEvent so List/Query can never surface it -- it is
// reached only via AuditRepo.LoadOutput(id), never through the replay path
// the AI-facing MCP surface uses.
//
// Unlike every migrateVN before it, this is a bare ALTER TABLE ADD COLUMN,
// not CREATE TABLE IF NOT EXISTS -- so it isn't naturally idempotent the
// way the others are. A db whose schema is already fully current but whose
// user_version was reset to an earlier value (exactly what the other
// migration tests in this file do to simulate an old db, without touching
// mcp_audit) would otherwise hit "duplicate column name" on replay. The
// column-existence check below makes it idempotent to match its siblings.
func migrateV6(db *sql.DB) error {
	has, err := columnExists(db, "mcp_audit", "output_ref")
	if err != nil {
		return fmt.Errorf("sqlite: migrate v6: %w", err)
	}
	if has {
		return nil
	}

	stmts := []string{
		`ALTER TABLE mcp_audit ADD COLUMN output_ref BLOB`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite: migrate v6: %w", err)
		}
	}
	return nil
}

// columnExists reports whether table has a column named column, via
// PRAGMA table_info -- SQLite has no "ADD COLUMN IF NOT EXISTS".
func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
