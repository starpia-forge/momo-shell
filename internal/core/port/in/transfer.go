package in

import "momo-shell/internal/core/domain"

// ConflictPolicy resolves a name collision at the destination when
// uploading or downloading. The caller (frontend) diffs the destination
// listing up front and picks one policy per call -- the queue itself never
// prompts.
type ConflictPolicy string

const (
	ConflictOverwrite ConflictPolicy = "overwrite"
	ConflictRename    ConflictPolicy = "rename" // appends " (1)", " (2)", ...
	ConflictSkip      ConflictPolicy = "skip"
)

// TaskKind identifies what a transfer task is moving and in which direction.
type TaskKind string

const (
	TaskUpload   TaskKind = "upload"
	TaskDownload TaskKind = "download"
)

// TaskState is a transfer task's lifecycle state.
type TaskState string

const (
	TaskQueued   TaskState = "queued"
	TaskRunning  TaskState = "running"
	TaskDone     TaskState = "done"
	TaskFailed   TaskState = "failed"
	TaskCanceled TaskState = "canceled"
)

// TaskInfo is the read-only snapshot of one transfer task, published on
// transfer:task (every change) and returned by Tasks().
type TaskInfo struct {
	ID          string
	SessionID   string
	Kind        TaskKind
	State       TaskState
	Src         string
	Dst         string
	CurrentFile string
	Bytes       int64
	Total       int64
	Offset      int64 // reserved for v2 resume; always 0 in v1
	Error       string
}

// TransferUseCase is the driving port for SFTP file-browser and transfer
// operations. ZMODEM (rz/sz) is layered on top of this same interface
// starting in M5; M1-M3 only exercise the SFTP path.
type TransferUseCase interface {
	ListRemoteDir(sessionID, path string) ([]domain.RemoteEntry, error)
	HomeDir(sessionID string) (string, error)
	StatRemote(sessionID, path string) (entry domain.RemoteEntry, ok bool, err error)

	Upload(sessionID string, localPaths []string, remoteDir string, policy ConflictPolicy) (taskIDs []string, err error)
	Download(sessionID string, remotePaths []string, localDir string, policy ConflictPolicy) (taskIDs []string, err error)

	Mkdir(sessionID, path string) error
	Rename(sessionID, oldPath, newPath string) error
	Remove(sessionID, path string) error
	Chmod(sessionID, path string, mode uint32) error

	// CopyRemote stream-copies files (not directories) within one session's
	// remote file system into dstDir, keeping their basenames -- SFTP has no
	// server-side copy. Synchronous; files only in v1.
	CopyRemote(sessionID string, srcPaths []string, dstDir string) error

	Cancel(taskID string) error
	Tasks() []TaskInfo

	// StartZmodemSend answers a pending rz-upload detection (see
	// transfer:zmodem:{sessionId} "detected" events) with the files the
	// user picked or dropped.
	StartZmodemSend(sessionID string, localPaths []string) (taskID string, err error)
	// CancelZmodem aborts any in-flight ZMODEM transfer, or a pending
	// awaiting-send prompt, on a session.
	CancelZmodem(sessionID string) error
}
