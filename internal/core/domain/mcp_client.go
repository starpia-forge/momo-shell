package domain

import "time"

// MCPClient is an MCP client this instance has paired with (issued a
// bearer token to, doc 20 D2). The token itself is never stored here --
// only its hash, held by the repository layer (see adapter/out/sqlite).
// Revoked keeps the row rather than deleting it (soft delete): audit
// history (E3) still needs to resolve a clientID to a name after
// revocation. Whether a revoked client's token is actually accepted is a
// decision for the caller (AuthClient, A2/A6), not this type.
type MCPClient struct {
	ClientID   string
	Name       string
	PairedAt   time.Time
	LastSeenAt *time.Time
	Revoked    bool
}
