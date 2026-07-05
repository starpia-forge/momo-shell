package transfer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

func waitForZ(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func zmodemEventsFor(pub *recordingPublisher, sessionID string) []zmodemPayload {
	var out []zmodemPayload
	for _, e := range pub.all() {
		if e.topic == "transfer:zmodem:"+sessionID {
			out = append(out, e.payload.(zmodemPayload))
		}
	}
	return out
}

func lastZmodemPhase(pub *recordingPublisher, sessionID string) string {
	events := zmodemEventsFor(pub, sessionID)
	if len(events) == 0 {
		return ""
	}
	return events[len(events)-1].Phase
}

func TestMiddleware_AttachIgnoresLocalSessions(t *testing.T) {
	shell := newFakeShellAccess()
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Zmodem: engine})

	svc.Attach("local-session", domain.KindLocal)
	if got := svc.OnOutput("local-session", []byte("anything")); string(got) != "anything" {
		t.Fatalf("expected untouched passthrough for a session never attached, got %q", got)
	}
}

func TestMiddleware_PlainOutputPassesThroughUnchanged(t *testing.T) {
	shell := newFakeShellAccess()
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Zmodem: engine})

	svc.Attach("s1", domain.KindSSH)
	// The detector withholds up to maxSignatureLen-1 trailing bytes per call
	// (until more data confirms they're not a signature start), so even the
	// concatenation of every call so far is only ever a prefix of what's
	// been sent, short by at most that much.
	first := svc.OnOutput("s1", []byte("$ ls -la\r\ntotal 0\r\n"))
	second := svc.OnOutput("s1", []byte("more output after a pause\r\n"))
	got := string(first) + string(second)
	want := "$ ls -la\r\ntotal 0\r\nmore output after a pause\r\n"
	if !bytes.HasPrefix([]byte(want), []byte(got)) {
		t.Fatalf("got %q is not a prefix of %q", got, want)
	}
	if len(want)-len(got) > maxSignatureLen-1 {
		t.Fatalf("withheld too much: got %d bytes, want at least %d", len(got), len(want)-(maxSignatureLen-1))
	}
	if bytes.Contains([]byte(got), uploadSignature) || bytes.Contains([]byte(got), downloadSignature) {
		t.Fatalf("passthrough must never contain a signature, got %q", got)
	}
}

func TestMiddleware_DetectsDownloadAndStartsReceive(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{} // default: blocks on ctx, which is all this test needs
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	// Real download-signature bytes, as captured from sz in the zmodem package's interop tests.
	chunk := append([]byte("$ sz file.bin\r\n"), downloadSignature...)
	chunk = append(chunk, []byte("112233")...)

	got := svc.OnOutput("s1", chunk)
	if string(got) != "$ sz file.bin\r\n" {
		t.Fatalf("expected only pre-signature bytes passed through, got %q", got)
	}

	waitForZ(t, time.Second, func() bool { return engine.recvCallCount() > 0 })
	waitForZ(t, time.Second, func() bool { return shell.isInputBlocked("s1") })
	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") != "" })

	events := zmodemEventsFor(pub, "s1")
	if events[0].Direction != "download" || events[0].Phase != "active" {
		t.Fatalf("expected first event to be download/active, got %+v", events[0])
	}
}

func TestMiddleware_DetectsUploadAndWaitsForStartZmodemSend(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	chunk := append([]byte("$ rz\r\nrz waiting to receive."), uploadSignature...)
	chunk = append(chunk, []byte("000000000000")...)

	got := svc.OnOutput("s1", chunk)
	if string(got) != "$ rz\r\nrz waiting to receive." {
		t.Fatalf("expected only pre-signature bytes passed through, got %q", got)
	}

	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "detected" })
	if engine.sendCallCount() != 0 {
		t.Fatalf("expected Send not to start until StartZmodemSend, got %d calls", engine.sendCallCount())
	}
	if shell.isInputBlocked("s1") {
		t.Fatalf("input should not be blocked before StartZmodemSend")
	}

	taskID, err := svc.StartZmodemSend("s1", []string{"/tmp/a.txt"})
	if err != nil {
		t.Fatalf("StartZmodemSend failed: %v", err)
	}
	if taskID == "" {
		t.Fatalf("expected a non-empty task ID")
	}

	waitForZ(t, time.Second, func() bool { return engine.sendCallCount() > 0 })
	waitForZ(t, time.Second, func() bool { return shell.isInputBlocked("s1") })
}

func TestMiddleware_StartZmodemSendWithoutDetectionFails(t *testing.T) {
	shell := newFakeShellAccess()
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	if _, err := svc.StartZmodemSend("s1", []string{"/tmp/a.txt"}); err != ErrNoZmodemPending {
		t.Fatalf("expected ErrNoZmodemPending, got %v", err)
	}
}

func TestMiddleware_StartZmodemSendUnknownSession(t *testing.T) {
	shell := newFakeShellAccess()
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Zmodem: engine})

	if _, err := svc.StartZmodemSend("nonexistent", nil); err != ErrNoZmodemSession {
		t.Fatalf("expected ErrNoZmodemSession, got %v", err)
	}
}

func TestMiddleware_ActiveTransferSuppressesAllOutput(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{} // default: blocks on ctx
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	chunk := append([]byte{}, downloadSignature...)
	svc.OnOutput("s1", chunk)
	waitForZ(t, time.Second, func() bool { return engine.recvCallCount() > 0 })

	got := svc.OnOutput("s1", []byte("more zmodem protocol bytes"))
	if len(got) != 0 {
		t.Fatalf("expected full suppression while active, got %q", got)
	}
}

func TestMiddleware_CancelZmodemStopsActiveTransfer(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return engine.recvCallCount() > 0 })

	if err := svc.CancelZmodem("s1"); err != nil {
		t.Fatalf("CancelZmodem failed: %v", err)
	}

	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "canceled" })
	waitForZ(t, time.Second, func() bool { return !shell.isInputBlocked("s1") })
}

func TestMiddleware_CancelZmodemSendsCancelBytesToRemote(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return engine.recvCallCount() > 0 })

	if err := svc.CancelZmodem("s1"); err != nil {
		t.Fatalf("CancelZmodem failed: %v", err)
	}

	found := false
	for _, w := range shell.rawWrittenTo("s1") {
		if bytes.Equal(w, engine.CancelBytes()) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected CancelZmodem to write the engine's cancel sequence to the remote, got %v", shell.rawWrittenTo("s1"))
	}
}

func TestMiddleware_FailedTransferSendsCancelBytesAndDrainsBeforeUnblocking(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{
		receiveFunc: func(ctx context.Context, rw io.ReadWriter, destDir string, progress func(out.TransferProgress)) ([]string, error) {
			return nil, errors.New("boom")
		},
	}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "failed" })

	found := false
	for _, w := range shell.rawWrittenTo("s1") {
		if bytes.Equal(w, engine.CancelBytes()) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a protocol failure to send the cancel sequence, got %v", shell.rawWrittenTo("s1"))
	}

	// Right after failing, trailing bytes from the remote must still be
	// swallowed (draining), not rendered to the terminal.
	if got := svc.OnOutput("s1", []byte("leftover zmodem noise")); len(got) != 0 {
		t.Fatalf("expected drain to suppress trailing output right after failure, got %q", got)
	}
	if !shell.isInputBlocked("s1") {
		t.Fatalf("expected input to stay blocked during the drain grace period")
	}

	waitForZ(t, 2*time.Second, func() bool { return !shell.isInputBlocked("s1") })

	// Once drained, output detection resumes normally (modulo the
	// detector's own trailing hold-back -- see TestMiddleware_
	// PlainOutputPassesThroughUnchanged).
	want := "$ normal prompt\r\n"
	got := string(svc.OnOutput("s1", []byte(want)))
	if !bytes.HasPrefix([]byte(want), []byte(got)) || len(want)-len(got) > maxSignatureLen-1 {
		t.Fatalf("expected normal (near-full) passthrough after drain ends, got %q want prefix of %q", got, want)
	}
}

func TestMiddleware_CancelDuringDrainEndsItImmediately(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{
		receiveFunc: func(ctx context.Context, rw io.ReadWriter, destDir string, progress func(out.TransferProgress)) ([]string, error) {
			return nil, nil // completes successfully and immediately, straight into drain
		},
	}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "done" })
	waitForZ(t, time.Second, func() bool { return shell.isInputBlocked("s1") }) // now draining

	if err := svc.CancelZmodem("s1"); err != nil {
		t.Fatalf("CancelZmodem failed: %v", err)
	}
	if shell.isInputBlocked("s1") {
		t.Fatalf("expected canceling during drain to unblock input immediately")
	}
}

func TestMiddleware_CancelPendingUploadPrompt(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", uploadSignature)
	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "detected" })

	if err := svc.CancelZmodem("s1"); err != nil {
		t.Fatalf("CancelZmodem failed: %v", err)
	}
	if _, err := svc.StartZmodemSend("s1", nil); err != ErrNoZmodemPending {
		t.Fatalf("expected the pending prompt to be cleared, got err=%v", err)
	}
}

func TestMiddleware_DetachCancelsActiveTransfer(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return engine.recvCallCount() > 0 })

	svc.Detach("s1")

	// After Detach, the session's zmodemState is gone -- OnOutput must
	// treat it as never-attached (passthrough), not panic on a nil state.
	got := svc.OnOutput("s1", []byte("post-detach output"))
	if string(got) != "post-detach output" {
		t.Fatalf("expected passthrough after Detach, got %q", got)
	}
}

// withShrunkStallTimeout shrinks transferStallTimeout/transferStallPollInterval
// for the duration of a test so watchdog tests don't wait the full 120s.
func withShrunkStallTimeout(t *testing.T, timeout, poll time.Duration) {
	t.Helper()
	origTimeout, origPoll := transferStallTimeout, transferStallPollInterval
	transferStallTimeout, transferStallPollInterval = timeout, poll
	t.Cleanup(func() { transferStallTimeout, transferStallPollInterval = origTimeout, origPoll })
}

// TestMiddleware_StallWatchdogForcesRecoveryWhenEngineNeverReturns is the
// direct regression test for FT-12's permanent terminal freeze: an engine
// that never returns from Receive (ignoring ctx entirely, simulating a
// truly stuck goroutine -- e.g. blocked on a write with no deadline) must
// still have the session's input unblocked once the stall watchdog's
// timeout elapses, instead of leaving the terminal stuck forever.
func TestMiddleware_StallWatchdogForcesRecoveryWhenEngineNeverReturns(t *testing.T) {
	withShrunkStallTimeout(t, 30*time.Millisecond, 5*time.Millisecond)

	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{
		receiveFunc: func(ctx context.Context, rw io.ReadWriter, destDir string, progress func(out.TransferProgress)) ([]string, error) {
			select {} // never returns, and never even looks at ctx
		},
	}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return engine.recvCallCount() > 0 })
	waitForZ(t, time.Second, func() bool { return shell.isInputBlocked("s1") })

	waitForZ(t, 2*time.Second, func() bool { return !shell.isInputBlocked("s1") })
	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "failed" })

	found := false
	for _, w := range shell.rawWrittenTo("s1") {
		if bytes.Equal(w, engine.CancelBytes()) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the stall watchdog to send the cancel sequence, got %v", shell.rawWrittenTo("s1"))
	}
}

// TestMiddleware_StallWatchdogIgnoresACompletedTransfer proves the watchdog
// doesn't fire (and doesn't clobber a subsequent transfer's state) once a
// transfer has already finished normally before the stall timeout elapses.
func TestMiddleware_StallWatchdogIgnoresACompletedTransfer(t *testing.T) {
	withShrunkStallTimeout(t, 50*time.Millisecond, 5*time.Millisecond)

	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{
		receiveFunc: func(ctx context.Context, rw io.ReadWriter, destDir string, progress func(out.TransferProgress)) ([]string, error) {
			return nil, nil // completes immediately
		},
	}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "done" })
	waitForZ(t, time.Second, func() bool { return !shell.isInputBlocked("s1") }) // drain ends fast (real drainQuietPeriod)

	// Give the (superseded) watchdog time to have fired if it were going to.
	time.Sleep(150 * time.Millisecond)

	events := zmodemEventsFor(pub, "s1")
	for _, e := range events {
		if e.Phase == "failed" {
			t.Fatalf("watchdog fired on an already-completed transfer: %+v", events)
		}
	}
}

// TestMiddleware_DrainScansForANewSignature proves a new transfer starting
// right on the heels of the last one's trailing bytes is still detected
// while draining, rather than being silently discarded (the previous
// behavior: draining unconditionally discarded output without scanning it).
func TestMiddleware_DrainScansForANewSignature(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	engine := &fakeZmodemEngine{
		receiveFunc: func(ctx context.Context, rw io.ReadWriter, destDir string, progress func(out.TransferProgress)) ([]string, error) {
			return nil, nil // completes immediately, straight into drain
		},
	}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "done" })
	waitForZ(t, time.Second, func() bool { return shell.isInputBlocked("s1") }) // now draining

	// A fresh upload signature arrives inside the drain's grace window.
	got := svc.OnOutput("s1", uploadSignature)
	if len(got) != 0 {
		t.Fatalf("expected the signature bytes themselves to still be suppressed, got %q", got)
	}

	waitForZ(t, time.Second, func() bool { return lastZmodemPhase(pub, "s1") == "detected" })
	events := zmodemEventsFor(pub, "s1")
	last := events[len(events)-1]
	if last.Direction != "upload" || last.Phase != "detected" {
		t.Fatalf("expected upload/detected to follow the drained download, got %+v", last)
	}
}

// TestMiddleware_CancelDoesNotDeadlockBehindABlockedPush is the regression
// test for a real deadlock found via live E2E testing (a 5MB download stalled
// at 1% and never recovered): OnOutput used to call conduit.push while
// holding st.mu. push blocks once the conduit's channel (32 slots) is full
// until the engine drains it via Read (or conduit.Close is called) -- but
// Close is only reachable through CancelZmodem or the stall watchdog, both of
// which themselves need st.mu. An engine that is merely slow to Read (not
// stuck, just busy) could wedge both of those recovery paths behind a lock a
// blocked push() call inside OnOutput never released.
func TestMiddleware_CancelDoesNotDeadlockBehindABlockedPush(t *testing.T) {
	shell := newFakeShellAccess()
	pub := &recordingPublisher{}
	blockReceive := make(chan struct{})
	engine := &fakeZmodemEngine{
		receiveFunc: func(ctx context.Context, rw io.ReadWriter, destDir string, progress func(out.TransferProgress)) ([]string, error) {
			<-blockReceive // never reads from rw until the test says so
			return nil, nil
		},
	}
	svc := New(Deps{Shell: shell, Pub: pub, Zmodem: engine})
	defer close(blockReceive)
	svc.Attach("s1", domain.KindSSH)

	svc.OnOutput("s1", downloadSignature)
	waitForZ(t, time.Second, func() bool { return engine.recvCallCount() > 0 })

	// The detection itself already pushed the signature bytes as the conduit's
	// first queued chunk, so fill the remaining capacity (channel cap is 32).
	for i := 0; i < 31; i++ {
		svc.OnOutput("s1", []byte("x"))
	}

	// One more chunk blocks inside push -- run it in the background so a
	// regression doesn't hang the test itself.
	go svc.OnOutput("s1", []byte("y"))
	time.Sleep(50 * time.Millisecond) // let it reach push and block on the full channel

	done := make(chan error, 1)
	go func() { done <- svc.CancelZmodem("s1") }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("CancelZmodem: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CancelZmodem deadlocked behind a blocked push() holding st.mu")
	}
}

func TestMiddleware_NoZmodemEngineDisablesDetection(t *testing.T) {
	shell := newFakeShellAccess()
	svc := New(Deps{Shell: shell}) // no Zmodem engine

	svc.Attach("s1", domain.KindSSH)
	got := svc.OnOutput("s1", downloadSignature)
	if string(got) != string(downloadSignature) {
		t.Fatalf("expected passthrough with no zmodem engine configured, got %q", got)
	}
}
