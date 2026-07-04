package session

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/port/in"
)

const oneSecond = time.Second

func TestMiddleware_NilLeavesOutputByteIdentical(t *testing.T) {
	// No SetMiddleware call at all -- the default nil case must behave
	// exactly like before this feature existed.
	stream := newFakeStream()
	svc, pub := newTestService(stream)

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	stream.push([]byte("hello output"))
	waitFor(t, oneSecond, func() bool { return len(dataEventsFor(pub.all(), info.ID)) > 0 })

	events := dataEventsFor(pub.all(), info.ID)
	if !bytes.Equal(events[0], []byte("hello output")) {
		t.Fatalf("expected untouched output, got %q", events[0])
	}
}

func TestMiddleware_AttachCalledOnSessionCreate(t *testing.T) {
	stream := newFakeStream()
	svc, _ := newTestService(stream)
	mw := &fakeMiddleware{}
	svc.SetMiddleware(mw)

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	waitFor(t, oneSecond, func() bool { return len(mw.attachedIDs()) > 0 })
	if ids := mw.attachedIDs(); len(ids) != 1 || ids[0] != info.ID {
		t.Fatalf("expected Attach(%s), got %v", info.ID, ids)
	}
}

func TestMiddleware_DetachCalledOnSessionClose(t *testing.T) {
	stream := newFakeStream()
	svc, _ := newTestService(stream)
	mw := &fakeMiddleware{}
	svc.SetMiddleware(mw)

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}
	if err := svc.Close(info.ID); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	waitFor(t, oneSecond, func() bool { return len(mw.detachedIDs()) > 0 })
}

func TestMiddleware_SuppressedChunkNeverReachesFrontendOrTap(t *testing.T) {
	stream := newFakeStream()
	pub := &recordingPublisher{}
	tap := &recordingTap{}
	svc := New(Deps{LocalOpener: &fakeOpener{stream: stream}, Publisher: pub, Tap: tap})

	mw := &fakeMiddleware{transform: func(sessionID string, chunk []byte) []byte { return nil }}
	svc.SetMiddleware(mw)

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	stream.push([]byte("secret protocol bytes"))
	// The middleware sees every chunk even when suppressing it -- wait for
	// that instead of the (never-arriving) publish, then assert nothing
	// downstream got it.
	waitFor(t, oneSecond, func() bool { return mw.outputCallCount() > 0 })

	if got := dataEventsFor(pub.all(), info.ID); len(got) != 0 {
		t.Fatalf("expected no session:data event for suppressed output, got %v", got)
	}
	if got := tap.outputsFor(info.ID); len(got) != 0 {
		t.Fatalf("expected CommandTap.OnOutput never called for suppressed output, got %v", got)
	}
}

func TestMiddleware_PassthroughChunkStillReachesTap(t *testing.T) {
	stream := newFakeStream()
	pub := &recordingPublisher{}
	tap := &recordingTap{}
	svc := New(Deps{LocalOpener: &fakeOpener{stream: stream}, Publisher: pub, Tap: tap})
	svc.SetMiddleware(&fakeMiddleware{}) // identity transform

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	stream.push([]byte("normal shell output"))
	waitFor(t, oneSecond, func() bool { return len(dataEventsFor(pub.all(), info.ID)) > 0 })

	if got := tap.outputsFor(info.ID); len(got) != 1 || !bytes.Equal(got[0], []byte("normal shell output")) {
		t.Fatalf("expected tap to see passthrough output, got %v", got)
	}
}

func TestMiddleware_RewrittenChunkIsWhatFrontendSees(t *testing.T) {
	stream := newFakeStream()
	svc, pub := newTestService(stream)
	svc.SetMiddleware(&fakeMiddleware{
		transform: func(sessionID string, chunk []byte) []byte {
			return bytes.ToUpper(chunk)
		},
	})

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	stream.push([]byte("lowercase"))
	waitFor(t, oneSecond, func() bool { return len(dataEventsFor(pub.all(), info.ID)) > 0 })

	events := dataEventsFor(pub.all(), info.ID)
	if !bytes.Equal(events[0], []byte("LOWERCASE")) {
		t.Fatalf("expected rewritten output, got %q", events[0])
	}
}

func TestWriteRaw_BypassesTapAndInputBlock(t *testing.T) {
	stream := newFakeStream()
	tap := &recordingTap{}
	svc := New(Deps{LocalOpener: &fakeOpener{stream: stream}, Publisher: &recordingPublisher{}, Tap: tap})

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}
	if err := svc.SetInputBlocked(info.ID, true); err != nil {
		t.Fatalf("SetInputBlocked failed: %v", err)
	}

	if err := svc.WriteRaw(info.ID, []byte("protocol-bytes")); err != nil {
		t.Fatalf("WriteRaw failed: %v", err)
	}

	written := stream.writtenBytes()
	if len(written) != 1 || !bytes.Equal(written[0], []byte("protocol-bytes")) {
		t.Fatalf("expected WriteRaw to reach the stream, got %v", written)
	}
	if got := tap.inputsFor(info.ID); len(got) != 0 {
		t.Fatalf("expected WriteRaw not to be seen by tap, got %v", got)
	}
}

func TestSetInputBlocked_DropsWriteButAllowsWriteRaw(t *testing.T) {
	stream := newFakeStream()
	svc, _ := newTestService(stream)

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}
	if err := svc.SetInputBlocked(info.ID, true); err != nil {
		t.Fatalf("SetInputBlocked failed: %v", err)
	}

	if err := svc.Write(info.ID, []byte("typed by user")); err != nil {
		t.Fatalf("expected Write to silently drop while blocked, got error: %v", err)
	}
	if len(stream.writtenBytes()) != 0 {
		t.Fatalf("expected no bytes written to stream while input blocked")
	}

	if err := svc.WriteRaw(info.ID, []byte("zmodem frame")); err != nil {
		t.Fatalf("WriteRaw failed: %v", err)
	}
	if written := stream.writtenBytes(); len(written) != 1 || !bytes.Equal(written[0], []byte("zmodem frame")) {
		t.Fatalf("expected WriteRaw to bypass the input block, got %v", written)
	}

	if err := svc.SetInputBlocked(info.ID, false); err != nil {
		t.Fatalf("SetInputBlocked(false) failed: %v", err)
	}
	if err := svc.Write(info.ID, []byte("typed again")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if written := stream.writtenBytes(); len(written) != 2 || !bytes.Equal(written[1], []byte("typed again")) {
		t.Fatalf("expected Write to resume after unblocking, got %v", written)
	}
}

func TestSetInputBlocked_UnknownSession(t *testing.T) {
	stream := newFakeStream()
	svc, _ := newTestService(stream)
	if err := svc.SetInputBlocked("nonexistent", true); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestWriteRaw_UnknownSession(t *testing.T) {
	stream := newFakeStream()
	svc, _ := newTestService(stream)
	if err := svc.WriteRaw("nonexistent", []byte("x")); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

// recordingTap is a minimal CommandTap that records every call, for
// asserting what the tap did/didn't see independent of history persistence.
type recordingTap struct {
	mu      sync.Mutex
	inputs  map[string][][]byte
	outputs map[string][][]byte
}

func (t *recordingTap) Attach(sessionID string, hostID string) {}
func (t *recordingTap) Detach(sessionID string)                {}

func (t *recordingTap) OnInput(sessionID string, data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inputs == nil {
		t.inputs = make(map[string][][]byte)
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	t.inputs[sessionID] = append(t.inputs[sessionID], cp)
}

func (t *recordingTap) OnOutput(sessionID string, data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.outputs == nil {
		t.outputs = make(map[string][][]byte)
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	t.outputs[sessionID] = append(t.outputs[sessionID], cp)
}

func (t *recordingTap) inputsFor(sessionID string) [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([][]byte{}, t.inputs[sessionID]...)
}

func (t *recordingTap) outputsFor(sessionID string) [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([][]byte{}, t.outputs[sessionID]...)
}

var _ CommandTap = (*recordingTap)(nil)
