package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// ErrShareClientNotFound is returned by Delete for an unknown client ID.
var ErrShareClientNotFound = errors.New("sqlite: share client not found")

// ShareClientRepo implements out.ShareClientRepository on top of the
// share_clients table.
type ShareClientRepo struct {
	db *sql.DB
}

var _ out.ShareClientRepository = (*ShareClientRepo)(nil)

func NewShareClientRepo(db *sql.DB) *ShareClientRepo {
	return &ShareClientRepo{db: db}
}

func (r *ShareClientRepo) List() ([]domain.ShareClient, error) {
	rows, err := r.db.Query(`SELECT id, name, paired_at, last_seen_at FROM share_clients ORDER BY paired_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list share clients: %w", err)
	}
	defer rows.Close()

	var clients []domain.ShareClient
	for rows.Next() {
		c, err := scanShareClient(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan share client: %w", err)
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

func (r *ShareClientRepo) FindByTokenHash(hash string) (domain.ShareClient, bool, error) {
	row := r.db.QueryRow(`SELECT id, name, paired_at, last_seen_at FROM share_clients WHERE token_hash = ?`, hash)
	c, err := scanShareClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ShareClient{}, false, nil
	}
	if err != nil {
		return domain.ShareClient{}, false, fmt.Errorf("sqlite: find share client: %w", err)
	}
	return c, true, nil
}

func (r *ShareClientRepo) Save(c domain.ShareClient, tokenHash string) error {
	_, err := r.db.Exec(`
		INSERT INTO share_clients (id, name, token_hash, paired_at)
		VALUES (?, ?, ?, ?)
	`, c.ID, c.Name, tokenHash, c.PairedAt.Unix())
	if err != nil {
		return fmt.Errorf("sqlite: save share client: %w", err)
	}
	return nil
}

func (r *ShareClientRepo) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM share_clients WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete share client: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: delete share client rows affected: %w", err)
	}
	if n == 0 {
		return ErrShareClientNotFound
	}
	return nil
}

func (r *ShareClientRepo) TouchSeen(id string) error {
	_, err := r.db.Exec(`UPDATE share_clients SET last_seen_at = ? WHERE id = ?`, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("sqlite: touch share client seen: %w", err)
	}
	return nil
}

func scanShareClient(row rowScanner) (domain.ShareClient, error) {
	var (
		c          domain.ShareClient
		pairedAt   int64
		lastSeenAt sql.NullInt64
	)
	if err := row.Scan(&c.ID, &c.Name, &pairedAt, &lastSeenAt); err != nil {
		return domain.ShareClient{}, err
	}
	c.PairedAt = time.Unix(pairedAt, 0).UTC()
	if lastSeenAt.Valid {
		t := time.Unix(lastSeenAt.Int64, 0).UTC()
		c.LastSeenAt = &t
	}
	return c, nil
}

// ShareSettingsRepo implements out.ShareSettings on top of the
// share_settings key-value table.
type ShareSettingsRepo struct {
	db *sql.DB
}

var _ out.ShareSettings = (*ShareSettingsRepo)(nil)

func NewShareSettingsRepo(db *sql.DB) *ShareSettingsRepo {
	return &ShareSettingsRepo{db: db}
}

const (
	settingsKeyInstanceID    = "instance_id"
	settingsKeySharedHostIDs = "shared_host_ids"
)

// InstanceID returns this instance's persistent identifier, generating and
// storing one on first call.
func (r *ShareSettingsRepo) InstanceID() (string, error) {
	value, found, err := r.get(settingsKeyInstanceID)
	if err != nil {
		return "", err
	}
	if found {
		return value, nil
	}
	id := uuid.NewString()
	if err := r.set(settingsKeyInstanceID, id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *ShareSettingsRepo) SharedHostIDs() ([]string, error) {
	value, found, err := r.get(settingsKeySharedHostIDs)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal shared host ids: %w", err)
	}
	return ids, nil
}

func (r *ShareSettingsRepo) SetSharedHostIDs(ids []string) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("sqlite: marshal shared host ids: %w", err)
	}
	return r.set(settingsKeySharedHostIDs, string(data))
}

func (r *ShareSettingsRepo) get(key string) (string, bool, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM share_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("sqlite: get setting %s: %w", key, err)
	}
	return value, true, nil
}

func (r *ShareSettingsRepo) set(key, value string) error {
	_, err := r.db.Exec(`
		INSERT INTO share_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("sqlite: set setting %s: %w", key, err)
	}
	return nil
}
