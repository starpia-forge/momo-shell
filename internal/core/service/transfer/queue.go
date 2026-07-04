package transfer

import "sync"

// maxConcurrentPerSession bounds how many tasks run at once for a single
// session's connection; the rest wait in FIFO order (spec: "연결당 동시
// 전송 2개").
const maxConcurrentPerSession = 2

// sessionQueue runs at most maxConcurrentPerSession tasks concurrently,
// queuing the rest. One instance exists per session with at least one
// enqueued task; run does the actual work and must itself observe
// t.ctx.Done() for cancellation.
type sessionQueue struct {
	mu      sync.Mutex
	running int
	pending []*task
}

func (q *sessionQueue) enqueue(t *task, run func(*task)) {
	q.mu.Lock()
	if q.running < maxConcurrentPerSession {
		q.running++
		q.mu.Unlock()
		go q.execute(t, run)
		return
	}
	q.pending = append(q.pending, t)
	q.mu.Unlock()
}

func (q *sessionQueue) execute(t *task, run func(*task)) {
	run(t)

	q.mu.Lock()
	var next *task
	if len(q.pending) > 0 {
		next = q.pending[0]
		q.pending = q.pending[1:]
	} else {
		q.running--
	}
	q.mu.Unlock()

	if next != nil {
		q.execute(next, run)
	}
}
