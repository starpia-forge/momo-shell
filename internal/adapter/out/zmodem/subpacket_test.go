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
