package zmodem

import (
	"errors"
	"io"
	"time"
)

// errReadTimeout marks a stalled read: the peer hasn't sent any bytes
// within ioReadTimeout. It's a recoverable condition -- callers use it to
// decide whether to poke the peer (resend the frame it might have missed)
// and retry, unlike EOF or a hard I/O error, which are terminal.
var errReadTimeout = errors.New("zmodem: read timeout")

const (
	// ioReadTimeout bounds how long a read waits for the next byte before
	// errReadTimeout lets the caller retry instead of blocking forever --
	// this is what turns a stalled rz/sz exchange into a bounded retry loop
	// instead of the indefinite hang seen in production (FT-11/FT-12).
	ioReadTimeout = 10 * time.Second
	// maxIORetries bounds how many times a stalled read is retried (each
	// retry re-pokes the peer) before the transfer gives up for good.
	maxIORetries = 10
)

// timeoutReader wraps an io.Reader with a single, persistent background
// goroutine that pumps raw chunks into a channel. This lets next() apply a
// per-call deadline without ever needing a second, overlapping call into
// the underlying reader: there is exactly one goroutine reading from src
// for timeoutReader's entire lifetime, so a timed-out call can simply be
// retried by the caller with zero risk of two reads racing on src (the
// hazard the old per-attempt-goroutine pattern had).
type timeoutReader struct {
	chunks chan []byte
	done   chan error // src's terminal error (io.EOF or otherwise), sent once
}

func newTimeoutReader(src io.Reader) *timeoutReader {
	t := &timeoutReader{
		chunks: make(chan []byte),
		done:   make(chan error, 1),
	}
	go t.pump(src)
	return t
}

func (t *timeoutReader) pump(src io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			t.chunks <- chunk
		}
		if err != nil {
			t.done <- err
			return
		}
	}
}

// next returns the next chunk read from src, the terminal error once src is
// exhausted or fails, or errReadTimeout if neither happens within d. On
// timeout the pump keeps running in the background; a later call can still
// observe whatever it eventually reads.
func (t *timeoutReader) next(d time.Duration) ([]byte, error) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case chunk := <-t.chunks:
		return chunk, nil
	case err := <-t.done:
		return nil, err
	case <-timer.C:
		return nil, errReadTimeout
	}
}
