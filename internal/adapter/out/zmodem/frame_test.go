package zmodem

import (
	"bytes"
	"testing"
)

func TestHexHeader_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := header{typ: zrinit, data: le32(0x01020304)}
	if err := writeHexHeader(&buf, want); err != nil {
		t.Fatalf("writeHexHeader: %v", err)
	}

	got, err := newFrameReader(&buf).readHeader()
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestBin32Header_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := header{typ: zdata, data: le32(1_048_576)}
	if err := writeBin32Header(&buf, want); err != nil {
		t.Fatalf("writeBin32Header: %v", err)
	}

	got, err := newFrameReader(&buf).readHeader()
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got.position() != 1_048_576 {
		t.Fatalf("position() = %d, want 1048576", got.position())
	}
}

func TestReadHeader_SkipsPrecedingGarbage(t *testing.T) {
	// Real streams have shell echo / rz's "rz\r" text before the first
	// header -- the reader must resync past it rather than erroring.
	var buf bytes.Buffer
	buf.WriteString("rz\r\n** some noise **")
	want := header{typ: zrqinit}
	if err := writeBin32Header(&buf, want); err != nil {
		t.Fatalf("writeBin32Header: %v", err)
	}

	got, err := newFrameReader(&buf).readHeader()
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReadHeader_CorruptedCRCFails(t *testing.T) {
	var buf bytes.Buffer
	if err := writeBin32Header(&buf, header{typ: zfin}); err != nil {
		t.Fatalf("writeBin32Header: %v", err)
	}
	corrupted := buf.Bytes()
	corrupted[len(corrupted)-1] ^= 0xff // flip a bit in the last CRC byte

	if _, err := newFrameReader(bytes.NewReader(corrupted)).readHeader(); err == nil {
		t.Fatalf("expected CRC mismatch error, got nil")
	}
}

func TestReadHeader_CancelSequenceReturnsErrCanceled(t *testing.T) {
	buf := bytes.NewReader(cancelSequence)
	if _, err := newFrameReader(buf).readHeader(); err != errCanceled {
		t.Fatalf("expected errCanceled, got %v", err)
	}
}
