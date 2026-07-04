package history

import (
	"reflect"
	"testing"
)

func feedAll(r *lineRecorder, chunks ...string) []string {
	var commits []string
	for _, c := range chunks {
		commits = append(commits, r.feed([]byte(c))...)
	}
	return commits
}

func TestLineRecorder_CommitsOnEnter(t *testing.T) {
	r := newLineRecorder()
	got := feedAll(r, "ls -la\r")
	want := []string{"ls -la"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_Backspace(t *testing.T) {
	r := newLineRecorder()
	got := feedAll(r, "lsx\x7f\r") // "lsx" then backspace then Enter
	want := []string{"ls"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_CtrlC_DiscardsBuffer(t *testing.T) {
	r := newLineRecorder()
	got := feedAll(r, "rm -rf /\x03", "ls\r")
	want := []string{"ls"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_ArrowKeys_MoveCursorForInsert(t *testing.T) {
	r := newLineRecorder()
	// Type "ad", move left (cursor before 'd'), insert 'c' -> "acd".
	got := feedAll(r, "ad\x1b[Dc\r")
	want := []string{"acd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_EscapeSequenceSplitAcrossFeedCalls(t *testing.T) {
	r := newLineRecorder()
	got := feedAll(r, "ab\x1b[", "Dc\r") // Left-arrow CSI split mid-sequence
	want := []string{"acb"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_CtrlU_KillToStart(t *testing.T) {
	r := newLineRecorder()
	got := feedAll(r, "hello world\x15", "bye\r")
	want := []string{"bye"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_CtrlK_KillToEnd(t *testing.T) {
	r := newLineRecorder()
	// Type "hello", move left 3 (cursor after "he"), Ctrl+K -> "he".
	got := feedAll(r, "hello\x1b[D\x1b[D\x1b[D\x0b\r")
	want := []string{"he"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_CtrlW_KillWordBack(t *testing.T) {
	r := newLineRecorder()
	got := feedAll(r, "foo bar\x17\r") // trailing "bar" killed, trims to "foo"
	want := []string{"foo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_SkipsEmptyAndSingleCharLines(t *testing.T) {
	r := newLineRecorder()
	got := feedAll(r, "\r", "a\r", "  \r")
	if len(got) != 0 {
		t.Fatalf("expected no commits, got %v", got)
	}
}

func TestLineRecorder_MultiByteUTF8_CountsRunesNotBytes(t *testing.T) {
	r := newLineRecorder()
	// A single Korean syllable is 3 bytes but 1 rune -- must NOT commit.
	got := feedAll(r, "안\r")
	if len(got) != 0 {
		t.Fatalf("expected single-rune line to be skipped, got %v", got)
	}

	got = feedAll(r, "안녕\r")
	want := []string{"안녕"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_AltScreen_SuppressesCapture(t *testing.T) {
	r := newLineRecorder()
	r.scanAltScreen([]byte("\x1b[?1049h")) // vim/htop enters alt screen
	got := feedAll(r, "this should be ignored\r")
	if len(got) != 0 {
		t.Fatalf("expected no commits while alt-screen active, got %v", got)
	}

	r.scanAltScreen([]byte("\x1b[?1049l")) // exits alt screen
	got = feedAll(r, "ls\r")
	want := []string{"ls"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLineRecorder_AltScreen_MarkerSplitAcrossChunks(t *testing.T) {
	r := newLineRecorder()
	r.scanAltScreen([]byte("\x1b[?104"))
	r.scanAltScreen([]byte("9h"))
	if !r.altScreen {
		t.Fatal("expected alt-screen enter marker split across chunks to be detected")
	}

	r.scanAltScreen([]byte("\x1b[?104"))
	r.scanAltScreen([]byte("9l"))
	if r.altScreen {
		t.Fatal("expected alt-screen exit marker split across chunks to be detected")
	}
}

func TestLineRecorder_AltScreen_LastMarkerInChunkWins(t *testing.T) {
	r := newLineRecorder()
	// Enter then exit within the same output chunk -- final state must be "exited".
	r.scanAltScreen([]byte("\x1b[?1049h" + "some ui" + "\x1b[?1049l"))
	if r.altScreen {
		t.Fatal("expected the later exit marker to win over the earlier enter marker")
	}
}
