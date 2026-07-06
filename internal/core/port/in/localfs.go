package in

import "momo-shell/internal/core/domain"

// LocalFSUseCase is the driving port for the SFTP page's local pane: listing
// and manipulating the user's local file system. domain.RemoteEntry doubles
// as the generic file-system entry type (its fields already fit either side).
type LocalFSUseCase interface {
	ListDir(path string) ([]domain.RemoteEntry, error)
	HomeDir() (string, error)
	// Roots returns the local file system's top-level entry points --
	// ["/"] on Unix, or the available drive letters (e.g. "C:\\") on
	// Windows.
	Roots() ([]string, error)
	Stat(path string) (entry domain.RemoteEntry, ok bool, err error)

	Mkdir(path string) error
	Rename(oldPath, newPath string) error
	Remove(path string) error // file or empty directory

	// Copy copies files (not directories) into dstDir, keeping their
	// basenames.
	Copy(srcPaths []string, dstDir string) error
	// Move relocates files or directories into dstDir via os.Rename,
	// keeping their basenames. Falls back to copy+remove when os.Rename
	// fails on a file (e.g. a cross-volume move on Windows); a failed
	// cross-volume directory move is returned as-is (v1 has no recursive
	// directory copy).
	Move(srcPaths []string, dstDir string) error
}
