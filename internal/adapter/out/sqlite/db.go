// Package sqlite implements the HostRepository and KnownHostsRepository
// out-ports on top of a single-file SQLite database (modernc.org/sqlite,
// a pure-Go driver -- this machine has no C toolchain for cgo drivers).
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DefaultPath returns the standard on-disk location for the app database,
// creating its parent directory if necessary.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("sqlite: resolve config dir: %w", err)
	}
	appDir := filepath.Join(dir, "momo-shell")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return "", fmt.Errorf("sqlite: create app dir: %w", err)
	}
	return filepath.Join(appDir, "data.db"), nil
}

// Open opens (creating if needed) the SQLite database at path, sets the
// pragmas appropriate for a single-process desktop app, and applies any
// pending migrations.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}

	// This app is the only writer and never opens more than one connection
	// worth of concurrent work; capping at 1 avoids SQLITE_BUSY from the
	// driver's own connection pool racing itself.
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite: %s: %w", pragma, err)
		}
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
