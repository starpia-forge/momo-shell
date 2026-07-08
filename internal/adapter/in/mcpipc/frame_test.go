package mcpipc

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestFrame_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := authRequest{Type: frameTypeAuth, Token: "abc123"}
	if err := writeFrame(&buf, want); err != nil {
		t.Fatalf("writeFrame() error = %v", err)
	}

	var got authRequest
	if err := readFrame(&buf, &got); err != nil {
		t.Fatalf("readFrame() error = %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReadFrame_RejectsOversizedLengthPrefix(t *testing.T) {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], maxAuthFrameBytes+1)
	buf.Write(lenBuf[:])

	var v authRequest
	if err := readFrame(&buf, &v); err != errFrameTooLarge {
		t.Fatalf("readFrame() error = %v, want errFrameTooLarge", err)
	}
}
