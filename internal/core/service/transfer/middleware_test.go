package transfer

import (
	"bytes"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
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

func TestMiddleware_NoZmodemEngineDisablesDetection(t *testing.T) {
	shell := newFakeShellAccess()
	svc := New(Deps{Shell: shell}) // no Zmodem engine

	svc.Attach("s1", domain.KindSSH)
	got := svc.OnOutput("s1", downloadSignature)
	if string(got) != string(downloadSignature) {
		t.Fatalf("expected passthrough with no zmodem engine configured, got %q", got)
	}
}
