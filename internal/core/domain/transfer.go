package domain

import "time"

// RemoteEntry is one file-system entry (file or directory) returned by a
// directory listing or stat over a RemoteFileSystem.
type RemoteEntry struct {
	Name     string
	Path     string
	Size     int64
	Mode     uint32 // raw POSIX mode bits (os.FileMode-compatible)
	ModeText string // ls-style rendering, e.g. "drwxr-xr-x"
	ModTime  time.Time
	IsDir    bool
}
