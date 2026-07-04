package in

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// LocalOpts configures a new local shell session. Shell == "" means auto-detect.
type LocalOpts struct {
	Shell string
	Cwd   string
	Cols  int
	Rows  int
	Env   []string
}

// SSHOpts configures a new SSH session against a saved host.
type SSHOpts struct {
	HostID string
	Cols   int
	Rows   int
}

// SessionUseCase is the driving port for session lifecycle operations.
type SessionUseCase interface {
	CreateLocal(opts LocalOpts) (domain.SessionInfo, error)
	// CreateSSH returns immediately with the session in a Connecting state;
	// the actual dial/handshake/auth happens in the background and is
	// reported via session:state events.
	CreateSSH(opts SSHOpts) (domain.SessionInfo, error)
	Write(id string, data []byte) error
	Resize(id string, cols, rows int) error
	Close(id string) error
	// CloseAll synchronously closes every live session (used on app shutdown).
	CloseAll()
	// RespondHostKey answers a pending session:hostkey:{id} prompt raised
	// while connecting. decision is one of "trust", "once", or "cancel".
	RespondHostKey(sessionID string, decision string) error
	// FileSystem lazily opens (and caches, per session) the SFTP subsystem
	// on the session's connection. Returns an error for local sessions or
	// SSH servers with SFTP disabled -- callers use that to fall back to
	// the rz/sz path.
	FileSystem(sessionID string) (out.RemoteFileSystem, error)
}
