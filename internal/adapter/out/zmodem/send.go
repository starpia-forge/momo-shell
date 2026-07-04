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

// readHeaderCtx makes a blocking header read cancellable. If ctx is
// canceled first, the read goroutine is left to finish on its own (the
// caller is expected to close rw on cancel, which unblocks it).
func readHeaderCtx(ctx context.Context, fr *frameReader) (header, error) {
	type result struct {
		h   header
		err error
	}
	ch := make(chan result, 1)
	go func() {
		h, err := fr.readHeader()
		ch <- result{h, err}
	}()
	select {
	case <-ctx.Done():
		return header{}, ctx.Err()
	case r := <-ch:
		return r.h, r.err
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
	if _, err := waitForHeaderType(ctx, fr, zfin); err != nil {
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
// with our own ZRQINIT as a single fallback if nothing arrives.
//
// Each attempt's readHeaderCtx call is only issued after the previous one
// has returned, never concurrently -- readHeaderCtx leaves its read
// goroutine running past a context timeout, and two overlapping reads on
// the same frameReader would race.
func negotiateSend(ctx context.Context, rw io.ReadWriter, fr *frameReader) error {
	h, err := readHeaderCtx(ctx, fr)
	if err == nil && h.typ == zrinit {
		return nil
	}
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return err
	}

	if err := writeHexHeader(rw, header{typ: zrqinit}); err != nil {
		return err
	}
	h, err = readHeaderCtx(ctx, fr)
	if err != nil {
		return err
	}
	if h.typ != zrinit {
		return fmt.Errorf("zmodem: expected ZRINIT, got frame type %d", h.typ)
	}
	return nil
}

// waitForHeaderType reads headers until one matches want, discarding
// anything else (e.g. a duplicate ZRINIT that crossed in transit with our
// own request) -- treating every response as authoritative would
// misinterpret an unrelated frame's data bytes as this one's payload.
func waitForHeaderType(ctx context.Context, fr *frameReader, want ...byte) (header, error) {
	const attempts = 10
	for i := 0; i < attempts; i++ {
		h, err := readHeaderCtx(ctx, fr)
		if err != nil {
			return header{}, err
		}
		for _, w := range want {
			if h.typ == w {
				return h, nil
			}
		}
	}
	return header{}, fmt.Errorf("zmodem: timed out waiting for header type in %v", want)
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

	if err := writeBin32Header(rw, header{typ: zfile}); err != nil {
		return err
	}
	// "name\0size mtime mode type serial nfiles nbytes\0" -- matches real
	// lrzsz's own wire format (verified against a captured real sz session).
	// A bare "name\0size" is not reliably accepted; lrzsz's receiver expects
	// the full field set and its own NUL terminator.
	fileInfo := fmt.Sprintf("%s%c%d %o %o 0 1 %d%c", name, 0, info.Size(), info.ModTime().Unix(), info.Mode().Perm(), info.Size(), 0)
	if err := writeSubpacket(rw, []byte(fileInfo), zcrcw); err != nil {
		return err
	}

	h, err := waitForHeaderType(ctx, fr, zrpos, zskip)
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
	// zcrcw closes the current ZDATA "run" (that's what distinguishes it
	// from zcrcg/zcrcq, which keep one open) -- resuming after its ack
	// requires a fresh ZDATA header at the new offset, not just the next
	// subpacket. Sending one header per chunk is simpler to reason about
	// than tracking whether a run is still open, at the cost of one extra
	// header per chunk.
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

		if err := writeBin32Header(rw, header{typ: zdata, data: le32(uint32(sent))}); err != nil {
			return err
		}
		term := byte(zcrcw)
		if isLast {
			term = zcrce
		}
		if err := writeSubpacket(rw, curBuf[:n], term); err != nil {
			return err
		}
		sent += int64(n)
		if progress != nil {
			progress(out.TransferProgress{File: name, Bytes: sent, Total: info.Size()})
		}
		if term == zcrcw {
			if _, err := waitForHeaderType(ctx, fr, zack); err != nil {
				return err
			}
		}
		if isLast {
			break
		}

		curBuf, nextBuf = nextBuf, curBuf
		n, rerr = nextN, nextErr
	}

	if err := writeBin32Header(rw, header{typ: zeof, data: le32(uint32(sent))}); err != nil {
		return err
	}
	// Receiver replies with a fresh ZRINIT once ready for the next file.
	_, err = waitForHeaderType(ctx, fr, zrinit)
	return err
}
