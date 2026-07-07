package session

import (
	"bytes"
	"testing"

	"momo-shell/internal/core/port/in"
)

func TestTap_FanOutToMultipleTaps(t *testing.T) {
	stream := newFakeStream()
	pub := &recordingPublisher{}
	tapA := &recordingTap{}
	tapB := &recordingTap{}
	svc := New(Deps{LocalOpener: &fakeOpener{stream: stream}, Publisher: pub, Tap: tapA})
	svc.AddTap(tapB)

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	if err := svc.Write(info.ID, []byte("ls\n")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	stream.push([]byte("shell output"))
	waitFor(t, oneSecond, func() bool { return len(dataEventsFor(pub.all(), info.ID)) > 0 })

	for name, tap := range map[string]*recordingTap{"Deps.Tap": tapA, "AddTap": tapB} {
		if got := tap.inputsFor(info.ID); len(got) != 1 || !bytes.Equal(got[0], []byte("ls\n")) {
			t.Fatalf("%s: expected to see input, got %v", name, got)
		}
		if got := tap.outputsFor(info.ID); len(got) != 1 || !bytes.Equal(got[0], []byte("shell output")) {
			t.Fatalf("%s: expected to see output, got %v", name, got)
		}
	}
}

func TestTap_NoTapsIsNoOp(t *testing.T) {
	// No Deps.Tap and no AddTap call -- the empty-slice path must behave
	// exactly like before this feature existed.
	stream := newFakeStream()
	svc, pub := newTestService(stream)

	info, err := svc.CreateLocal(in.LocalOpts{})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	stream.push([]byte("shell output"))
	waitFor(t, oneSecond, func() bool { return len(dataEventsFor(pub.all(), info.ID)) > 0 })

	events := dataEventsFor(pub.all(), info.ID)
	if !bytes.Equal(events[0], []byte("shell output")) {
		t.Fatalf("expected untouched output with no taps registered, got %q", events[0])
	}
}
