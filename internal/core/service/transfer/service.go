// Package transfer implements in.TransferUseCase: SFTP file-browser
// operations and the upload/download task queue. ZMODEM (rz/sz) is added on
// top of this same Service in a later milestone via the session output
// middleware seam.
package transfer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// ErrTaskNotFound is returned by Cancel for an unknown or already-finished task ID.
var ErrTaskNotFound = errors.New("transfer: task not found")

const (
	copyBufSize      = 32 * 1024
	progressInterval = 200 * time.Millisecond
)

// ShellAccess is the narrow, core-defined interface transfer.Service needs
// from the session service -- consumer-side, matching how session.CommandTap
// is defined by its consumer rather than by session itself.
type ShellAccess interface {
	FileSystem(sessionID string) (out.RemoteFileSystem, error)
}

// Deps are the out-ports/collaborators Service needs.
type Deps struct {
	Shell ShellAccess
	Pub   out.EventPublisher
}

// Service implements in.TransferUseCase with an in-memory task registry and
// a per-session concurrency-limited queue (v1 has no persistence or resume;
// see TaskInfo.Offset).
type Service struct {
	shell ShellAccess
	pub   out.EventPublisher

	mu     sync.Mutex
	tasks  map[string]*task
	queues map[string]*sessionQueue
}

var _ in.TransferUseCase = (*Service)(nil)

func New(deps Deps) *Service {
	return &Service{
		shell:  deps.Shell,
		pub:    deps.Pub,
		tasks:  make(map[string]*task),
		queues: make(map[string]*sessionQueue),
	}
}

func (s *Service) ListRemoteDir(sessionID, dir string) ([]domain.RemoteEntry, error) {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return nil, err
	}
	return remote.ReadDir(dir)
}

func (s *Service) HomeDir(sessionID string) (string, error) {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return "", err
	}
	return remote.HomeDir()
}

func (s *Service) StatRemote(sessionID, p string) (domain.RemoteEntry, bool, error) {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return domain.RemoteEntry{}, false, err
	}
	entry, err := remote.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.RemoteEntry{}, false, nil
		}
		return domain.RemoteEntry{}, false, err
	}
	return entry, true, nil
}

func (s *Service) Mkdir(sessionID, p string) error {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return err
	}
	return remote.Mkdir(p)
}

func (s *Service) Rename(sessionID, oldPath, newPath string) error {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return err
	}
	return remote.Rename(oldPath, newPath)
}

func (s *Service) Remove(sessionID, p string) error {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return err
	}
	return remote.Remove(p)
}

func (s *Service) Chmod(sessionID, p string, mode uint32) error {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return err
	}
	return remote.Chmod(p, mode)
}

// Upload enqueues one task per entry in localPaths (a file or a directory,
// transferred recursively) into remoteDir. Entries resolved to "skip" by
// policy are silently omitted from the returned task IDs.
func (s *Service) Upload(sessionID string, localPaths []string, remoteDir string, policy in.ConflictPolicy) ([]string, error) {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(localPaths))
	for _, localPath := range localPaths {
		t, skipped, err := s.buildUploadTask(sessionID, remote, localPath, remoteDir, policy)
		if err != nil {
			return ids, err
		}
		if skipped {
			continue
		}
		s.start(sessionID, t, func(tt *task) { s.runUpload(remote, tt) })
		ids = append(ids, t.info.ID)
	}
	return ids, nil
}

// Download enqueues one task per entry in remotePaths into localDir.
func (s *Service) Download(sessionID string, remotePaths []string, localDir string, policy in.ConflictPolicy) ([]string, error) {
	remote, err := s.shell.FileSystem(sessionID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(remotePaths))
	for _, remotePath := range remotePaths {
		t, skipped, err := s.buildDownloadTask(sessionID, remote, remotePath, localDir, policy)
		if err != nil {
			return ids, err
		}
		if skipped {
			continue
		}
		s.start(sessionID, t, func(tt *task) { s.runDownload(remote, tt) })
		ids = append(ids, t.info.ID)
	}
	return ids, nil
}

func (s *Service) Cancel(taskID string) error {
	s.mu.Lock()
	t, ok := s.tasks[taskID]
	s.mu.Unlock()
	if !ok {
		return ErrTaskNotFound
	}

	t.cancel()
	// A still-queued task's copy loop hasn't started, so it won't observe
	// ctx.Done() until its turn -- flip it to Canceled now so the UI
	// reflects the cancellation immediately, matching the "취소 즉시 반영"
	// acceptance criterion. If it has already moved to Running, runCopy's
	// own ctx check settles the final state (harmless if this races it).
	if t.snapshot().State == in.TaskQueued {
		t.setState(in.TaskCanceled)
		s.publishTask(t)
	}
	return nil
}

func (s *Service) Tasks() []in.TaskInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	infos := make([]in.TaskInfo, 0, len(s.tasks))
	for _, t := range s.tasks {
		infos = append(infos, t.snapshot())
	}
	return infos
}

// start registers t and hands it to the session's queue.
func (s *Service) start(sessionID string, t *task, run func(*task)) {
	s.mu.Lock()
	s.tasks[t.info.ID] = t
	q, ok := s.queues[sessionID]
	if !ok {
		q = &sessionQueue{}
		s.queues[sessionID] = q
	}
	s.mu.Unlock()

	s.publishTask(t)
	q.enqueue(t, run)
}

func (s *Service) publishTask(t *task) {
	if s.pub != nil {
		s.pub.Publish(out.TopicTransferTask(), t.snapshot())
	}
}

// progressPayload is published on transfer:progress:{taskId}.
type progressPayload struct {
	Bytes int64   `json:"bytes"`
	Total int64   `json:"total"`
	Rate  float64 `json:"rate"` // bytes/sec since the previous publish
	State string  `json:"state"`
	File  string  `json:"file"`
}

func (s *Service) publishProgress(t *task, rate float64) {
	if s.pub == nil {
		return
	}
	info := t.snapshot()
	s.pub.Publish(out.TopicTransferProgress(info.ID), progressPayload{
		Bytes: info.Bytes,
		Total: info.Total,
		Rate:  rate,
		State: string(info.State),
		File:  info.CurrentFile,
	})
}

func (s *Service) runUpload(remote out.RemoteFileSystem, t *task) {
	s.runCopy(t,
		func(job fileJob) (io.ReadCloser, error) { return os.Open(job.src) },
		func(job fileJob) (io.WriteCloser, error) { return remote.Create(job.dst) },
		func(dst string) error { return remoteMkdirAll(remote, path.Dir(dst)) },
	)
}

func (s *Service) runDownload(remote out.RemoteFileSystem, t *task) {
	s.runCopy(t,
		func(job fileJob) (io.ReadCloser, error) { return remote.Open(job.src) },
		func(job fileJob) (io.WriteCloser, error) { return os.Create(job.dst) },
		func(dst string) error { return os.MkdirAll(filepath.Dir(dst), 0o755) },
	)
}

// runCopy streams every job's bytes in copyBufSize chunks, publishing
// throttled progress and checking ctx.Done() at chunk granularity so a
// cancel takes effect within one read, not after a whole file -- required
// for both responsiveness on 1GB transfers and immediate cancel.
func (s *Service) runCopy(t *task, openSrc func(fileJob) (io.ReadCloser, error), openDst func(fileJob) (io.WriteCloser, error), mkDirFor func(dst string) error) {
	t.setState(in.TaskRunning)
	s.publishTask(t)

	buf := make([]byte, copyBufSize)
	lastPublish := time.Now()
	lastBytes := int64(0)

	for _, job := range t.jobs {
		if t.ctx.Err() != nil {
			t.setState(in.TaskCanceled)
			s.publishTask(t)
			return
		}

		if err := mkDirFor(job.dst); err != nil {
			t.fail(err)
			s.publishTask(t)
			return
		}

		t.setCurrentFile(job.src)

		src, err := openSrc(job)
		if err != nil {
			t.fail(err)
			s.publishTask(t)
			return
		}
		dst, err := openDst(job)
		if err != nil {
			src.Close()
			t.fail(err)
			s.publishTask(t)
			return
		}

		for {
			if t.ctx.Err() != nil {
				src.Close()
				dst.Close()
				t.setState(in.TaskCanceled)
				s.publishTask(t)
				return
			}

			n, rerr := src.Read(buf)
			if n > 0 {
				if _, werr := dst.Write(buf[:n]); werr != nil {
					src.Close()
					dst.Close()
					t.fail(werr)
					s.publishTask(t)
					return
				}
				t.addBytes(int64(n))

				if elapsed := time.Since(lastPublish); elapsed >= progressInterval {
					bytes := t.snapshot().Bytes
					rate := float64(bytes-lastBytes) / elapsed.Seconds()
					s.publishProgress(t, rate)
					lastPublish = time.Now()
					lastBytes = bytes
				}
			}
			if rerr != nil {
				src.Close()
				dst.Close()
				if rerr != io.EOF {
					t.fail(rerr)
					s.publishTask(t)
					return
				}
				break
			}
		}
	}

	t.setState(in.TaskDone)
	s.publishProgress(t, 0)
	s.publishTask(t)
}

// buildUploadTask stats localPath, resolves a same-name collision at
// remoteDir per policy, and -- for a directory -- walks it into a flat job
// list with remote destinations under the (possibly renamed) top-level entry.
func (s *Service) buildUploadTask(sessionID string, remote out.RemoteFileSystem, localPath, remoteDir string, policy in.ConflictPolicy) (t *task, skipped bool, err error) {
	localInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, false, err
	}

	name, skip, err := resolveRemoteConflict(remote, remoteDir, filepath.Base(localPath), policy)
	if err != nil {
		return nil, false, err
	}
	if skip {
		return nil, true, nil
	}
	remoteDst := path.Join(remoteDir, name)

	var jobs []fileJob
	if localInfo.IsDir() {
		walkErr := filepath.WalkDir(localPath, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(localPath, p)
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			jobs = append(jobs, fileJob{src: p, dst: path.Join(remoteDst, filepath.ToSlash(rel)), size: info.Size()})
			return nil
		})
		if walkErr != nil {
			return nil, false, walkErr
		}
	} else {
		jobs = []fileJob{{src: localPath, dst: remoteDst, size: localInfo.Size()}}
	}

	return newTask(uuid.NewString(), sessionID, in.TaskUpload, localPath, remoteDst, jobs), false, nil
}

// buildDownloadTask is buildUploadTask's mirror: stats remotePath, resolves
// a collision at localDir, and -- for a directory -- recursively lists it
// into a flat job list with local destinations under the resolved name.
func (s *Service) buildDownloadTask(sessionID string, remote out.RemoteFileSystem, remotePath, localDir string, policy in.ConflictPolicy) (t *task, skipped bool, err error) {
	remoteInfo, err := remote.Stat(remotePath)
	if err != nil {
		return nil, false, err
	}

	name, skip, err := resolveLocalConflict(localDir, path.Base(remotePath), policy)
	if err != nil {
		return nil, false, err
	}
	if skip {
		return nil, true, nil
	}
	localDst := filepath.Join(localDir, name)

	var jobs []fileJob
	if remoteInfo.IsDir {
		files, err := listRemoteFiles(remote, remotePath)
		if err != nil {
			return nil, false, err
		}
		for _, f := range files {
			rel := strings.TrimPrefix(f.Path, remotePath+"/")
			jobs = append(jobs, fileJob{src: f.Path, dst: filepath.Join(localDst, filepath.FromSlash(rel)), size: f.Size})
		}
	} else {
		jobs = []fileJob{{src: remotePath, dst: localDst, size: remoteInfo.Size}}
	}

	return newTask(uuid.NewString(), sessionID, in.TaskDownload, remotePath, localDst, jobs), false, nil
}

// listRemoteFiles recursively lists every non-directory entry under root.
func listRemoteFiles(remote out.RemoteFileSystem, root string) ([]domain.RemoteEntry, error) {
	var files []domain.RemoteEntry
	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := remote.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir {
				if err := walk(e.Path); err != nil {
					return err
				}
				continue
			}
			files = append(files, e)
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	return files, nil
}

// remoteMkdirAll is sftp.Client.Mkdir (single-level) made recursive, since
// pkg/sftp has no MkdirAll on the interface surface we depend on.
func remoteMkdirAll(remote out.RemoteFileSystem, dir string) error {
	if dir == "" || dir == "." || dir == "/" {
		return nil
	}
	if _, err := remote.Stat(dir); err == nil {
		return nil
	}
	if parent := path.Dir(dir); parent != dir {
		if err := remoteMkdirAll(remote, parent); err != nil {
			return err
		}
	}
	return remote.Mkdir(dir)
}

// resolveRemoteConflict and resolveLocalConflict decide the final entry
// name for a top-level upload/download item per ConflictPolicy, checked
// once against the destination directory (files inside a transferred
// directory tree are never individually conflict-checked -- the tree is
// resolved as one unit under its own possibly-renamed root).

func resolveRemoteConflict(remote out.RemoteFileSystem, dir, name string, policy in.ConflictPolicy) (finalName string, skip bool, err error) {
	return resolveConflict(name, policy, func(candidate string) (bool, error) {
		_, statErr := remote.Stat(path.Join(dir, candidate))
		return statErr == nil, nil
	})
}

func resolveLocalConflict(dir, name string, policy in.ConflictPolicy) (finalName string, skip bool, err error) {
	return resolveConflict(name, policy, func(candidate string) (bool, error) {
		_, statErr := os.Stat(filepath.Join(dir, candidate))
		return statErr == nil, nil
	})
}

func resolveConflict(name string, policy in.ConflictPolicy, exists func(candidate string) (bool, error)) (finalName string, skip bool, err error) {
	found, err := exists(name)
	if err != nil {
		return "", false, err
	}
	if !found {
		return name, false, nil
	}

	switch policy {
	case in.ConflictOverwrite:
		return name, false, nil
	case in.ConflictSkip:
		return "", true, nil
	case in.ConflictRename:
		ext := path.Ext(name)
		base := strings.TrimSuffix(name, ext)
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
			found, err := exists(candidate)
			if err != nil {
				return "", false, err
			}
			if !found {
				return candidate, false, nil
			}
		}
	default:
		return name, false, nil
	}
}
