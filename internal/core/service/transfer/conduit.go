package transfer

import (
	"io"
	"sync"
)

// conduit bridges a ZMODEM engine (which wants a plain io.ReadWriter) onto
// a session's shell channel: output chunks the middleware diverts arrive
// via push (called from the pump goroutine), and Write goes back out
// through writeFn (the session's WriteRaw). push blocking when the engine
// is slow to Read is the same backpressure chain the pump already relies
// on elsewhere -- it propagates up through OnOutput, readLoop, and the
// transport's own flow control.
type conduit struct {
	in      chan []byte
	writeFn func([]byte) error

	closeOnce sync.Once
	closed    chan struct{}

	pending []byte // leftover from a chunk larger than the caller's Read buffer
}

func newConduit(writeFn func([]byte) error) *conduit {
	return &conduit{
		in:      make(chan []byte, 32),
		writeFn: writeFn,
		closed:  make(chan struct{}),
	}
}

// push hands a diverted output chunk to the engine. Safe to call after
// Close (becomes a no-op) so a race between the engine finishing and the
// pump delivering one more chunk can't block forever.
func (c *conduit) push(chunk []byte) {
	select {
	case c.in <- chunk:
	case <-c.closed:
	}
}

func (c *conduit) Read(p []byte) (int, error) {
	if len(c.pending) > 0 {
		n := copy(p, c.pending)
		c.pending = c.pending[n:]
		return n, nil
	}
	select {
	case chunk, ok := <-c.in:
		if !ok {
			return 0, io.EOF
		}
		n := copy(p, chunk)
		if n < len(chunk) {
			c.pending = chunk[n:]
		}
		return n, nil
	case <-c.closed:
		return 0, io.EOF
	}
}

func (c *conduit) Write(p []byte) (int, error) {
	if err := c.writeFn(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close unblocks any in-flight push or Read. Idempotent.
func (c *conduit) Close() {
	c.closeOnce.Do(func() { close(c.closed) })
}
