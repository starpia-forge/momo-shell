package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/out"
)

// ErrHostNotFound is returned by Get/Delete/TouchConnected for an unknown ID.
var ErrHostNotFound = errors.New("sqlite: host not found")

// HostRepo implements out.HostRepository on top of the hosts table.
type HostRepo struct {
	db *sql.DB
}

var _ out.HostRepository = (*HostRepo)(nil)

func NewHostRepo(db *sql.DB) *HostRepo {
	return &HostRepo{db: db}
}

const hostColumns = `id, name, address, port, labels, username, auth_type, key_path, source, created_at, updated_at, last_connected_at`

func (r *HostRepo) List() ([]domain.Host, error) {
	rows, err := r.db.Query(`SELECT ` + hostColumns + ` FROM hosts ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list hosts: %w", err)
	}
	defer rows.Close()

	var hosts []domain.Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan host: %w", err)
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (r *HostRepo) Get(id string) (domain.Host, error) {
	row := r.db.QueryRow(`SELECT `+hostColumns+` FROM hosts WHERE id = ?`, id)
	h, err := scanHost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Host{}, ErrHostNotFound
	}
	if err != nil {
		return domain.Host{}, fmt.Errorf("sqlite: get host: %w", err)
	}
	return h, nil
}

func (r *HostRepo) Save(h domain.Host) (domain.Host, error) {
	labelsJSON, err := json.Marshal(h.Labels)
	if err != nil {
		return domain.Host{}, fmt.Errorf("sqlite: marshal labels: %w", err)
	}
	source := h.Source
	if source == "" {
		source = "local"
	}

	_, err = r.db.Exec(`
		INSERT INTO hosts (id, name, address, port, labels, username, auth_type, key_path, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			address = excluded.address,
			port = excluded.port,
			labels = excluded.labels,
			username = excluded.username,
			auth_type = excluded.auth_type,
			key_path = excluded.key_path,
			updated_at = excluded.updated_at
	`, h.ID, h.Name, h.Address, h.Port, string(labelsJSON), h.Username, string(h.AuthType), nullableString(h.KeyPath), source, h.CreatedAt.Unix(), h.UpdatedAt.Unix())
	if err != nil {
		return domain.Host{}, fmt.Errorf("sqlite: save host: %w", err)
	}

	return r.Get(h.ID)
}

func (r *HostRepo) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete host: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: delete host rows affected: %w", err)
	}
	if n == 0 {
		return ErrHostNotFound
	}
	return nil
}

func (r *HostRepo) TouchConnected(id string) error {
	res, err := r.db.Exec(`UPDATE hosts SET last_connected_at = ? WHERE id = ?`, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("sqlite: touch connected: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: touch connected rows affected: %w", err)
	}
	if n == 0 {
		return ErrHostNotFound
	}
	return nil
}

func (r *HostRepo) ListLabels() ([]string, error) {
	rows, err := r.db.Query(`SELECT labels FROM hosts`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list labels: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	var labels []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("sqlite: scan labels: %w", err)
		}
		var hostLabels []string
		if err := json.Unmarshal([]byte(raw), &hostLabels); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal labels: %w", err)
		}
		for _, l := range hostLabels {
			if !seen[l] {
				seen[l] = true
				labels = append(labels, l)
			}
		}
	}
	return labels, rows.Err()
}

// rowScanner abstracts over *sql.Row and *sql.Rows so scanHost works for both.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanHost(row rowScanner) (domain.Host, error) {
	var (
		h               domain.Host
		labelsJSON      string
		authType        string
		keyPath         sql.NullString
		createdAt       int64
		updatedAt       int64
		lastConnectedAt sql.NullInt64
	)

	if err := row.Scan(&h.ID, &h.Name, &h.Address, &h.Port, &labelsJSON, &h.Username, &authType, &keyPath, &h.Source, &createdAt, &updatedAt, &lastConnectedAt); err != nil {
		return domain.Host{}, err
	}

	if err := json.Unmarshal([]byte(labelsJSON), &h.Labels); err != nil {
		return domain.Host{}, fmt.Errorf("unmarshal labels: %w", err)
	}
	h.AuthType = domain.AuthType(authType)
	h.KeyPath = keyPath.String
	h.CreatedAt = time.Unix(createdAt, 0).UTC()
	h.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	if lastConnectedAt.Valid {
		t := time.Unix(lastConnectedAt.Int64, 0).UTC()
		h.LastConnectedAt = &t
	}
	return h, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
