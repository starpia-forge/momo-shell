package out

import "momo-shell/internal/core/domain"

// MCPClientRepository persists MCP clients this instance has paired with
// (doc 20 D2), used to resolve a bearer token to a clientID (AuthClient,
// in.MCPServerCallbacks -- A2/A6). Tokens are stored hashed -- see the
// pairing service once it exists (A2). FindByTokenHash returns revoked
// rows as-is (Revoked=true); rejecting a revoked client's token is the
// caller's decision, not this port's.
type MCPClientRepository interface {
	List() ([]domain.MCPClient, error)
	FindByTokenHash(hash string) (domain.MCPClient, bool, error)
	Save(c domain.MCPClient, tokenHash string) error
	// Revoke soft-deletes: the row is kept (so audit history, E3, can
	// still resolve this clientID to a name after revocation) but flagged
	// so FindByTokenHash callers see Revoked=true.
	Revoke(clientID string) error
	// Delete hard-removes the row entirely -- distinct from Revoke.
	Delete(clientID string) error
	TouchSeen(clientID string) error
}
