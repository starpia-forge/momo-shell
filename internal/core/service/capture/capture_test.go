package capture

import (
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/service/session"
)

var _ session.CommandTap = (*Service)(nil)

// fakeSink is a hand-rolled OutputSink (house style: no mock framework --
// see scrollback's sibling test conventions) that records every
// AttachOutput call and signals a channel, so tests can synchronize on
// seal's finalize goroutine without an arbitrary sleep.
type fakeSink struct {
	mu    sync.Mutex
	calls []sealCall
	ch    chan sealCall
}

type sealCall struct {
	auditID int64
	output  []byte
}

func newFakeSink() *fakeSink {
	return &fakeSink{ch: make(chan sealCall, 16)}
}

func (f *fakeSink) AttachOutput(auditID int64, output []byte) error {
	call := sealCall{auditID: auditID, output: append([]byte(nil), output...)}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()
	f.ch <- call
	return nil
}

func (f *fakeSink) waitForSeal(t *testing.T) sealCall {
	t.Helper()
	select {
	case c := <-f.ch:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for AttachOutput")
		return sealCall{}
	}
}

// TestCapture_BeginOutputEndSeals is the base case: End flags the capture,
// and the seal only happens on the NEXT OnOutput call (the package doc's
// central invariant -- in production this is the same pump iteration's
// already-inbound tap-phase call for the chunk that triggered End).
func TestCapture_BeginOutputEndSeals(t *testing.T) {
	sink := newFakeSink()
	s := New(sink)

	s.Begin("sess-1", 42)
	s.OnOutput("sess-1", []byte("hello "))
	s.OnOutput("sess-1", []byte("world"))
	s.End("sess-1")
	s.OnOutput("sess-1", []byte("!")) // the deferred seal trigger

	got := sink.waitForSeal(t)
	if got.auditID != 42 {
		t.Errorf("auditID = %d, want 42", got.auditID)
	}
	if string(got.output) != "hello world!" {
		t.Errorf("output = %q, want %q", got.output, "hello world!")
	}
}

// TestCapture_EndWithNoFurtherOutput_SealsOnDetach covers a fully-suppressed
// end-marker chunk (pump.go skips taps entirely when a middleware strips a
// chunk to zero bytes): End is flagged but no further OnOutput ever comes
// before the session closes. Detach is the catch-all that still seals it.
func TestCapture_EndWithNoFurtherOutput_SealsOnDetach(t *testing.T) {
	sink := newFakeSink()
	s := New(sink)

	s.Begin("sess-1", 7)
	s.OnOutput("sess-1", []byte("output"))
	s.End("sess-1")
	s.Detach("sess-1")

	got := sink.waitForSeal(t)
	if got.auditID != 7 || string(got.output) != "output" {
		t.Errorf("seal = %+v, want {auditID:7, output:\"output\"}", got)
	}
}

// TestCapture_HeadCapTruncatesAndMarks confirms the byte cap keeps the
// earliest bytes (head-first) and appends a truncation marker on seal
// rather than silently dropping the tail.
func TestCapture_HeadCapTruncatesAndMarks(t *testing.T) {
	sink := newFakeSink()
	s := NewWithCapacity(sink, 10)

	s.Begin("sess-1", 1)
	s.OnOutput("sess-1", []byte("0123456789")) // exactly at cap
	s.OnOutput("sess-1", []byte("overflow"))    // dropped -- marks truncated
	s.End("sess-1")
	s.OnOutput("sess-1", []byte("more")) // still dropped; triggers the seal

	got := sink.waitForSeal(t)
	if got.auditID != 1 {
		t.Errorf("auditID = %d, want 1", got.auditID)
	}
	want := "0123456789\n...[output truncated at 10 bytes]"
	if string(got.output) != want {
		t.Errorf("output = %q, want %q", got.output, want)
	}
}

// TestCapture_Begin_SealsPriorUnsealedCapture covers overlapping commands:
// aicontrol doesn't strictly enforce one in-flight command per session, so
// a second RunCommand can arm before the first's OnCommandEnd lands. Begin
// must seal whatever was buffered for the PRIOR auditID before installing
// the new one, so output never cross-links to the wrong command.
func TestCapture_Begin_SealsPriorUnsealedCapture(t *testing.T) {
	sink := newFakeSink()
	s := New(sink)

	s.Begin("sess-1", 1)
	s.OnOutput("sess-1", []byte("first command output"))
	s.Begin("sess-1", 2) // supersedes before command 1 ever called End

	got := sink.waitForSeal(t)
	if got.auditID != 1 || string(got.output) != "first command output" {
		t.Errorf("superseded seal = %+v, want {auditID:1, output:\"first command output\"}", got)
	}

	s.OnOutput("sess-1", []byte("second command output"))
	s.End("sess-1")
	s.OnOutput("sess-1", nil)

	got2 := sink.waitForSeal(t)
	if got2.auditID != 2 || string(got2.output) != "second command output" {
		t.Errorf("second seal = %+v, want {auditID:2, output:\"second command output\"}", got2)
	}
}

// TestCapture_Detach_SealsCaptureMidCommand covers the session closing
// before the in-flight command ever reaches OnCommandEnd.
func TestCapture_Detach_SealsCaptureMidCommand(t *testing.T) {
	sink := newFakeSink()
	s := New(sink)

	s.Begin("sess-1", 9)
	s.OnOutput("sess-1", []byte("partial output"))
	s.Detach("sess-1")

	got := sink.waitForSeal(t)
	if got.auditID != 9 || string(got.output) != "partial output" {
		t.Errorf("seal = %+v, want {auditID:9, output:\"partial output\"}", got)
	}
}

// TestCapture_OnOutput_WithoutBeginIsIgnored mirrors scrollback's
// before-Attach guard: a session with no active capture (never Begin'd, or
// already sealed) produces no seal call.
func TestCapture_OnOutput_WithoutBeginIsIgnored(t *testing.T) {
	sink := newFakeSink()
	s := New(sink)

	s.OnOutput("never-begun", []byte("ignored"))
	s.End("never-begun")
	s.Detach("never-begun")

	select {
	case call := <-sink.ch:
		t.Fatalf("unexpected seal call: %+v", call)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestCapture_OnInput_NeverAppearsInCapture mirrors scrollback.Service's
// OnInput contract: capture only needs command output.
func TestCapture_OnInput_NeverAppearsInCapture(t *testing.T) {
	sink := newFakeSink()
	s := New(sink)

	s.Begin("sess-1", 1)
	s.OnInput("sess-1", []byte("typed input"))
	s.OnOutput("sess-1", []byte("output only"))
	s.End("sess-1")
	s.OnOutput("sess-1", nil)

	got := sink.waitForSeal(t)
	if string(got.output) != "output only" {
		t.Errorf("output = %q, want %q (OnInput bytes must never appear)", got.output, "output only")
	}
}
