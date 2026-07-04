// Package history captures typed shell commands into a searchable,
// clickable-to-copy record, without any shell integration script -- it
// reconstructs command lines purely from the session's raw input byte
// stream, since that is the only signal available for every backend
// (local PTY and every SSH server) without asking the user to install
// anything.
package history

import (
	"strings"
	"unicode/utf8"
)

const (
	altScreenEnter = "\x1b[?1049h"
	altScreenExit  = "\x1b[?1049l"
)

// lineRecorder reconstructs one session's typed command lines from its raw
// input stream. It only understands enough terminal line-editing to handle
// common keys (backspace, arrow-key cursor movement, a handful of readline-
// style kill bindings) -- shell history recall (Up/Down) and Tab completion
// can still desync the buffer from what's actually on screen. This is a
// deliberate v1 limitation (favors recall over precision, per
// docs/plan/04-phase3-workspace-ux.md §3.1): alt-screen suppression, the
// 1-character minimum, sensitive-value masking, and a delete affordance in
// the history panel are what keep it useful despite the occasional bad entry.
type lineRecorder struct {
	buf    []rune
	cursor int

	// pending carries bytes of an escape sequence (or a multi-byte UTF-8
	// rune) that was still incomplete at the end of the last feed() call,
	// so a sequence split across two Write()s isn't misread as literal text.
	pending []byte

	// outTail carries up to len(altScreenEnter)-1 trailing output bytes
	// across scanAltScreen calls, so an alt-screen marker split across a
	// flush boundary is still detected.
	outTail []byte

	altScreen bool
}

func newLineRecorder() *lineRecorder {
	return &lineRecorder{}
}

// feed processes a chunk of raw input bytes and returns every command line
// committed (Enter pressed on a non-trivial line) within it. While an
// alt-screen program (vim, htop, ...) is active, input is ignored entirely
// rather than fed into the buffer, so its keystrokes never pollute history.
func (r *lineRecorder) feed(data []byte) []string {
	if r.altScreen {
		return nil
	}

	buf := append(r.pending, data...)
	r.pending = nil

	var commits []string
	i := 0
	for i < len(buf) {
		b := buf[i]
		switch {
		case b == '\x1b':
			consumed, complete := parseEscape(buf[i:])
			if !complete {
				r.pending = append(r.pending, buf[i:]...)
				i = len(buf)
				continue
			}
			r.handleEscape(buf[i : i+consumed])
			i += consumed

		case b == '\r' || b == '\n':
			if line, ok := r.commit(); ok {
				commits = append(commits, line)
			}
			i++

		case b == 0x7f || b == 0x08: // Backspace / DEL
			r.backspace()
			i++

		case b == 0x03: // Ctrl+C
			r.discard()
			i++

		case b == 0x15: // Ctrl+U -- kill to start of line
			r.killToStart()
			i++

		case b == 0x17: // Ctrl+W -- kill word backward
			r.killWordBack()
			i++

		case b == 0x0b: // Ctrl+K -- kill to end of line
			r.killToEnd()
			i++

		case b < 0x20: // other control bytes (Tab, etc.) -- not tracked
			i++

		default:
			if !utf8.FullRune(buf[i:]) {
				r.pending = append(r.pending, buf[i:]...)
				i = len(buf)
				continue
			}
			rn, size := utf8.DecodeRune(buf[i:])
			r.insert(rn)
			i += size
		}
	}
	return commits
}

// scanAltScreen inspects an output chunk for the alternate-screen enter/exit
// sequences and updates altScreen to whichever toggle occurs last in the
// (tail-joined) chunk.
func (r *lineRecorder) scanAltScreen(out []byte) {
	combined := append(r.outTail, out...)

	lastIdx, entry := -1, false
	if idx := lastIndex(combined, altScreenEnter); idx > lastIdx {
		lastIdx, entry = idx, true
	}
	if idx := lastIndex(combined, altScreenExit); idx > lastIdx {
		lastIdx, entry = idx, false
	}
	if lastIdx >= 0 {
		r.altScreen = entry
	}

	keep := len(altScreenEnter) - 1
	if len(combined) <= keep {
		r.outTail = append([]byte(nil), combined...)
		return
	}
	r.outTail = append([]byte(nil), combined[len(combined)-keep:]...)
}

func lastIndex(haystack []byte, needle string) int {
	return strings.LastIndex(string(haystack), needle)
}

// parseEscape reports how many bytes of an escape sequence starting at
// b[0]=='\x1b' were consumed, and whether it was complete (false if b ends
// mid-sequence, e.g. split across two Write calls).
func parseEscape(b []byte) (consumed int, complete bool) {
	if len(b) < 2 {
		return 0, false
	}
	switch b[1] {
	case '[': // CSI: ESC [ params... final
		for i := 2; i < len(b); i++ {
			if b[i] >= 0x40 && b[i] <= 0x7e {
				return i + 1, true
			}
		}
		return 0, false
	case ']': // OSC: ESC ] ... (BEL | ESC \)
		for i := 2; i < len(b); i++ {
			if b[i] == 0x07 {
				return i + 1, true
			}
			if b[i] == '\x1b' && i+1 < len(b) && b[i+1] == '\\' {
				return i + 2, true
			}
		}
		return 0, false
	default: // single-character escape (e.g. keypad SS3 sequences)
		return 2, true
	}
}

// handleEscape updates cursor position for the CSI sequences shells
// commonly use for line editing (arrow keys, Home/End, Delete). Anything
// else is consumed silently -- it was already excluded from the buffer by
// virtue of going through this path instead of the printable-rune case.
func (r *lineRecorder) handleEscape(seq []byte) {
	if len(seq) < 3 || seq[1] != '[' {
		return
	}
	final := seq[len(seq)-1]
	params := string(seq[2 : len(seq)-1])

	switch final {
	case 'C': // Right
		if r.cursor < len(r.buf) {
			r.cursor++
		}
	case 'D': // Left
		if r.cursor > 0 {
			r.cursor--
		}
	case 'H': // Home (no params -- CUP with params isn't used for line editing)
		if params == "" {
			r.cursor = 0
		}
	case 'F': // End
		if params == "" {
			r.cursor = len(r.buf)
		}
	case '~': // Home/End/Delete on terminals that send them as ESC [ n ~
		switch params {
		case "1", "7":
			r.cursor = 0
		case "4", "8":
			r.cursor = len(r.buf)
		case "3":
			r.deleteForward()
		}
	}
}

func (r *lineRecorder) insert(rn rune) {
	r.buf = append(r.buf, 0)
	copy(r.buf[r.cursor+1:], r.buf[r.cursor:])
	r.buf[r.cursor] = rn
	r.cursor++
}

func (r *lineRecorder) backspace() {
	if r.cursor == 0 {
		return
	}
	copy(r.buf[r.cursor-1:], r.buf[r.cursor:])
	r.buf = r.buf[:len(r.buf)-1]
	r.cursor--
}

func (r *lineRecorder) deleteForward() {
	if r.cursor >= len(r.buf) {
		return
	}
	copy(r.buf[r.cursor:], r.buf[r.cursor+1:])
	r.buf = r.buf[:len(r.buf)-1]
}

func (r *lineRecorder) killToStart() {
	r.buf = r.buf[r.cursor:]
	r.cursor = 0
}

func (r *lineRecorder) killToEnd() {
	r.buf = r.buf[:r.cursor]
}

func (r *lineRecorder) killWordBack() {
	end := r.cursor
	i := r.cursor
	for i > 0 && r.buf[i-1] == ' ' {
		i--
	}
	for i > 0 && r.buf[i-1] != ' ' {
		i--
	}
	r.buf = append(r.buf[:i], r.buf[end:]...)
	r.cursor = i
}

func (r *lineRecorder) discard() {
	r.buf = r.buf[:0]
	r.cursor = 0
}

// commit finalizes the current buffer as a candidate history line: trimmed,
// and only accepted if it has more than one character (skips accidental
// single-key "commands" and stray Enters).
func (r *lineRecorder) commit() (string, bool) {
	line := strings.TrimSpace(string(r.buf))
	r.buf = r.buf[:0]
	r.cursor = 0
	if utf8.RuneCountInString(line) <= 1 {
		return "", false
	}
	return line, true
}
