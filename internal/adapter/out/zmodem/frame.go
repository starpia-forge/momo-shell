package zmodem

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// errCanceled is returned when the peer sends a cancel sequence (repeated
// ZDLE/CAN bytes) instead of a valid frame.
var errCanceled = errors.New("zmodem: peer canceled")

func le32(v uint32) [4]byte {
	return [4]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

func decodeLE32(b [4]byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// header is one ZMODEM header: a frame type plus 4 bytes whose meaning
// depends on the type (a little-endian file position for ZRPOS/ZDATA/ZEOF,
// capability flags for ZRINIT, or unused for ZRQINIT/ZFIN/ZCAN).
type header struct {
	typ  byte
	data [4]byte
}

func (h header) position() uint32 { return decodeLE32(h.data) }

// writeHexHeader sends h using the ASCII-hex format conventionally used for
// the initial handshake (ZRQINIT/ZRINIT/ZFIN/ZACK) so it survives paths
// that might not be 8-bit clean. CRC is always 16-bit in this format.
func writeHexHeader(w io.Writer, h header) error {
	buf := []byte{zpad, zpad, zdle, zhex}
	payload := append([]byte{h.typ}, h.data[:]...)
	crc := crc16(payload)
	full := append(payload, byte(crc>>8), byte(crc))
	for _, b := range full {
		buf = append(buf, hexDigit(b>>4), hexDigit(b&0xf))
	}
	// CR + LF-with-8th-bit-set + XON: the exact trailing bytes real
	// lrzsz emits after a hex header (verified against captured output).
	buf = append(buf, '\r', 0x8a, xon)
	_, err := w.Write(buf)
	return err
}

func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}

func unhexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// writeBin32Header sends h in binary form with a 32-bit CRC -- the format
// used for everything after the initial handshake.
func writeBin32Header(w io.Writer, h header) error {
	buf := []byte{zpad, zpad, zdle, zbin32}
	payload := append([]byte{h.typ}, h.data[:]...)
	crc := crc32Of(payload)
	crcBytes := []byte{byte(crc), byte(crc >> 8), byte(crc >> 16), byte(crc >> 24)}
	for _, b := range append(payload, crcBytes...) {
		buf = appendEscaped(buf, b)
	}
	_, err := w.Write(buf)
	return err
}

// appendEscaped appends b to buf, ZDLE-escaping it first if it's the escape
// character itself or one of a small set of bytes that could be mistaken
// for flow control by an intervening layer (XON/XOFF/CR). Escaping more
// than the negotiated minimum is always safe -- any compliant receiver
// unescapes generically.
func appendEscaped(buf []byte, b byte) []byte {
	switch b {
	case zdle, xon, 0x13 /* XOFF */, 0x0d /* CR */ :
		return append(buf, zdle, b^0x40)
	default:
		return append(buf, b)
	}
}

// frameReader wraps a buffered byte source with the ZMODEM sync/escape
// state machine: finding the next header regardless of what garbage
// (echoed shell output, stray XON bytes) precedes it, and unescaping
// header/subpacket bytes contextually.
type frameReader struct {
	r *bufio.Reader
}

func newFrameReader(r io.Reader) *frameReader {
	return &frameReader{r: bufio.NewReaderSize(r, 4096)}
}

func (f *frameReader) readByte() (byte, error) {
	return f.r.ReadByte()
}

// syncToHeader scans forward until it sees ZDLE followed by a recognized
// header-format selector, discarding everything before it (echoed "rz"
// text, repeated ZPAD padding, stray XON bytes -- all expected noise before
// a real header in practice).
func (f *frameReader) syncToHeader() (format byte, err error) {
	for {
		b, err := f.readByte()
		if err != nil {
			return 0, err
		}
		if b != zdle {
			continue
		}
		next, err := f.readByte()
		if err != nil {
			return 0, err
		}
		switch next {
		case zbin, zhex, zbin32:
			return next, nil
		case zdle:
			return 0, errCanceled
		}
		// Anything else after a stray ZDLE is noise; keep scanning.
	}
}

// readEscapedByte reads one logical (already-unescaped) byte from header or
// subpacket context, transparently consuming ZDLE-escape pairs.
func (f *frameReader) readEscapedByte() (byte, error) {
	b, err := f.readByte()
	if err != nil {
		return 0, err
	}
	if b != zdle {
		return b, nil
	}
	e, err := f.readByte()
	if err != nil {
		return 0, err
	}
	if e == zdle {
		return 0, errCanceled
	}
	return e ^ 0x40, nil
}

// readHeader synchronizes to the next header and decodes it, dispatching on
// the format byte found during sync.
func (f *frameReader) readHeader() (header, error) {
	format, err := f.syncToHeader()
	if err != nil {
		return header{}, err
	}

	switch format {
	case zhex:
		return f.readHexHeaderBody()
	case zbin, zbin32:
		return f.readBinHeaderBody(format == zbin32)
	default:
		return header{}, fmt.Errorf("zmodem: unknown header format %q", format)
	}
}

func (f *frameReader) readHexHeaderBody() (header, error) {
	raw := make([]byte, 7) // type + 4 data + 2 crc
	for i := range raw {
		hi, err := f.readByte()
		if err != nil {
			return header{}, err
		}
		lo, err := f.readByte()
		if err != nil {
			return header{}, err
		}
		hiN, ok1 := unhexDigit(hi)
		loN, ok2 := unhexDigit(lo)
		if !ok1 || !ok2 {
			return header{}, fmt.Errorf("zmodem: invalid hex header byte %q%q", hi, lo)
		}
		raw[i] = hiN<<4 | loN
	}
	// Trailing CR/LF (and possibly XON) are conventional but not load-bearing; skip them.
	f.consumeLineEnding()

	got := crc16(raw[:5])
	want := uint16(raw[5])<<8 | uint16(raw[6])
	if got != want {
		return header{}, fmt.Errorf("zmodem: hex header CRC mismatch")
	}
	var h header
	h.typ = raw[0]
	copy(h.data[:], raw[1:5])
	return h, nil
}

// consumeLineEnding best-effort-skips the CR/LF (and optional XON) that
// conventionally follow a hex header. It only looks at bytes already
// buffered -- peeking for one more than the peer actually sent would block
// forever waiting on a reply that's blocked on us reading first. Any
// trailing byte left unconsumed is harmless: syncToHeader treats it as
// ordinary noise before the next header.
func (f *frameReader) consumeLineEnding() {
	for i := 0; i < 3; i++ {
		if f.r.Buffered() == 0 {
			return
		}
		b, err := f.r.Peek(1)
		if err != nil || (b[0] != '\r' && b[0] != '\n' && b[0] != 0x8a && b[0] != xon) {
			return
		}
		_, _ = f.readByte()
	}
}

func (f *frameReader) readBinHeaderBody(is32 bool) (header, error) {
	n := 5
	crcLen := 2
	if is32 {
		crcLen = 4
	}
	raw := make([]byte, 0, n+crcLen)
	for i := 0; i < n+crcLen; i++ {
		b, err := f.readEscapedByte()
		if err != nil {
			return header{}, err
		}
		raw = append(raw, b)
	}

	payload := raw[:n]
	if is32 {
		got := crc32Of(payload)
		want := uint32(raw[5]) | uint32(raw[6])<<8 | uint32(raw[7])<<16 | uint32(raw[8])<<24
		if got != want {
			return header{}, fmt.Errorf("zmodem: bin32 header CRC mismatch")
		}
	} else {
		got := crc16(payload)
		want := uint16(raw[5])<<8 | uint16(raw[6])
		if got != want {
			return header{}, fmt.Errorf("zmodem: bin header CRC mismatch")
		}
	}

	var h header
	h.typ = payload[0]
	copy(h.data[:], payload[1:5])
	return h, nil
}
