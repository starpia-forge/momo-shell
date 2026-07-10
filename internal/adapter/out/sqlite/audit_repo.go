package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// AuditRepo implements out.AuditRepository on top of the mcp_audit table.
type AuditRepo struct {
	db *sql.DB
}

var _ out.AuditRepository = (*AuditRepo)(nil)

func NewAuditRepo(db *sql.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

const auditColumns = `id, ts, client_id, session_id, kind, target, original_cmd, guarded_cmd, resolve_json, approver, decision`

func (r *AuditRepo) Append(e domain.AuditEvent) error {
	resolveJSON, err := json.Marshal(e.Resolve)
	if err != nil {
		return fmt.Errorf("sqlite: marshal audit resolve summary: %w", err)
	}

	_, err = r.db.Exec(`
		INSERT INTO mcp_audit (ts, client_id, session_id, kind, target, original_cmd, guarded_cmd, resolve_json, approver, decision)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.Timestamp.Unix(), e.ClientID, e.SessionID, string(e.Kind), e.Target, e.OriginalCmd, e.GuardedCmd, string(resolveJSON), e.Approver, e.Decision)
	if err != nil {
		return fmt.Errorf("sqlite: append audit event: %w", err)
	}
	return nil
}

func (r *AuditRepo) List(sessionID string) ([]domain.AuditEvent, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if sessionID == "" {
		rows, err = r.db.Query(`SELECT ` + auditColumns + ` FROM mcp_audit ORDER BY id ASC`)
	} else {
		rows, err = r.db.Query(`SELECT `+auditColumns+` FROM mcp_audit WHERE session_id = ? ORDER BY id ASC`, sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: list audit events: %w", err)
	}
	defer rows.Close()

	var events []domain.AuditEvent
	for rows.Next() {
		e, err := scanAuditEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan audit event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func scanAuditEvent(row rowScanner) (domain.AuditEvent, error) {
	var (
		e           domain.AuditEvent
		ts          int64
		kind        string
		resolveJSON string
	)
	if err := row.Scan(&e.ID, &ts, &e.ClientID, &e.SessionID, &kind, &e.Target, &e.OriginalCmd, &e.GuardedCmd, &resolveJSON, &e.Approver, &e.Decision); err != nil {
		return domain.AuditEvent{}, err
	}
	e.Timestamp = time.Unix(ts, 0).UTC()
	e.Kind = domain.AuditKind(kind)
	if err := json.Unmarshal([]byte(resolveJSON), &e.Resolve); err != nil {
		return domain.AuditEvent{}, fmt.Errorf("unmarshal resolve summary: %w", err)
	}
	return e, nil
}
