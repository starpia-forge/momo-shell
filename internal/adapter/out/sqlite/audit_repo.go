package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
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

// Append inserts e and returns its assigned id, so a caller (E3-b's capture
// tap) can later attach output to this exact row via UpdateOutputRef --
// LastInsertId is race-free where a MAX(id) lookup wouldn't be, since
// aicontrol doesn't strictly enforce one in-flight command per session.
func (r *AuditRepo) Append(e domain.AuditEvent) (int64, error) {
	resolveJSON, err := json.Marshal(e.Resolve)
	if err != nil {
		return 0, fmt.Errorf("sqlite: marshal audit resolve summary: %w", err)
	}

	result, err := r.db.Exec(`
		INSERT INTO mcp_audit (ts, client_id, session_id, kind, target, original_cmd, guarded_cmd, resolve_json, approver, decision)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.Timestamp.Unix(), e.ClientID, e.SessionID, string(e.Kind), e.Target, e.OriginalCmd, e.GuardedCmd, string(resolveJSON), e.Approver, e.Decision)
	if err != nil {
		return 0, fmt.Errorf("sqlite: append audit event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("sqlite: read audit event id: %w", err)
	}
	return id, nil
}

// UpdateOutputRef attaches auditID's encrypted output capture (E3-b). The
// plaintext audit trail (List/scanAuditEvent) never selects this column --
// it is a side-channel reached only by explicit id (LoadOutput), so the
// AI-facing MCP surface has no path to it.
func (r *AuditRepo) UpdateOutputRef(auditID int64, ciphertext []byte) error {
	if _, err := r.db.Exec(`UPDATE mcp_audit SET output_ref = ? WHERE id = ?`, ciphertext, auditID); err != nil {
		return fmt.Errorf("sqlite: update audit output_ref: %w", err)
	}
	return nil
}

// LoadOutput returns auditID's encrypted output capture, or (nil, nil) if
// none is attached (no capture yet, a non-command event, or retention has
// already nulled it). Not exposed to the AI -- only E5's audit panel and
// tests call this.
func (r *AuditRepo) LoadOutput(auditID int64) ([]byte, error) {
	var ciphertext []byte
	err := r.db.QueryRow(`SELECT output_ref FROM mcp_audit WHERE id = ?`, auditID).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: load audit output_ref: %w", err)
	}
	return ciphertext, nil
}

// ClearOutputsBefore nulls output_ref for every row older than cutoffUnix
// that still has one attached (audit.Service.PurgeOutputsBefore's retention
// sweep) and reports how many rows were cleared. The decision row itself is
// never touched -- retention bounds the forensic output blob, not the
// audit trail.
func (r *AuditRepo) ClearOutputsBefore(cutoffUnix int64) (int64, error) {
	result, err := r.db.Exec(`UPDATE mcp_audit SET output_ref = NULL WHERE output_ref IS NOT NULL AND ts < ?`, cutoffUnix)
	if err != nil {
		return 0, fmt.Errorf("sqlite: clear expired audit outputs: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sqlite: read cleared audit output count: %w", err)
	}
	return n, nil
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
