package session

import (
	"fmt"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

const (
	flushInterval  = 16 * time.Millisecond
	flushThreshold = 32 * 1024
	readBufSize    = 32 * 1024
)

// StatePayload is published on session:state:{id}.
type StatePayload struct {
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

// ClosedPayload is published on session:closed:{id}.
type ClosedPayload struct {
	ExitCode *int `json:"exitCode,omitempty"`
}

// readLoop is the sole sender and closer of live.readCh: it forwards bytes
// from the stream until Read errors (EOF or a closed stream), then closes
// the channel exactly once. Backpressure comes for free -- if the pump falls
// behind, this send blocks, which stops Read() from being called, which
// makes the underlying transport apply its own backpressure to the shell.
func (s *Service) readLoop(live *liveSession) {
	defer s.wg.Done()
	buf := make([]byte, readBufSize)
	for {
		n, err := live.getStream().Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			live.readCh <- chunk
		}
		if err != nil {
			close(live.readCh)
			return
		}
	}
}

// pump coalesces output into at-most-16ms/32KiB-sized events and is the sole
// publisher for this session's events, which guarantees per-session event
// ordering. It also owns the session's terminal shutdown sequence: once
// readCh is closed (stream ended, whether by explicit Close or the shell
// exiting on its own) it drains, flushes, waits for the exit code, and emits
// the closed/state events.
func (s *Service) pump(id string, live *liveSession) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			live.closeFileSystem()
			_ = live.getStream().Close()
			s.remove(id)
			if s.tap != nil {
				s.tap.Detach(id)
			}
			if s.middleware != nil {
				s.middleware.Detach(id)
			}
			_ = live.session.TransitionTo(domain.StateError)
			s.pub.Publish(out.TopicSessionState(id), StatePayload{State: string(domain.StateError), Error: fmt.Sprint(r)})
		}
	}()

	acc := make([]byte, 0, flushThreshold*2)
	flush := func() {
		if len(acc) == 0 {
			return
		}
		s.pub.Publish(out.TopicSessionData(id), acc)
		acc = make([]byte, 0, flushThreshold*2)
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

loop:
	for {
		select {
		case chunk, ok := <-live.readCh:
			if !ok {
				break loop
			}
			if s.middleware != nil {
				chunk = s.middleware.OnOutput(id, chunk)
				if len(chunk) == 0 {
					continue
				}
			}
			if s.tap != nil {
				s.tap.OnOutput(id, chunk)
			}
			acc = append(acc, chunk...)
			if len(acc) >= flushThreshold {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
	flush()

	exitCode, _ := live.getStream().Wait()
	live.closeFileSystem()
	_ = live.getStream().Close()
	s.remove(id)
	if s.tap != nil {
		s.tap.Detach(id)
	}
	if s.middleware != nil {
		s.middleware.Detach(id)
	}
	_ = live.session.TransitionTo(domain.StateClosed)

	s.pub.Publish(out.TopicSessionState(id), StatePayload{State: string(domain.StateClosed)})
	code := exitCode
	s.pub.Publish(out.TopicSessionClosed(id), ClosedPayload{ExitCode: &code})
}
