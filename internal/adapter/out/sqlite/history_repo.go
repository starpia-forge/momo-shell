package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/out"
)

// ErrHistoryEntryNotFound is returned by TouchLast/Delete for an unknown ID.
var ErrHistoryEntryNotFound = errors.New("sqlite: history entry not found")

// HistoryRepo implements out.HistoryRepository on top of the
// command_history table (schema already created by migrateV1).
type HistoryRepo struct {
	db *sql.DB
}

var _ out.HistoryRepository = (*HistoryRepo)(nil)

func NewHistoryRepo(db *sql.DB) *HistoryRepo {
	return &HistoryRepo{db: db}
}

const historyColumns = `id, host_id, command, executed_at`

func (r *HistoryRepo) Append(hostID *string, command string, executedAt int64) (domain.HistoryEntry, error) {
	res, err := r.db.Exec(`INSERT INTO command_history (host_id, command, executed_at) VALUES (?, ?, ?)`,
		nullableStringPtr(hostID), command, executedAt)
	if err != nil {
		return domain.HistoryEntry{}, fmt.Errorf("sqlite: append history: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.HistoryEntry{}, fmt.Errorf("sqlite: append history last insert id: %w", err)
	}
	return domain.HistoryEntry{ID: id, HostID: hostID, Command: command, ExecutedAt: executedAt}, nil
}

func (r *HistoryRepo) TouchLast(id int64, executedAt int64) error {
	res, err := r.db.Exec(`UPDATE command_history SET executed_at = ? WHERE id = ?`, executedAt, id)
	if err != nil {
		return fmt.Errorf("sqlite: touch history: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: touch history rows affected: %w", err)
	}
	if n == 0 {
		return ErrHistoryEntryNotFound
	}
	return nil
}

func (r *HistoryRepo) LastForHost(hostID *string) (domain.HistoryEntry, bool, error) {
	var row *sql.Row
	if hostID == nil {
		row = r.db.QueryRow(`SELECT ` + historyColumns + ` FROM command_history WHERE host_id IS NULL ORDER BY executed_at DESC LIMIT 1`)
	} else {
		row = r.db.QueryRow(`SELECT `+historyColumns+` FROM command_history WHERE host_id = ? ORDER BY executed_at DESC LIMIT 1`, *hostID)
	}

	entry, err := scanHistoryEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.HistoryEntry{}, false, nil
	}
	if err != nil {
		return domain.HistoryEntry{}, false, fmt.Errorf("sqlite: last history for host: %w", err)
	}
	return entry, true, nil
}

func (r *HistoryRepo) List(q domain.HistoryQuery) ([]domain.HistoryEntry, error) {
	query := `SELECT ` + historyColumns + ` FROM command_history WHERE 1=1`
	var args []any
	if q.HostID != nil {
		query += ` AND host_id = ?`
		args = append(args, *q.HostID)
	}
	if q.Search != "" {
		query += ` AND command LIKE ?`
		args = append(args, "%"+q.Search+"%")
	}
	query += ` ORDER BY executed_at DESC`
	if q.Limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, q.Limit, q.Offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list history: %w", err)
	}
	defer rows.Close()

	var entries []domain.HistoryEntry
	for rows.Next() {
		e, err := scanHistoryEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *HistoryRepo) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM command_history WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete history: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: delete history rows affected: %w", err)
	}
	if n == 0 {
		return ErrHistoryEntryNotFound
	}
	return nil
}

func (r *HistoryRepo) Clear(hostID *string) error {
	if hostID == nil {
		if _, err := r.db.Exec(`DELETE FROM command_history`); err != nil {
			return fmt.Errorf("sqlite: clear all history: %w", err)
		}
		return nil
	}
	if _, err := r.db.Exec(`DELETE FROM command_history WHERE host_id = ?`, *hostID); err != nil {
		return fmt.Errorf("sqlite: clear host history: %w", err)
	}
	return nil
}

func scanHistoryEntry(row rowScanner) (domain.HistoryEntry, error) {
	var (
		e      domain.HistoryEntry
		hostID sql.NullString
	)
	if err := row.Scan(&e.ID, &hostID, &e.Command, &e.ExecutedAt); err != nil {
		return domain.HistoryEntry{}, err
	}
	if hostID.Valid {
		e.HostID = &hostID.String
	}
	return e, nil
}

func nullableStringPtr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
