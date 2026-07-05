package zmodem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTempFile creates a file with the given content under a fresh temp
// dir and returns its path, for tests that need a real local.Send source.
func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	return path
}

// TestSend_WrongTypeFloodIsBounded reproduces the FT-12 "echo reflection"
// failure mode: a peer that keeps answering with valid-but-unwanted-type
// headers (previously: our own poke bytes reflected back by a shell after
// the real rz/sz process exited) must not spin waitForHeader forever, since
// a wrong-type header doesn't count against the silence-based retry budget.
func TestSend_WrongTypeFloodIsBounded(t *testing.T) {
	ours, peer := newPipePair()
	path := writeTempFile(t, "flood.txt", []byte("hello"))

	errCh := make(chan error, 1)
	go func() {
		errCh <- func() error {
			fr := newFrameReader(peer)
			if err := writeHexHeader(peer, header{typ: zrinit, data: [4]byte{0, 0, 0, canFC32}}); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zfile); err != nil {
				return err
			}
			if _, _, err := fr.readSubpacket(); err != nil {
				return err
			}
			// Flood with a valid header of a type sendFile never asked for
			// -- more than maxWrongTypeFrames of them -- instead of the
			// zrpos/zskip it's actually waiting on.
			for i := 0; i < maxWrongTypeFrames+5; i++ {
				if err := writeBin32Header(peer, header{typ: zack, data: le32(0)}); err != nil {
					return err
				}
			}
			return nil
		}()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := New().Send(ctx, ours, []string{path}, nil)
	if err == nil {
		t.Fatal("expected Send to fail once the wrong-type frame flood exceeds maxWrongTypeFrames, got nil error")
	}
	if perr := <-errCh; perr != nil {
		t.Fatalf("scripted peer: %v", perr)
	}
}

// TestSend_EndgameAcceptsZfinInPlaceOfZrinit reproduces the receiver ending
// the exchange unilaterally right after ZEOF (in practice: because its
// rz/sz process has already exited) instead of announcing a fresh ZRINIT.
// Send must recognize this and skip straight to "OO" instead of waiting for
// a second ZFIN reply that will never come from a peer that's already gone.
func TestSend_EndgameAcceptsZfinInPlaceOfZrinit(t *testing.T) {
	ours, peer := newPipePair()
	content := []byte("whole file fits in one subpacket")
	path := writeTempFile(t, "onefile.txt", content)

	errCh := make(chan error, 1)
	go func() {
		errCh <- func() error {
			fr := newFrameReader(peer)
			if err := writeHexHeader(peer, header{typ: zrinit, data: [4]byte{0, 0, 0, canFC32}}); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zfile); err != nil {
				return err
			}
			if _, _, err := fr.readSubpacket(); err != nil {
				return err
			}
			if err := writeBin32Header(peer, header{typ: zrpos, data: le32(0)}); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zdata); err != nil {
				return err
			}
			if _, _, err := fr.readSubpacket(); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zeof); err != nil {
				return err
			}
			// The regression: ZFIN directly, no fresh ZRINIT -- the
			// receiver has already decided the exchange is over.
			return writeHexHeader(peer, header{typ: zfin})
		}()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := New().Send(ctx, ours, []string{path}, nil); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if perr := <-errCh; perr != nil {
		t.Fatalf("scripted peer: %v", perr)
	}
}

// TestSend_EndgameSilenceFailsFastNotSlow proves the post-ZEOF wait gives up
// within the reduced endgame budget rather than the full ~100s general-
// purpose retry budget when the receiver goes completely silent right after
// ZEOF (simulating its rz/sz process having already exited) -- this is the
// direct regression test for FT-12's permanent terminal freeze.
func TestSend_EndgameSilenceFailsFastNotSlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-time endgame timeout test in -short mode")
	}
	ours, peer := newPipePair()
	content := []byte("data fully sent, then the peer vanishes")
	path := writeTempFile(t, "vanishing.txt", content)

	errCh := make(chan error, 1)
	go func() {
		errCh <- func() error {
			fr := newFrameReader(peer)
			if err := writeHexHeader(peer, header{typ: zrinit, data: [4]byte{0, 0, 0, canFC32}}); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zfile); err != nil {
				return err
			}
			if _, _, err := fr.readSubpacket(); err != nil {
				return err
			}
			if err := writeBin32Header(peer, header{typ: zrpos, data: le32(0)}); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zdata); err != nil {
				return err
			}
			if _, _, err := fr.readSubpacket(); err != nil {
				return err
			}
			if _, err := expectHeader(fr, zeof); err != nil {
				return err
			}
			// Then: nothing. The remote process has "exited".
			return nil
		}()
	}()

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	err := New().Send(ctx, ours, []string{path}, nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected Send to fail once the endgame retry budget is exhausted")
	}
	// endgameMaxRetries=3 at ioReadTimeout=10s each should give up well
	// before the general-purpose maxIORetries=10 budget (~100s) would.
	if elapsed > 60*time.Second {
		t.Fatalf("Send took %s to fail -- endgame budget doesn't appear to be in effect", elapsed)
	}
	if perr := <-errCh; perr != nil {
		t.Fatalf("scripted peer: %v", perr)
	}
}
