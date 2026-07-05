package zmodem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"momo-shell/internal/core/port/out"
)

// waitForHeader blocks for one header of any of the given types, silently
// discarding a header of a different type (e.g. a duplicate ZRINIT that
// crossed in transit with our own request) -- treating every response as
// authoritative would misinterpret an unrelated frame's data bytes as this
// one's payload. If a read stalls (errReadTimeout), poke (if non-nil) is
// invoked to nudge the peer -- typically by resending the last frame it
// might have missed -- and the read is retried, up to maxIORetries times.
func waitForHeader(ctx context.Context, fr *frameReader, poke func() error, want ...byte) (header, error) {
	retries := 0
	for {
		if err := ctx.Err(); err != nil {
			return header{}, err
		}
		h, err := fr.readHeader()
		if err == nil {
			for _, w := range want {
				if h.typ == w {
					return h, nil
				}
			}
			continue // wrong type -- doesn't count against the retry budget
		}
		if !errors.Is(err, errReadTimeout) {
			return header{}, err
		}
		retries++
		if retries > maxIORetries {
			return header{}, fmt.Errorf("zmodem: timed out waiting for header type in %v after %d retries", want, maxIORetries)
		}
		if poke != nil {
			if err := poke(); err != nil {
				return header{}, err
			}
		}
	}
}

// Send implements out.StreamTransfer: drives our sender role against a
// remote rz waiting to receive.
func (e *Engine) Send(ctx context.Context, rw io.ReadWriter, localPaths []string, progress func(out.TransferProgress)) error {
	fr := newFrameReader(rw)

	if err := negotiateSend(ctx, rw, fr); err != nil {
		return err
	}

	for _, path := range localPaths {
		if err := sendFile(ctx, rw, fr, path, progress); err != nil {
			return err
		}
	}

	if err := writeHexHeader(rw, header{typ: zfin}); err != nil {
		return err
	}
	sendZFIN := func() error { return writeHexHeader(rw, header{typ: zfin}) }
	if _, err := waitForHeader(ctx, fr, sendZFIN, zfin); err != nil {
		return err
	}
	_, err := rw.Write([]byte("OO"))
	return err
}

// negotiateSend waits for the receiver's ZRINIT. Real receivers (rz)
// announce it unprompted as soon as they start, without waiting for our
// ZRQINIT -- and empirically, sending ZRQINIT anyway *after* the receiver
// already has desyncs at least rz's state machine, even though the spec
// treats an extra ZRQINIT as harmless. So: listen first, and only prompt
// with our own ZRQINIT (retried on further stalls) if nothing arrives
// within ioReadTimeout.
func negotiateSend(ctx context.Context, rw io.ReadWriter, fr *frameReader) error {
	h, err := fr.readHeader()
	if err == nil && h.typ == zrinit {
		return nil
	}
	if err != nil && !errors.Is(err, errReadTimeout) {
		return err
	}

	sendZRQINIT := func() error { return writeHexHeader(rw, header{typ: zrqinit}) }
	if err := sendZRQINIT(); err != nil {
		return err
	}
	_, err = waitForHeader(ctx, fr, sendZRQINIT, zrinit)
	return err
}

func sendFile(ctx context.Context, rw io.ReadWriter, fr *frameReader, path string, progress func(out.TransferProgress)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	name := filepath.Base(path)

	// "name\0size mtime mode type serial nfiles nbytes\0" -- matches real
	// lrzsz's own wire format (verified against a captured real sz session).
	// A bare "name\0size" is not reliably accepted; lrzsz's receiver expects
	// the full field set and its own NUL terminator.
	fileInfo := fmt.Sprintf("%s%c%d %o %o 0 1 %d%c", name, 0, info.Size(), info.ModTime().Unix(), info.Mode().Perm(), info.Size(), 0)
	sendZFILE := func() error {
		if err := writeBin32Header(rw, header{typ: zfile}); err != nil {
			return err
		}
		return writeSubpacket(rw, []byte(fileInfo), zcrcw)
	}
	if err := sendZFILE(); err != nil {
		return err
	}

	h, err := waitForHeader(ctx, fr, sendZFILE, zrpos, zskip)
	if err != nil {
		return err
	}
	if h.typ == zskip {
		return nil
	}
	offset := int64(h.position())
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return err
		}
	}

	// os.File.Read returns the last partial chunk with a nil error -- EOF
	// only arrives on a following, separate call -- so the true last chunk
	// can't be identified from a single read. Look one chunk ahead instead:
	// only the chunk followed by an empty EOF read gets the zcrce
	// terminator; every other chunk (including a full-buffer read exactly
	// at the file's end) gets zcrcw and waits for an ack.
	//
	// One zcrcw+ack round trip per subpacket (rather than streaming several
	// as zcrcg before a checkpoint) is deliberate: a real lrzsz rz build,
	// tested live, rejects a zcrcg-streamed run with a ZRPOS resync at the
	// following zcrcw checkpoint even though the zcrcg subpackets before it
	// land fine -- i.e. this rz doesn't actually support our streaming
	// attempt, it only silently downgrades to per-chunk resync. Matching
	// its expected per-chunk zcrcw cadence is what real interop needs;
	// resilience against a lost/corrupted ack still comes from
	// waitForHeader's timeout+retry and the ZRPOS-resume branch below.
	//
	// zcrcw closes the current ZDATA "run" (that's what distinguishes it
	// from zcrcg/zcrcq, which keep one open) -- resuming after its ack
	// requires a fresh ZDATA header at the new offset, not just the next
	// subpacket.
	sent := offset
	curBuf := make([]byte, subpacketMaxLen)
	nextBuf := make([]byte, subpacketMaxLen)

	n, rerr := f.Read(curBuf)
	if rerr != nil && rerr != io.EOF {
		return rerr
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		nextN, nextErr := 0, rerr
		if rerr != io.EOF {
			nextN, nextErr = f.Read(nextBuf)
			if nextErr != nil && nextErr != io.EOF {
				return nextErr
			}
		}
		isLast := nextN == 0 && nextErr == io.EOF

		chunkOffset := sent
		term := byte(zcrcw)
		if isLast {
			term = zcrce
		}
		sendChunk := func() error {
			if err := writeBin32Header(rw, header{typ: zdata, data: le32(uint32(chunkOffset))}); err != nil {
				return err
			}
			return writeSubpacket(rw, curBuf[:n], term)
		}
		if err := sendChunk(); err != nil {
			return err
		}
		sent += int64(n)
		if progress != nil {
			progress(out.TransferProgress{File: name, Bytes: sent, Total: info.Size()})
		}

		if term == zcrcw {
			// Poke by resending this exact frame -- a real rz has been
			// observed to go silent (no ZACK/ZRPOS at all) after a subpacket
			// it didn't process for some reason, apparently expecting a
			// retransmission rather than volunteering a NAK itself.
			resp, err := waitForHeader(ctx, fr, sendChunk, zack, zrpos)
			if err != nil {
				return err
			}
			if resp.typ == zrpos {
				// Receiver wants us to resume from a different offset
				// (e.g. it lost the subpacket we just sent) -- rewind and
				// resend from there instead of aborting the transfer.
				pos := int64(resp.position())
				if _, err := f.Seek(pos, io.SeekStart); err != nil {
					return err
				}
				sent = pos
				n, rerr = f.Read(curBuf)
				if rerr != nil && rerr != io.EOF {
					return rerr
				}
				continue
			}
		}

		if isLast {
			break
		}

		curBuf, nextBuf = nextBuf, curBuf
		n, rerr = nextN, nextErr
	}

	sendZEOF := func() error {
		return writeBin32Header(rw, header{typ: zeof, data: le32(uint32(sent))})
	}
	if err := sendZEOF(); err != nil {
		return err
	}
	// Receiver replies with a fresh ZRINIT once ready for the next file.
	_, err = waitForHeader(ctx, fr, sendZEOF, zrinit)
	return err
}
