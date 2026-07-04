package wails

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// TransferDialogs owns the Wails context needed for native file/directory
// pickers used by uploads and downloads -- kept separate from TransferService
// for the same reason as KeyFileBrowser: SetContext must never end up in the
// bound method set.
type TransferDialogs struct {
	ctx atomic.Pointer[context.Context]
}

func NewTransferDialogs() *TransferDialogs {
	return &TransferDialogs{}
}

// SetContext must be called from OnStartup before any dialog is used.
func (d *TransferDialogs) SetContext(ctx context.Context) {
	d.ctx.Store(&ctx)
}

func (d *TransferDialogs) chooseFiles() ([]string, error) {
	ctxPtr := d.ctx.Load()
	if ctxPtr == nil {
		return nil, errors.New("transfer dialogs: context not set")
	}
	return runtime.OpenMultipleFilesDialog(*ctxPtr, runtime.OpenDialogOptions{Title: "업로드할 파일 선택"})
}

func (d *TransferDialogs) chooseDirectory() (string, error) {
	ctxPtr := d.ctx.Load()
	if ctxPtr == nil {
		return "", errors.New("transfer dialogs: context not set")
	}
	return runtime.OpenDirectoryDialog(*ctxPtr, runtime.OpenDialogOptions{Title: "저장할 폴더 선택"})
}

// RemoteEntryDTO is the JSON-facing response DTO for one file-browser entry.
type RemoteEntryDTO struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Mode     uint32 `json:"mode"`
	ModeText string `json:"modeText"`
	ModTime  int64  `json:"modTime"` // unix ms
	IsDir    bool   `json:"isDir"`
}

// TaskInfoDTO is the JSON-facing snapshot of one transfer task, mirroring
// in.TaskInfo (also published verbatim-shaped on transfer:task).
type TaskInfoDTO struct {
	ID          string `json:"id"`
	SessionID   string `json:"sessionId"`
	Kind        string `json:"kind"`
	State       string `json:"state"`
	Src         string `json:"src"`
	Dst         string `json:"dst"`
	CurrentFile string `json:"currentFile,omitempty"`
	Bytes       int64  `json:"bytes"`
	Total       int64  `json:"total"`
	Offset      int64  `json:"offset"`
	Error       string `json:"error,omitempty"`
}

// TransferService is the Wails-bound facade over in.TransferUseCase. It only
// converts between JSON-facing DTOs and domain/port types -- no business logic.
type TransferService struct {
	uc      in.TransferUseCase
	dialogs *TransferDialogs
}

func NewTransferService(uc in.TransferUseCase, dialogs *TransferDialogs) *TransferService {
	return &TransferService{uc: uc, dialogs: dialogs}
}

// BrowseForUploadFiles opens a native multi-file picker and returns the
// chosen absolute paths, or an empty slice if the user cancelled.
func (s *TransferService) BrowseForUploadFiles() ([]string, error) {
	return s.dialogs.chooseFiles()
}

// BrowseForDownloadDirectory opens a native folder picker and returns the
// chosen path, or "" if the user cancelled.
func (s *TransferService) BrowseForDownloadDirectory() (string, error) {
	return s.dialogs.chooseDirectory()
}

func (s *TransferService) ListRemoteDir(sessionID, path string) ([]RemoteEntryDTO, error) {
	entries, err := s.uc.ListRemoteDir(sessionID, path)
	if err != nil {
		return nil, err
	}
	dtos := make([]RemoteEntryDTO, len(entries))
	for i, e := range entries {
		dtos[i] = remoteEntryToDTO(e)
	}
	return dtos, nil
}

func (s *TransferService) HomeDir(sessionID string) (string, error) {
	return s.uc.HomeDir(sessionID)
}

// StatRemote returns nil (not an error) when path doesn't exist -- Wails
// bindings only support (value, error) returns, so the "found" flag from
// in.TransferUseCase.StatRemote is folded into nil-vs-non-nil here.
func (s *TransferService) StatRemote(sessionID, path string) (*RemoteEntryDTO, error) {
	entry, ok, err := s.uc.StatRemote(sessionID, path)
	if err != nil || !ok {
		return nil, err
	}
	dto := remoteEntryToDTO(entry)
	return &dto, nil
}

func (s *TransferService) Upload(sessionID string, localPaths []string, remoteDir string, policy string) ([]string, error) {
	return s.uc.Upload(sessionID, localPaths, remoteDir, in.ConflictPolicy(policy))
}

func (s *TransferService) Download(sessionID string, remotePaths []string, localDir string, policy string) ([]string, error) {
	return s.uc.Download(sessionID, remotePaths, localDir, in.ConflictPolicy(policy))
}

func (s *TransferService) Mkdir(sessionID, path string) error {
	return s.uc.Mkdir(sessionID, path)
}

func (s *TransferService) Rename(sessionID, oldPath, newPath string) error {
	return s.uc.Rename(sessionID, oldPath, newPath)
}

func (s *TransferService) Remove(sessionID, path string) error {
	return s.uc.Remove(sessionID, path)
}

func (s *TransferService) Chmod(sessionID, path string, mode uint32) error {
	return s.uc.Chmod(sessionID, path, mode)
}

func (s *TransferService) CancelTransfer(taskID string) error {
	return s.uc.Cancel(taskID)
}

func (s *TransferService) ListTasks() []TaskInfoDTO {
	tasks := s.uc.Tasks()
	dtos := make([]TaskInfoDTO, len(tasks))
	for i, t := range tasks {
		dtos[i] = taskInfoToDTO(t)
	}
	return dtos
}

func remoteEntryToDTO(e domain.RemoteEntry) RemoteEntryDTO {
	return RemoteEntryDTO{
		Name:     e.Name,
		Path:     e.Path,
		Size:     e.Size,
		Mode:     e.Mode,
		ModeText: e.ModeText,
		ModTime:  e.ModTime.UnixMilli(),
		IsDir:    e.IsDir,
	}
}

func taskInfoToDTO(t in.TaskInfo) TaskInfoDTO {
	return TaskInfoDTO{
		ID:          t.ID,
		SessionID:   t.SessionID,
		Kind:        string(t.Kind),
		State:       string(t.State),
		Src:         t.Src,
		Dst:         t.Dst,
		CurrentFile: t.CurrentFile,
		Bytes:       t.Bytes,
		Total:       t.Total,
		Offset:      t.Offset,
		Error:       t.Error,
	}
}
