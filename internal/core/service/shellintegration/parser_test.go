package shellintegration

import (
	"bytes"
	"testing"
)

func TestParser_PromptCommandExitMarkers(t *testing.T) {
	var p parser
	input := []byte("before\x1b]133;A\x07mid\x1b]133;C\x07more\x1b]133;D;0\x07after")

	rendered, events := p.feed(input)

	if got, want := string(rendered), "beforemidmoreafter"; got != want {
		t.Fatalf("rendered = %q, want %q", got, want)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %+v", len(events), events)
	}
	if events[0].kind != evPromptStart {
		t.Errorf("event 0 kind = %v, want evPromptStart", events[0].kind)
	}
	if events[1].kind != evCommandStart {
		t.Errorf("event 1 kind = %v, want evCommandStart", events[1].kind)
	}
	if events[2].kind != evCommandDone || events[2].exitCode != 0 {
		t.Errorf("event 2 = %+v, want evCommandDone exitCode=0", events[2])
	}
}

func TestParser_ExitCodeNonZero(t *testing.T) {
	var p parser
	_, events := p.feed([]byte("\x1b]133;D;127\x07"))
	if len(events) != 1 || events[0].kind != evCommandDone || events[0].exitCode != 127 {
		t.Fatalf("expected evCommandDone exitCode=127, got %+v", events)
	}
}

func TestParser_MalformedExitCodeNotRecognized(t *testing.T) {
	var p parser
	// "133;D;" with no code (can happen if $LASTEXITCODE is $null upstream --
	// this is exactly the PowerShell bug caught during the S4 spike, doc 19 §3.2).
	rendered, events := p.feed([]byte("\x1b]133;D;\x07"))
	if len(events) != 0 {
		t.Fatalf("expected no events for a malformed exit-code marker, got %+v", events)
	}
	if !bytes.Contains(rendered, []byte("133;D;")) {
		t.Fatalf("expected the unrecognized marker to pass through unchanged, got %q", rendered)
	}
}

func TestParser_HookInstalledMarker(t *testing.T) {
	var p parser
	rendered, events := p.feed([]byte("\x1b]1337;momo;hookinstalled;deadbeef\x07"))
	if len(rendered) != 0 {
		t.Fatalf("expected the hookinstalled marker to be fully stripped, got %q", rendered)
	}
	if len(events) != 1 || events[0].kind != evHookInstalled || events[0].nonce != "deadbeef" {
		t.Fatalf("expected evHookInstalled nonce=deadbeef, got %+v", events)
	}
}

func TestParser_UnrecognizedOSCPassesThrough(t *testing.T) {
	var p parser
	// OSC 0 (window title) -- not one of our markers.
	input := []byte("\x1b]0;my title\x07rest")
	rendered, events := p.feed(input)
	if !bytes.Equal(rendered, input) {
		t.Fatalf("expected unrecognized OSC to pass through byte-for-byte, got %q want %q", rendered, input)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events for an unrecognized OSC, got %+v", events)
	}
}

func TestParser_STTerminatedUnit(t *testing.T) {
	var p parser
	// ESC \ (ST) terminator instead of BEL.
	rendered, events := p.feed([]byte("x\x1b]133;A\x1b\\y"))
	if string(rendered) != "xy" {
		t.Fatalf("rendered = %q, want %q", rendered, "xy")
	}
	if len(events) != 1 || events[0].kind != evPromptStart {
		t.Fatalf("expected evPromptStart, got %+v", events)
	}
}

// TestParser_ChunkSplitExhaustive feeds one known-good input to the parser
// split at EVERY possible byte offset across two feed() calls, asserting
// identical rendered+event output to feeding it whole in one call. This is
// the authoritative proof for "청크 경계 분할: 무손실 복원" (doc 18/19) --
// stronger than relying on real PTY read timing, which is nondeterministic.
func TestParser_ChunkSplitExhaustive(t *testing.T) {
	whole := []byte("hello\x1b]133;A\x07world\x1b]133;C\x07more output\x1b]133;D;7\x07\x1b]1337;momo;hookinstalled;abc123\x07tail")

	var want parser
	wantRendered, wantEvents := want.feed(whole)

	for split := 0; split <= len(whole); split++ {
		var p parser
		r1, e1 := p.feed(whole[:split])
		r2, e2 := p.feed(whole[split:])

		gotRendered := append(append([]byte{}, r1...), r2...)
		gotEvents := append(append([]parsedEvent{}, e1...), e2...)

		if !bytes.Equal(gotRendered, wantRendered) {
			t.Fatalf("split=%d: rendered = %q, want %q", split, gotRendered, wantRendered)
		}
		if len(gotEvents) != len(wantEvents) {
			t.Fatalf("split=%d: got %d events, want %d: got=%+v want=%+v", split, len(gotEvents), len(wantEvents), gotEvents, wantEvents)
		}
		for i := range wantEvents {
			if gotEvents[i] != wantEvents[i] {
				t.Fatalf("split=%d: event %d = %+v, want %+v", split, i, gotEvents[i], wantEvents[i])
			}
		}
	}
}

func TestParser_MultipleChunkSplits(t *testing.T) {
	// A marker split into three separate feed() calls (not just two).
	var p parser
	r1, e1 := p.feed([]byte("a\x1b]13"))
	r2, e2 := p.feed([]byte("3;A\x07"))
	r3, e3 := p.feed([]byte("b"))

	rendered := string(r1) + string(r2) + string(r3)
	events := append(append(e1, e2...), e3...)

	if rendered != "ab" {
		t.Fatalf("rendered = %q, want %q", rendered, "ab")
	}
	if len(events) != 1 || events[0].kind != evPromptStart {
		t.Fatalf("expected evPromptStart, got %+v", events)
	}
}

func TestAltScreenDetector_EnterExit(t *testing.T) {
	var d altScreenDetector
	enter, exit := d.scan([]byte("before\x1b[?1049hmid\x1b[?1049lafter"))
	if !enter || !exit {
		t.Fatalf("enter=%v exit=%v, want both true", enter, exit)
	}
}

func TestAltScreenDetector_NoMarkers(t *testing.T) {
	var d altScreenDetector
	enter, exit := d.scan([]byte("just plain output"))
	if enter || exit {
		t.Fatalf("enter=%v exit=%v, want both false", enter, exit)
	}
}

func TestAltScreenDetector_SplitAcrossChunks(t *testing.T) {
	full := []byte("x\x1b[?1049hy\x1b[?1049lz")
	for split := 0; split <= len(full); split++ {
		var d altScreenDetector
		enter1, exit1 := d.scan(full[:split])
		enter2, exit2 := d.scan(full[split:])
		enter := enter1 || enter2
		exit := exit1 || exit2
		if !enter || !exit {
			t.Fatalf("split=%d: enter=%v exit=%v, want both true", split, enter, exit)
		}
	}
}
