package out

import (
	"io"

	"momo-shell/internal/core/domain"
)

// RemoteFileSystem is the driven port for file operations over an existing
// connection (an SFTP subsystem opened on the session's SSH connection).
// Implementations are per-session and must be Closed when no longer needed.
type RemoteFileSystem interface {
	ReadDir(path string) ([]domain.RemoteEntry, error)
	Stat(path string) (domain.RemoteEntry, error)
	Open(path string) (io.ReadCloser, error)    // download source
	Create(path string) (io.WriteCloser, error) // upload sink; truncates
	Mkdir(path string) error
	Rename(oldPath, newPath string) error
	Remove(path string) error // file or empty directory
	Chmod(path string, mode uint32) error
	// HomeDir resolves the remote user's home directory (sftp RealPath(".")),
	// used as the file browser's default location.
	HomeDir() (string, error)
	Close() error
}

// FileSystemCapable is an optional capability of a TerminalStream whose
// transport can open subsystem channels on the same underlying connection
// (SSH can; a local PTY cannot). The session service type-asserts a stream
// against this interface to serve TransferUseCase.FileSystem.
type FileSystemCapable interface {
	OpenFileSystem() (RemoteFileSystem, error)
}
