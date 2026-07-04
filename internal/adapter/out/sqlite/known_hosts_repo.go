package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"momo-shell/internal/core/port/out"
)

// KnownHostsRepo implements out.KnownHostsRepository on top of the
// known_hosts table.
type KnownHostsRepo struct {
	db *sql.DB
}

var _ out.KnownHostsRepository = (*KnownHostsRepo)(nil)

func NewKnownHostsRepo(db *sql.DB) *KnownHostsRepo {
	return &KnownHostsRepo{db: db}
}

func (r *KnownHostsRepo) Get(address string, port int, algo string) (string, bool, error) {
	var fingerprint string
	err := r.db.QueryRow(
		`SELECT fingerprint FROM known_hosts WHERE address = ? AND port = ? AND algo = ?`,
		address, port, algo,
	).Scan(&fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("sqlite: get known host: %w", err)
	}
	return fingerprint, true, nil
}

func (r *KnownHostsRepo) Put(address string, port int, algo string, fingerprint string) error {
	_, err := r.db.Exec(`
		INSERT INTO known_hosts (address, port, algo, fingerprint)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(address, port, algo) DO UPDATE SET fingerprint = excluded.fingerprint
	`, address, port, algo, fingerprint)
	if err != nil {
		return fmt.Errorf("sqlite: put known host: %w", err)
	}
	return nil
}

func (r *KnownHostsRepo) Delete(address string, port int, algo string) error {
	_, err := r.db.Exec(`DELETE FROM known_hosts WHERE address = ? AND port = ? AND algo = ?`, address, port, algo)
	if err != nil {
		return fmt.Errorf("sqlite: delete known host: %w", err)
	}
	return nil
}
