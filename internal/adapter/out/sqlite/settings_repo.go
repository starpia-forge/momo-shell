package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"momo-shell/internal/core/port/out"
)

// SettingsRepo implements out.SettingsRepository on top of the
// app_settings table.
type SettingsRepo struct {
	db *sql.DB
}

var _ out.SettingsRepository = (*SettingsRepo)(nil)

func NewSettingsRepo(db *sql.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

func (r *SettingsRepo) Get(key string) (string, bool, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("sqlite: get app setting %s: %w", key, err)
	}
	return value, true, nil
}

func (r *SettingsRepo) Set(key, value string) error {
	_, err := r.db.Exec(`
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("sqlite: set app setting %s: %w", key, err)
	}
	return nil
}

func (r *SettingsRepo) All() (map[string]string, error) {
	rows, err := r.db.Query(`SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list app settings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("sqlite: scan app setting: %w", err)
		}
		result[key] = value
	}
	return result, rows.Err()
}
