package zmodem

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// scriptedFileInfo builds a minimal ZFILE info subpacket payload, matching
// the field format real lrzsz emits (see sendFile's own comment).
func scriptedFileInfo(name string, size int64) []byte {
	return []byte(fmt.Sprintf("%s%c%d 0 0 0 1 %d%c", name, 0, size, size, 0))
}

func expectHeader(fr *frameReader, want byte, wantPos ...uint32) (header, error) {
	h, err := fr.readHeader()
	if err != nil {
		return h, fmt.Errorf("reading header (want type %d): %w", want, err)
	}
	if h.typ != want {
		return h, fmt.Errorf("got frame type %d, want %d", h.typ, want)
	}
	if len(wantPos) > 0 && h.position() != wantPos[0] {
		return h, fmt.Errorf("got position %d, want %d", h.position(), wantPos[0])
	}
	return h, nil
}

// encodeSubpacket renders exactly what writeSubpacket would send, for tests
// that need to mutate the wire bytes afterward (e.g. corrupting a CRC).
func encodeSubpacket(payload []byte, term byte) []byte {
	var buf bytes.Buffer
	_ = writeSubpacket(&buf, payload, term)
	return buf.Bytes()
}

// TestReceive_ZcrcwAckThenDirectZeof reproduces the FT-11 field bug: a real
// sz build ends its single-run data phase with zcrcw, and upon our ZACK,
// sends ZEOF directly rather than announcing a fresh ZDATA run first. The
// original receiveOneFile hard-required ZDATA there and aborted with
// "expected ZDATA, got 11" -- this is the direct regression test for that
// fix (receive.go's zcrcw case now accepts zdata OR zeof).
func TestReceive_ZcrcwAckThenDirectZeof(t *testing.T) {
	ours, peer := newPipePair()
	content := []byte("hello world, this is the whole file in one subpacket")

	errCh := make(chan error, 1)
	go func() {
		errCh <- func() error {
			fr := newFrameReader(peer)

			// Receive() sends ZRINIT first, unprompted.
			if _, err := expectHeader(fr, zrinit); err != nil {
				return err
			}
			if err := writeBin32Header(peer, header{typ: zfile}); err != nil {
				return err
			}
			if err := writeSubpacket(peer, scriptedFileInfo("script.txt", int64(len(content))), zcrcw); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zrpos, 0); err != nil {
				return err
			}

			if err := writeBin32Header(peer, header{typ: zdata, data: le32(0)}); err != nil {
				return err
			}
			if err := writeSubpacket(peer, content, zcrcw); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zack, uint32(len(content))); err != nil {
				return err
			}

			// The regression: ZEOF directly, no fresh ZDATA in between.
			if err := writeBin32Header(peer, header{typ: zeof, data: le32(uint32(len(content)))}); err != nil {
				return err
			}

			if _, err := expectHeader(fr, zrinit); err != nil {
				return err
			}
			if err := writeHexHeader(peer, header{typ: zfin}); err != nil {
				return err
			}
			_, err := expectHeader(fr, zfin)
			return err
		}()
	}()

	dstDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	saved, err := New().Receive(ctx, ours, dstDir, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if perr := <-errCh; perr != nil {
		t.Fatalf("scripted peer: %v", perr)
	}
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved file, got %d: %v", len(saved), saved)
	}
	got, err := os.ReadFile(filepath.Join(dstDir, "script.txt"))
	if err != nil {
		t.Fatalf("read received file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content mismatch: got %q want %q", got, content)
	}
}

// TestReceive_ZcrcgAndZcrcqContinuation proves the receiver correctly
// handles a streamed run (zcrcg subpackets with no ack, a zcrcq subpacket
// that does expect one) before the final zcrce -- terminators the original
// code didn't handle at all.
func TestReceive_ZcrcgAndZcrcqContinuation(t *testing.T) {
	ours, peer := newPipePair()
	part1 := []byte("first part streamed as zcrcg, ")
	part2 := []byte("second part as zcrcq, ")
	part3 := []byte("final part as zcrce.")
	content := append(append(append([]byte{}, part1...), part2...), part3...)

	errCh := make(chan error, 1)
	go func() {
		errCh <- func() error {
			fr := newFrameReader(peer)

			if _, err := expectHeader(fr, zrinit); err != nil {
				return err
			}
			if err := writeBin32Header(peer, header{typ: zfile}); err != nil {
				return err
			}
			if err := writeSubpacket(peer, scriptedFileInfo("stream.bin", int64(len(content))), zcrcw); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zrpos, 0); err != nil {
				return err
			}

			if err := writeBin32Header(peer, header{typ: zdata, data: le32(0)}); err != nil {
				return err
			}
			if err := writeSubpacket(peer, part1, zcrcg); err != nil {
				return err
			}
			if err := writeSubpacket(peer, part2, zcrcq); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zack, uint32(len(part1)+len(part2))); err != nil {
				return err
			}
			if err := writeSubpacket(peer, part3, zcrce); err != nil {
				return err
			}
			if err := writeBin32Header(peer, header{typ: zeof, data: le32(uint32(len(content)))}); err != nil {
				return err
			}

			if _, err := expectHeader(fr, zrinit); err != nil {
				return err
			}
			if err := writeHexHeader(peer, header{typ: zfin}); err != nil {
				return err
			}
			_, err := expectHeader(fr, zfin)
			return err
		}()
	}()

	dstDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	saved, err := New().Receive(ctx, ours, dstDir, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if perr := <-errCh; perr != nil {
		t.Fatalf("scripted peer: %v", perr)
	}
	got, err := os.ReadFile(saved[0])
	if err != nil {
		t.Fatalf("read received file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content mismatch: got %q want %q", got, content)
	}
}

// TestReceive_CorruptedSubpacketResyncsViaZRPOS proves a subpacket with a
// bad CRC doesn't abort the transfer: the receiver asks for a resend from
// the last good offset (ZRPOS) and continues once the sender obliges,
// instead of erroring out immediately like the original code did.
func TestReceive_CorruptedSubpacketResyncsViaZRPOS(t *testing.T) {
	ours, peer := newPipePair()
	content := []byte("this subpacket's first attempt will be corrupted on the wire")

	errCh := make(chan error, 1)
	go func() {
		errCh <- func() error {
			fr := newFrameReader(peer)

			if _, err := expectHeader(fr, zrinit); err != nil {
				return err
			}
			if err := writeBin32Header(peer, header{typ: zfile}); err != nil {
				return err
			}
			if err := writeSubpacket(peer, scriptedFileInfo("corrupt.bin", int64(len(content))), zcrcw); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zrpos, 0); err != nil {
				return err
			}

			// First attempt: valid ZDATA header, but a subpacket whose CRC
			// we deliberately corrupt after building it correctly.
			if err := writeBin32Header(peer, header{typ: zdata, data: le32(0)}); err != nil {
				return err
			}
			corrupted := encodeSubpacket(content, zcrcw)
			corrupted[len(corrupted)-1] ^= 0xff
			if _, err := peer.Write(corrupted); err != nil {
				return err
			}

			// Receiver must ask to resume from offset 0 again.
			if _, err := expectHeader(fr, zrpos, 0); err != nil {
				return fmt.Errorf("after corrupted subpacket: %w", err)
			}

			// Second attempt: the same data, uncorrupted.
			if err := writeBin32Header(peer, header{typ: zdata, data: le32(0)}); err != nil {
				return err
			}
			if err := writeSubpacket(peer, content, zcrce); err != nil {
				return err
			}
			if err := writeBin32Header(peer, header{typ: zeof, data: le32(uint32(len(content)))}); err != nil {
				return err
			}

			if _, err := expectHeader(fr, zrinit); err != nil {
				return err
			}
			if err := writeHexHeader(peer, header{typ: zfin}); err != nil {
				return err
			}
			_, err := expectHeader(fr, zfin)
			return err
		}()
	}()

	dstDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	saved, err := New().Receive(ctx, ours, dstDir, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if perr := <-errCh; perr != nil {
		t.Fatalf("scripted peer: %v", perr)
	}
	got, err := os.ReadFile(saved[0])
	if err != nil {
		t.Fatalf("read received file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content mismatch: got %q want %q", got, content)
	}
}
