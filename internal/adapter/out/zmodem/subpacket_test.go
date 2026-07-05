package zmodem

import (
	"bytes"
	"testing"
)

func TestSubpacket_RoundTrip(t *testing.T) {
	for _, term := range []byte{zcrce, zcrcg, zcrcq, zcrcw} {
		payload := []byte("hello, zmodem world! this has \x18 a literal ZDLE byte in it.")
		var buf bytes.Buffer
		if err := writeSubpacket(&buf, payload, term); err != nil {
			t.Fatalf("writeSubpacket(term=%q): %v", term, err)
		}

		gotPayload, gotTerm, err := newFrameReader(&buf).readSubpacket()
		if err != nil {
			t.Fatalf("readSubpacket(term=%q): %v", term, err)
		}
		if !bytes.Equal(gotPayload, payload) {
			t.Fatalf("term=%q: payload mismatch: got %q, want %q", term, gotPayload, payload)
		}
		if gotTerm != term {
			t.Fatalf("term mismatch: got %q, want %q", gotTerm, term)
		}
	}
}

func TestSubpacket_EmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	if err := writeSubpacket(&buf, nil, zcrce); err != nil {
		t.Fatalf("writeSubpacket: %v", err)
	}
	payload, term, err := newFrameReader(&buf).readSubpacket()
	if err != nil {
		t.Fatalf("readSubpacket: %v", err)
	}
	if len(payload) != 0 || term != zcrce {
		t.Fatalf("got payload=%q term=%q, want empty/zcrce", payload, term)
	}
}

// TestAppendEscaped_EscapesHighBitControlVariants is the regression test for
// a real interop bug found via live E2E testing against real lrzsz rz: a
// multi-chunk upload of random binary data was rejected by rz on every
// single subpacket (rz kept replying ZRPOS(0), demanding a restart from the
// very beginning, forever) even though our own loopback/round-trip tests
// never caught it -- because our own reader unescapes generically and never
// required these bytes to be escaped in the first place, so a bug here is
// invisible to a self-to-self round trip. Real rz, however, does require
// ZDLE, XON, XOFF, CR and their high-bit-set counterparts (0x98, 0x91, 0x93,
// 0x8d) to always be escaped -- appendEscaped previously only escaped the
// low-bit forms, so any of these four high-bit bytes in the payload (near
//-certain within a few KB of random binary) desynced real rz's parser.
func TestAppendEscaped_EscapesHighBitControlVariants(t *testing.T) {
	mustEscape := []byte{zdle, xon, 0x13, 0x0d, zdle | 0x80, xon | 0x80, 0x13 | 0x80, 0x0d | 0x80}
	for _, b := range mustEscape {
		got := appendEscaped(nil, b)
		if len(got) != 2 || got[0] != zdle || got[1] != b^0x40 {
			t.Fatalf("appendEscaped(0x%02x): got %v, want [zdle, 0x%02x]", b, got, b^0x40)
		}
	}
}

func TestSubpacket_CorruptedCRCFails(t *testing.T) {
	var buf bytes.Buffer
	if err := writeSubpacket(&buf, []byte("data"), zcrcw); err != nil {
		t.Fatalf("writeSubpacket: %v", err)
	}
	corrupted := buf.Bytes()
	corrupted[len(corrupted)-1] ^= 0xff

	if _, _, err := newFrameReader(bytes.NewReader(corrupted)).readSubpacket(); err == nil {
		t.Fatalf("expected CRC mismatch error, got nil")
	}
}
