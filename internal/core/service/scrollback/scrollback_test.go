package scrollback

import (
	"bytes"
	"testing"

	"momo-shell/internal/core/service/session"
)

var _ session.CommandTap = (*Service)(nil)

func TestOnOutput_ReadSinceZeroReturnsAllCapturedBytes(t *testing.T) {
	s := New()
	s.Attach("sess-1", "")
	s.OnOutput("sess-1", []byte("hello "))
	s.OnOutput("sess-1", []byte("world"))

	data, nextSeq := s.ReadSince("sess-1", 0)

	if string(data) != "hello world" {
		t.Errorf("data = %q, want %q", data, "hello world")
	}
	if nextSeq != 2 {
		t.Errorf("nextSeq = %d, want 2", nextSeq)
	}
}

func TestReadSince_MidSeqReturnsOnlyLaterChunks(t *testing.T) {
	s := New()
	s.Attach("sess-1", "")
	s.OnOutput("sess-1", []byte("first"))
	_, seqAfterFirst := s.ReadSince("sess-1", 0)
	s.OnOutput("sess-1", []byte("second"))

	data, nextSeq := s.ReadSince("sess-1", seqAfterFirst)

	if string(data) != "second" {
		t.Errorf("data = %q, want %q", data, "second")
	}
	if nextSeq != 2 {
		t.Errorf("nextSeq = %d, want 2", nextSeq)
	}
}

func TestReadSince_NoNewDataReturnsNilAndUnchangedSeq(t *testing.T) {
	s := New()
	s.Attach("sess-1", "")
	s.OnOutput("sess-1", []byte("only"))
	_, seq := s.ReadSince("sess-1", 0)

	data, nextSeq := s.ReadSince("sess-1", seq)

	if data != nil {
		t.Errorf("data = %q, want nil", data)
	}
	if nextSeq != seq {
		t.Errorf("nextSeq = %d, want unchanged %d", nextSeq, seq)
	}
}

func TestOnOutput_EvictsOldestChunksOverCapacity(t *testing.T) {
	s := NewWithCapacity(10)
	s.Attach("sess-1", "")
	s.OnOutput("sess-1", []byte("0123456789")) // exactly at capacity
	s.OnOutput("sess-1", []byte("abc"))        // pushes total to 13 > 10 -- must evict

	data, _ := s.ReadSince("sess-1", 0)

	if bytes.Contains(data, []byte("0123456789")) {
		t.Errorf("data = %q, want the oldest chunk evicted", data)
	}
	if !bytes.Contains(data, []byte("abc")) {
		t.Errorf("data = %q, want the newest chunk retained", data)
	}
}

func TestOnOutput_SingleChunkOverCapacityIsNeverDropped(t *testing.T) {
	s := NewWithCapacity(4)
	s.Attach("sess-1", "")
	s.OnOutput("sess-1", []byte("this-is-way-over-capacity"))

	data, _ := s.ReadSince("sess-1", 0)

	if string(data) != "this-is-way-over-capacity" {
		t.Errorf("data = %q, want the single oversized chunk retained", data)
	}
}

func TestOnOutput_SessionsAreIsolated(t *testing.T) {
	s := New()
	s.Attach("sess-1", "")
	s.Attach("sess-2", "")
	s.OnOutput("sess-1", []byte("one"))
	s.OnOutput("sess-2", []byte("two"))

	data1, _ := s.ReadSince("sess-1", 0)
	data2, _ := s.ReadSince("sess-2", 0)

	if string(data1) != "one" {
		t.Errorf("sess-1 data = %q, want %q", data1, "one")
	}
	if string(data2) != "two" {
		t.Errorf("sess-2 data = %q, want %q", data2, "two")
	}
}

func TestHostID_ReportsAttachedHostOrLocalOrUnknown(t *testing.T) {
	s := New()
	s.Attach("remote-sess", "host-42")
	s.Attach("local-sess", "")

	if hostID, ok := s.HostID("remote-sess"); !ok || hostID != "host-42" {
		t.Errorf("HostID(remote-sess) = (%q, %v), want (host-42, true)", hostID, ok)
	}
	if hostID, ok := s.HostID("local-sess"); !ok || hostID != "" {
		t.Errorf("HostID(local-sess) = (%q, %v), want (\"\", true)", hostID, ok)
	}
	if hostID, ok := s.HostID("unknown-sess"); ok || hostID != "" {
		t.Errorf("HostID(unknown-sess) = (%q, %v), want (\"\", false)", hostID, ok)
	}
}

func TestDetach_ClearsBufferAndHostID(t *testing.T) {
	s := New()
	s.Attach("sess-1", "host-1")
	s.OnOutput("sess-1", []byte("data"))

	s.Detach("sess-1")

	data, nextSeq := s.ReadSince("sess-1", 0)
	if data != nil || nextSeq != 0 {
		t.Errorf("ReadSince after Detach = (%q, %d), want (nil, 0)", data, nextSeq)
	}
	if hostID, ok := s.HostID("sess-1"); ok || hostID != "" {
		t.Errorf("HostID after Detach = (%q, %v), want (\"\", false)", hostID, ok)
	}
}

func TestOnInput_NeverAppearsInScrollback(t *testing.T) {
	s := New()
	s.Attach("sess-1", "")
	s.OnInput("sess-1", []byte("typed input"))
	s.OnOutput("sess-1", []byte("output only"))

	data, _ := s.ReadSince("sess-1", 0)

	if bytes.Contains(data, []byte("typed input")) {
		t.Errorf("data = %q, want no trace of OnInput bytes", data)
	}
}

func TestOnOutput_BeforeAttachIsIgnoredWithoutPanic(t *testing.T) {
	s := New()

	s.OnOutput("never-attached", []byte("ignored"))

	data, nextSeq := s.ReadSince("never-attached", 0)
	if data != nil || nextSeq != 0 {
		t.Errorf("ReadSince(never-attached) = (%q, %d), want (nil, 0)", data, nextSeq)
	}
}

func TestOnOutput_DefensiveCopyProtectsAgainstCallerMutation(t *testing.T) {
	s := New()
	s.Attach("sess-1", "")
	buf := []byte("original")
	s.OnOutput("sess-1", buf)

	for i := range buf {
		buf[i] = 'X'
	}

	data, _ := s.ReadSince("sess-1", 0)
	if string(data) != "original" {
		t.Errorf("data = %q, want %q (unaffected by caller mutation)", data, "original")
	}
}
