package in

import "momo-terminal/internal/core/domain"

// LocalOpts configures a new local shell session. Shell == "" means auto-detect.
type LocalOpts struct {
	Shell string
	Cwd   string
	Cols  int
	Rows  int
	Env   []string
}

// SessionUseCase is the driving port for session lifecycle operations.
type SessionUseCase interface {
	CreateLocal(opts LocalOpts) (domain.SessionInfo, error)
	Write(id string, data []byte) error
	Resize(id string, cols, rows int) error
	Close(id string) error
	// CloseAll synchronously closes every live session (used on app shutdown).
	CloseAll()
}
