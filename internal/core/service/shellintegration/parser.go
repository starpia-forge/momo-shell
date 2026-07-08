package shellintegration

import (
	"bytes"
	"strconv"
	"strings"
)

type eventKind int

const (
	evPromptStart eventKind = iota
	evCommandStart
	evCommandDone
	evHookInstalled
)

type parsedEvent struct {
	kind     eventKind
	exitCode int
	nonce    string
}

const (
	oscStart = "\x1b]"
	bel      = byte('\x07')
	esc      = byte('\x1b')
)

// parser strips OSC133/1337 shell-integration markers from a session's raw
// output, carrying a possibly-incomplete OSC unit across feed calls the
// same way transfer/detect.go's signatureDetector carries a tail across
// pump-flushed chunks -- except here the delimited region is a
// variable-length OSC unit rather than a fixed 6-byte signature, and
// unrelated bytes surround it on both sides (must keep rendering them)
// rather than being suppressed wholesale like ZMODEM's all-or-nothing
// active phase. Unrecognized OSC sequences (window title, etc.) are passed
// through untouched -- only markers this package's own hook scripts emit
// are stripped. Verified against real bash/PowerShell output in the S4
// spike (.claudedocs/plan/19-mcp-s4-shellintegration-spike.md §2/§3).
type parser struct {
	tail []byte
}

func (p *parser) feed(chunk []byte) (rendered []byte, events []parsedEvent) {
	buf := append(p.tail, chunk...)
	p.tail = nil

	for {
		idx := bytes.Index(buf, []byte(oscStart))
		if idx < 0 {
			if len(buf) > 0 && buf[len(buf)-1] == esc {
				rendered = append(rendered, buf[:len(buf)-1]...)
				p.tail = []byte{esc}
			} else {
				rendered = append(rendered, buf...)
			}
			return rendered, events
		}

		rendered = append(rendered, buf[:idx]...)
		rest := buf[idx:]

		n, ok := findOSCEnd(rest)
		if !ok {
			// Incomplete OSC unit at the end of this chunk -- carry the
			// whole thing (from ESC ] onward) to the next feed call rather
			// than losing or misrendering it.
			p.tail = append([]byte{}, rest...)
			return rendered, events
		}

		unit := rest[:n]
		if ev, ok := parseOSCUnit(unit); ok {
			events = append(events, ev)
		} else {
			rendered = append(rendered, unit...)
		}
		buf = rest[n:]
	}
}

// findOSCEnd returns the byte length of the OSC unit starting at buf[0:2]
// ("ESC ]"), including its BEL or ST terminator, or ok=false if the
// terminator hasn't arrived yet within buf. Mirrors the OSC-framing branch
// of history/recorder.go's parseEscape.
func findOSCEnd(buf []byte) (n int, ok bool) {
	for i := 2; i < len(buf); i++ {
		if buf[i] == bel {
			return i + 1, true
		}
		if buf[i] == esc && i+1 < len(buf) && buf[i+1] == '\\' {
			return i + 2, true
		}
	}
	return 0, false
}

func trimTerminator(body []byte) []byte {
	if len(body) >= 1 && body[len(body)-1] == bel {
		return body[:len(body)-1]
	}
	if len(body) >= 2 && body[len(body)-2] == esc && body[len(body)-1] == '\\' {
		return body[:len(body)-2]
	}
	return body
}

func parseOSCUnit(unit []byte) (parsedEvent, bool) {
	if len(unit) < 2 {
		return parsedEvent{}, false
	}
	s := string(trimTerminator(unit[2:]))

	switch {
	case s == "133;A":
		return parsedEvent{kind: evPromptStart}, true
	case s == "133;C":
		return parsedEvent{kind: evCommandStart}, true
	case strings.HasPrefix(s, "133;D;"):
		code, err := strconv.Atoi(strings.TrimPrefix(s, "133;D;"))
		if err != nil {
			return parsedEvent{}, false
		}
		return parsedEvent{kind: evCommandDone, exitCode: code}, true
	case strings.HasPrefix(s, "1337;momo;hookinstalled;"):
		return parsedEvent{kind: evHookInstalled, nonce: strings.TrimPrefix(s, "1337;momo;hookinstalled;")}, true
	default:
		return parsedEvent{}, false
	}
}

// --- alt-screen detection (CSI, not OSC -- a different escape family, so
// this scans the parser's already-OSC-stripped rendered output rather than
// hooking into feed above) ---

var (
	altScreenEnter = []byte("\x1b[?1049h")
	altScreenExit  = []byte("\x1b[?1049l")
)

const maxAltScreenLen = 8

// altScreenDetector reports whether either alt-screen mode-1049 sequence
// appears in scanned output, carrying a possible-prefix tail across calls
// so a marker split across a chunk boundary isn't missed. It never
// modifies bytes -- unlike OSC133/1337 markers (always fully consumed),
// mode-1049 bytes must reach the terminal unmodified for the screen to
// actually switch; this only observes.
//
// Known limitation (doc 19 §4.2): on Windows, local sessions hosted via
// ConPTY never surface these bytes at all -- ConPTY implements the
// alternate-screen-buffer semantics itself rather than passing mode-1049
// through as a dumb pipe would, diverting everything written between enter
// and exit into an internal buffer this process never sees. SSH/remote
// sessions (a raw byte channel, no ConPTY) and local sessions on
// macOS/Linux (a real PTY) are unaffected.
type altScreenDetector struct {
	tail []byte
}

func (d *altScreenDetector) scan(chunk []byte) (enter, exit bool) {
	combined := append(d.tail, chunk...)
	d.tail = nil

	enter = bytes.Contains(combined, altScreenEnter)
	exit = bytes.Contains(combined, altScreenExit)

	hold := altScreenPrefixOverlap(combined)
	if hold > 0 {
		d.tail = append([]byte{}, combined[len(combined)-hold:]...)
	}
	return enter, exit
}

func altScreenPrefixOverlap(combined []byte) int {
	maxHold := maxAltScreenLen - 1
	if len(combined) < maxHold {
		maxHold = len(combined)
	}
	for l := maxHold; l > 0; l-- {
		suffix := combined[len(combined)-l:]
		if bytes.HasPrefix(altScreenEnter, suffix) || bytes.HasPrefix(altScreenExit, suffix) {
			return l
		}
	}
	return 0
}
