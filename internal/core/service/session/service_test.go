package session

import (
	"bytes"
	"testing"
	"time"

	"momo-shell/internal/core/port/in"
)

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition not met within %s", timeout)
	}
}

func newTestService(stream *fakeStream) (*Service, *recordingPublisher) {
	pub := &recordingPublisher{}
	svc := New(Deps{LocalOpener: &fakeOpener{stream: stream}, Publisher: pub})
	return svc, pub
}

func dataEventsFor(events []recordedEvent, id string) [][]byte {
	var out [][]byte
	for _, e := range events {
		if e.topic == "session:data:"+id {
			out = append(out, e.payload.([]byte))
		}
	}
	return out
}

func stateEventsFor(events []recordedEvent, id string) []StatePayload {
	var out []StatePayload
	for _, e := range events {
		if e.topic == "session:state:"+id {
			out = append(out, e.payload.(StatePayload))
		}
	}
	return out
}

func closedEventFor(events []recordedEvent, id string) (ClosedPayload, bool) {
	for _, e := range events {
		if e.topic == "session:closed:"+id {
			return e.payload.(ClosedPayload), true
		}
	}
	return ClosedPayload{}, false
}

func TestCreateLocal_PublishesRunningStateAndReturnsInfo(t *testing.T) {
	stream := newFakeStream()
	svc, pub := newTestService(stream)
	defer svc.CloseAll()

	info, err := svc.CreateLocal(in.LocalOpts{Shell: "bash", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}
	if info.ID == "" {
		t.Fatal("expected a non-empty session ID")
	}
	if info.Shell != "bash" || info.Cols != 80 || info.Rows != 24 {
		t.Fatalf("unexpected SessionInfo: %+v", info)
	}

	waitFor(t, time.Second, func() bool {
		return len(stateEventsFor(pub.all(), info.ID)) > 0
	})
	states := stateEventsFor(pub.all(), info.ID)
	if states[0].State != "running" {
		t.Fatalf("expected first state event to be running, got %+v", states[0])
	}
}

func TestCreateLocal_OpenerError(t *testing.T) {
	pub := &recordingPublisher{}
	svc := New(Deps{LocalOpener: &fakeOpener{err: errBoom}, Publisher: pub})

	_, err := svc.CreateLocal(in.LocalOpts{})
	if err == nil {
		t.Fatal("expected an error when the opener fails")
	}
}

func TestWrite_ForwardsToStream(t *testing.T) {
	stream := newFakeStream()
	svc, _ := newTestService(stream)
	defer svc.CloseAll()

	info, _ := svc.CreateLocal(in.LocalOpts{})

	if err := svc.Write(info.ID, []byte("ls\n")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	waitFor(t, time.Second, func() bool { return len(stream.writtenBytes()) > 0 })
	writes := stream.writtenBytes()
	if !bytes.Equal(writes[0], []byte("ls\n")) {
		t.Fatalf("unexpected write: %q", writes[0])
	}
}

func TestWrite_UnknownSession(t *testing.T) {
	svc, _ := newTestService(newFakeStream())
	if err := svc.Write("nope", []byte("x")); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestResize_ForwardsToStream(t *testing.T) {
	stream := newFakeStream()
	svc, _ := newTestService(stream)
	defer svc.CloseAll()

	info, _ := svc.CreateLocal(in.LocalOpts{})

	if err := svc.Resize(info.ID, 120, 40); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}

	waitFor(t, time.Second, func() bool { return len(stream.resizeCalls()) > 0 })
	calls := stream.resizeCalls()
	if calls[0] != [2]int{120, 40} {
		t.Fatalf("unexpected resize call: %+v", calls[0])
	}
}

func TestOutputCoalescing_MergesRapidChunks(t *testing.T) {
	stream := newFakeStream()
	svc, pub := newTestService(stream)
	defer svc.CloseAll()

	info, _ := svc.CreateLocal(in.LocalOpts{})

	var want []byte
	for i := 0; i < 200; i++ {
		chunk := []byte("x")
		want = append(want, chunk...)
		stream.push(chunk)
	}

	waitFor(t, time.Second, func() bool {
		var total int
		for _, d := range dataEventsFor(pub.all(), info.ID) {
			total += len(d)
		}
		return total == len(want)
	})

	events := dataEventsFor(pub.all(), info.ID)
	if len(events) >= 200 {
		t.Fatalf("expected coalescing to merge 200 rapid chunks into far fewer events, got %d", len(events))
	}

	var got []byte
	for _, d := range events {
		got = append(got, d...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("coalesced output mismatch: got %q want %q", got, want)
	}
}

func TestOutputCoalescing_FlushesAt32KiBThreshold(t *testing.T) {
	stream := newFakeStream()
	svc, pub := newTestService(stream)
	defer svc.CloseAll()

	info, _ := svc.CreateLocal(in.LocalOpts{})

	// Larger than one read buffer (readBufSize == flushThreshold == 32KiB),
	// so the reader delivers it as 32768 bytes then a 8192-byte remainder --
	// same as a real PTY/ConPTY read would chunk it.
	big := bytes.Repeat([]byte("a"), 40*1024)
	stream.push(big)

	// The first 32KiB should flush immediately (threshold-triggered),
	// well before we'd otherwise need to rely on the 16ms ticker.
	waitFor(t, 200*time.Millisecond, func() bool {
		return len(dataEventsFor(pub.all(), info.ID)) > 0
	})
	first := dataEventsFor(pub.all(), info.ID)[0]
	if len(first) != flushThreshold {
		t.Fatalf("expected first threshold-triggered flush of exactly %d bytes, got %d", flushThreshold, len(first))
	}

	// The remainder (8KiB) arrives via the 16ms ticker; total must match with no data lost.
	waitFor(t, 200*time.Millisecond, func() bool {
		var total int
		for _, d := range dataEventsFor(pub.all(), info.ID) {
			total += len(d)
		}
		return total == len(big)
	})
}

func TestClose_EmitsClosedEventWithExitCodeAndRemovesSession(t *testing.T) {
	stream := newFakeStream()
	svc, pub := newTestService(stream)

	info, _ := svc.CreateLocal(in.LocalOpts{})

	if err := svc.Close(info.ID); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		_, ok := closedEventFor(pub.all(), info.ID)
		return ok
	})

	states := stateEventsFor(pub.all(), info.ID)
	if states[len(states)-1].State != "closed" {
		t.Fatalf("expected final state to be closed, got %+v", states[len(states)-1])
	}

	if err := svc.Write(info.ID, []byte("x")); err != ErrSessionNotFound {
		t.Fatalf("expected session to be removed after close, got %v", err)
	}
}

func TestShellExitsOnItsOwn_ReportsRealExitCode(t *testing.T) {
	stream := newFakeStream()
	svc, pub := newTestService(stream)

	info, _ := svc.CreateLocal(in.LocalOpts{})

	stream.simulateProcessExit(7)

	waitFor(t, time.Second, func() bool {
		_, ok := closedEventFor(pub.all(), info.ID)
		return ok
	})

	closed, _ := closedEventFor(pub.all(), info.ID)
	if closed.ExitCode == nil || *closed.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %+v", closed)
	}
}

func TestClose_IsIdempotentAfterSessionRemoved(t *testing.T) {
	stream := newFakeStream()
	svc, pub := newTestService(stream)

	info, _ := svc.CreateLocal(in.LocalOpts{})

	if err := svc.Close(info.ID); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	waitFor(t, time.Second, func() bool {
		_, ok := closedEventFor(pub.all(), info.ID)
		return ok
	})

	if err := svc.Close(info.ID); err != ErrSessionNotFound {
		t.Fatalf("expected second Close to report ErrSessionNotFound, got %v", err)
	}
}

func TestPumpPanic_RecoversAndPublishesErrorState(t *testing.T) {
	stream := newFakeStream()
	pub := &recordingPublisher{panicOnPrefix: "session:data:"}
	svc := New(Deps{LocalOpener: &fakeOpener{stream: stream}, Publisher: pub})

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	stream.push([]byte("boom"))

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "error"
	})

	if err := svc.Write(info.ID, []byte("x")); err != ErrSessionNotFound {
		t.Fatalf("expected session to be cleaned up after panic recovery, got %v", err)
	}
}

func TestCloseAll_DrainsEverySession(t *testing.T) {
	stream1 := newFakeStream()
	stream2 := newFakeStream()
	pub := &recordingPublisher{}

	callCount := 0
	streams := []*fakeStream{stream1, stream2}
	svc := New(Deps{
		LocalOpener: openerFunc(func() (*fakeStream, error) {
			s := streams[callCount]
			callCount++
			return s, nil
		}),
		Publisher: pub,
	})

	info1, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal 1 failed: %v", err)
	}
	info2, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal 2 failed: %v", err)
	}

	svc.CloseAll()

	for _, id := range []string{info1.ID, info2.ID} {
		if _, ok := closedEventFor(pub.all(), id); !ok {
			t.Fatalf("expected closed event for session %s", id)
		}
	}
	if err := svc.Write(info1.ID, []byte("x")); err != ErrSessionNotFound {
		t.Fatalf("expected session 1 to be removed, got %v", err)
	}
	if err := svc.Write(info2.ID, []byte("x")); err != ErrSessionNotFound {
		t.Fatalf("expected session 2 to be removed, got %v", err)
	}
}
