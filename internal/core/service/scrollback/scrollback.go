// Package scrollback retains a bounded, seq-tagged ring of each session's
// raw output bytes (design doc 17 §7, doc 18 E2 2부-a). This is the missing
// data source E2(1부)'s mask.Service noted it needed: ReadScrollback (E2
// 2부-c) will later read a slice via ReadSince and run it through
// mask.Apply. Reading a live buffer with real secret/command wiring is
// deferred (E2 2부-b/c) -- this package only captures and replays raw
// bytes.
package scrollback

import "sync"

// defaultCapacityBytes bounds retained raw output per session. Comfortably
// larger than session/pump.go's 32 KiB flush threshold; old chunks are
// evicted first once exceeded.
const defaultCapacityBytes = 256 * 1024

// Service retains each session's raw output bytes so a later reader can
// replay a masked slice. It satisfies session.CommandTap structurally (no
// import of the session package, cf. history.Service).
type Service struct {
	maxBytes int

	mu      sync.Mutex // guards buffers map only
	buffers map[string]*sessionBuffer
}

type sessionBuffer struct {
	hostID string

	mu      sync.Mutex // guards chunks/bytes/lastSeq (off the pump hot path's map lock)
	chunks  []chunk
	bytes   int
	lastSeq uint64
}

type chunk struct {
	seq  uint64
	data []byte
}

func New() *Service {
	return NewWithCapacity(defaultCapacityBytes)
}

func NewWithCapacity(maxBytes int) *Service {
	return &Service{maxBytes: maxBytes, buffers: make(map[string]*sessionBuffer)}
}

// Attach registers a newly-running session for capture. hostID == "" means
// local (mirrors session.CommandTap.Attach's contract).
func (s *Service) Attach(sessionID string, hostID string) {
	s.mu.Lock()
	s.buffers[sessionID] = &sessionBuffer{hostID: hostID}
	s.mu.Unlock()
}

func (s *Service) Detach(sessionID string) {
	s.mu.Lock()
	delete(s.buffers, sessionID)
	s.mu.Unlock()
}

// OnInput is a no-op: the PTY echoes input back as output, so scrollback
// only needs to capture the output side.
func (s *Service) OnInput(sessionID string, data []byte) {}

func (s *Service) OnOutput(sessionID string, data []byte) {
	b := s.bufferFor(sessionID)
	if b == nil || len(data) == 0 {
		return
	}
	cp := append([]byte(nil), data...) // pump may reuse its buffer; never alias the caller's slice

	b.mu.Lock()
	b.lastSeq++
	b.chunks = append(b.chunks, chunk{seq: b.lastSeq, data: cp})
	b.bytes += len(cp)
	for len(b.chunks) > 1 && b.bytes > s.maxBytes { // never drop the newest chunk
		b.bytes -= len(b.chunks[0].data)
		b.chunks = b.chunks[1:]
	}
	b.mu.Unlock()
}

// ReadSince returns every captured byte after sinceSeq, plus the seq cursor
// to pass as sinceSeq on the next call. seq is a chunk cursor, not a byte
// offset, so it stays stable even though masking (applied by a later
// reader) can change a chunk's length. If sinceSeq refers to an
// already-evicted chunk, the remaining oldest chunks are returned
// (scrollback is bounded best-effort).
func (s *Service) ReadSince(sessionID string, sinceSeq uint64) (data []byte, nextSeq uint64) {
	b := s.bufferFor(sessionID)
	if b == nil {
		return nil, sinceSeq
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	next := sinceSeq
	var buf []byte
	for _, c := range b.chunks {
		if c.seq > sinceSeq {
			buf = append(buf, c.data...)
			next = c.seq
		}
	}
	return buf, next
}

// HostID reports the hostID passed to Attach for sessionID ("" for local
// sessions), and whether sessionID is currently attached.
func (s *Service) HostID(sessionID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buffers[sessionID]
	if !ok {
		return "", false
	}
	return b.hostID, true
}

func (s *Service) bufferFor(sessionID string) *sessionBuffer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffers[sessionID]
}
