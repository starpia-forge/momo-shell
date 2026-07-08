package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// ErrMCPClientNotFound is returned by Revoke/Delete for an unknown client ID.
var ErrMCPClientNotFound = errors.New("sqlite: mcp client not found")

// MCPClientRepo implements out.MCPClientRepository on top of the
// mcp_clients table.
type MCPClientRepo struct {
	db *sql.DB
}

var _ out.MCPClientRepository = (*MCPClientRepo)(nil)

func NewMCPClientRepo(db *sql.DB) *MCPClientRepo {
	return &MCPClientRepo{db: db}
}

const mcpClientColumns = `client_id, name, paired_at, last_seen_at, revoked`

func (r *MCPClientRepo) List() ([]domain.MCPClient, error) {
	rows, err := r.db.Query(`SELECT ` + mcpClientColumns + ` FROM mcp_clients ORDER BY paired_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list mcp clients: %w", err)
	}
	defer rows.Close()

	var clients []domain.MCPClient
	for rows.Next() {
		c, err := scanMCPClient(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan mcp client: %w", err)
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

func (r *MCPClientRepo) FindByTokenHash(hash string) (domain.MCPClient, bool, error) {
	row := r.db.QueryRow(`SELECT `+mcpClientColumns+` FROM mcp_clients WHERE token_hash = ?`, hash)
	c, err := scanMCPClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MCPClient{}, false, nil
	}
	if err != nil {
		return domain.MCPClient{}, false, fmt.Errorf("sqlite: find mcp client: %w", err)
	}
	return c, true, nil
}

func (r *MCPClientRepo) Save(c domain.MCPClient, tokenHash string) error {
	_, err := r.db.Exec(`
		INSERT INTO mcp_clients (client_id, name, token_hash, paired_at)
		VALUES (?, ?, ?, ?)
	`, c.ClientID, c.Name, tokenHash, c.PairedAt.Unix())
	if err != nil {
		return fmt.Errorf("sqlite: save mcp client: %w", err)
	}
	return nil
}

func (r *MCPClientRepo) Revoke(clientID string) error {
	res, err := r.db.Exec(`UPDATE mcp_clients SET revoked = 1 WHERE client_id = ?`, clientID)
	if err != nil {
		return fmt.Errorf("sqlite: revoke mcp client: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: revoke mcp client rows affected: %w", err)
	}
	if n == 0 {
		return ErrMCPClientNotFound
	}
	return nil
}

func (r *MCPClientRepo) Delete(clientID string) error {
	res, err := r.db.Exec(`DELETE FROM mcp_clients WHERE client_id = ?`, clientID)
	if err != nil {
		return fmt.Errorf("sqlite: delete mcp client: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: delete mcp client rows affected: %w", err)
	}
	if n == 0 {
		return ErrMCPClientNotFound
	}
	return nil
}

func (r *MCPClientRepo) TouchSeen(clientID string) error {
	_, err := r.db.Exec(`UPDATE mcp_clients SET last_seen_at = ? WHERE client_id = ?`, time.Now().Unix(), clientID)
	if err != nil {
		return fmt.Errorf("sqlite: touch mcp client seen: %w", err)
	}
	return nil
}

func scanMCPClient(row rowScanner) (domain.MCPClient, error) {
	var (
		c          domain.MCPClient
		pairedAt   int64
		lastSeenAt sql.NullInt64
		revoked    int
	)
	if err := row.Scan(&c.ClientID, &c.Name, &pairedAt, &lastSeenAt, &revoked); err != nil {
		return domain.MCPClient{}, err
	}
	c.PairedAt = time.Unix(pairedAt, 0).UTC()
	if lastSeenAt.Valid {
		t := time.Unix(lastSeenAt.Int64, 0).UTC()
		c.LastSeenAt = &t
	}
	c.Revoked = revoked != 0
	return c, nil
}
