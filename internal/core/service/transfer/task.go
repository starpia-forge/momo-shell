package transfer

import (
	"context"
	"sync"

	"momo-shell/internal/core/port/in"
)

// fileJob is one concrete file copy within a task, after any directory has
// been flattened at enqueue time.
type fileJob struct {
	src, dst string
	size     int64
}

// task tracks one enqueued transfer (a single file or a directory tree,
// copied as one unit). jobs is precomputed before the task starts running.
type task struct {
	jobs []fileJob

	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.Mutex
	info in.TaskInfo
}

func newTask(id, sessionID string, kind in.TaskKind, src, dst string, jobs []fileJob) *task {
	ctx, cancel := context.WithCancel(context.Background())
	var total int64
	for _, j := range jobs {
		total += j.size
	}
	return &task{
		jobs:   jobs,
		ctx:    ctx,
		cancel: cancel,
		info: in.TaskInfo{
			ID:        id,
			SessionID: sessionID,
			Kind:      kind,
			State:     in.TaskQueued,
			Src:       src,
			Dst:       dst,
			Total:     total,
		},
	}
}

func (t *task) snapshot() in.TaskInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.info
}

func (t *task) setState(state in.TaskState) {
	t.mu.Lock()
	t.info.State = state
	t.mu.Unlock()
}

func (t *task) fail(err error) {
	t.mu.Lock()
	t.info.State = in.TaskFailed
	t.info.Error = err.Error()
	t.mu.Unlock()
}

func (t *task) setCurrentFile(name string) {
	t.mu.Lock()
	t.info.CurrentFile = name
	t.mu.Unlock()
}

func (t *task) addBytes(n int64) {
	t.mu.Lock()
	t.info.Bytes += n
	t.mu.Unlock()
}
