package zmodem

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"momo-shell/internal/core/port/out"
)

// maxConsecutiveSubpacketErrors bounds how many corrupted/stalled
// subpackets in a row receiveOneFile will try to resync past (via ZRPOS)
// before giving up -- a safety net against a peer that never recovers.
const maxConsecutiveSubpacketErrors = 20

// Receive implements out.StreamTransfer: drives our receiver role against a
// remote sz sending one or more files.
func (e *Engine) Receive(ctx context.Context, rw io.ReadWriter, destDir string, progress func(out.TransferProgress)) ([]string, error) {
	fr := newFrameReader(rw)
	var saved []string

	sendZRINIT := func() error {
		return writeHexHeader(rw, header{typ: zrinit, data: [4]byte{0, 0, 0, canFC32}})
	}

	for {
		if ctx.Err() != nil {
			return saved, ctx.Err()
		}
		if err := sendZRINIT(); err != nil {
			return saved, err
		}
		h, err := waitForHeader(ctx, fr, sendZRINIT, zfile, zfin, zrqinit)
		if err != nil {
			return saved, err
		}

		switch h.typ {
		case zfile:
			path, err := receiveOneFile(ctx, rw, fr, destDir, progress)
			if err != nil {
				return saved, err
			}
			saved = append(saved, path)
		case zfin:
			if err := writeHexHeader(rw, header{typ: zfin}); err != nil {
				return saved, err
			}
			_, _ = rw.Write([]byte("OO"))
			return saved, nil
		case zrqinit:
			// Sender re-requesting our ZRINIT (e.g. it raced our first one); loop and resend.
		}
	}
}

func receiveOneFile(ctx context.Context, rw io.ReadWriter, fr *frameReader, destDir string, progress func(out.TransferProgress)) (string, error) {
	payload, _, err := fr.readSubpacket()
	if err != nil {
		return "", err
	}
	name, size := parseFileHeader(payload)
	destPath := filepath.Join(destDir, name)

	f, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var received int64
	// sendZRPOS both starts the data phase (offset 0) and is reused as the
	// resync request whenever a subpacket is lost or corrupted: since data
	// is only counted into `received` once a subpacket fully validates,
	// this is always a safe resume point to hand back to the sender.
	sendZRPOS := func() error {
		return writeBin32Header(rw, header{typ: zrpos, data: le32(uint32(received))})
	}

	if err := sendZRPOS(); err != nil {
		return "", err
	}
	if _, err := waitForHeader(ctx, fr, sendZRPOS, zdata); err != nil {
		return "", err
	}

	consecutiveErrors := 0
readLoop:
	for {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		data, term, err := fr.readSubpacket()
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors > maxConsecutiveSubpacketErrors {
				return "", fmt.Errorf("zmodem: too many consecutive subpacket errors: %w", err)
			}
			if err := sendZRPOS(); err != nil {
				return "", err
			}
			if _, err := waitForHeader(ctx, fr, sendZRPOS, zdata); err != nil {
				return "", err
			}
			continue
		}
		consecutiveErrors = 0

		if len(data) > 0 {
			if _, err := f.Write(data); err != nil {
				return "", err
			}
			received += int64(len(data))
			if progress != nil {
				progress(out.TransferProgress{File: name, Bytes: received, Total: size})
			}
		}

		switch term {
		case zcrcg:
			// Run continues; the next item is another subpacket.
		case zcrcq:
			if err := writeBin32Header(rw, header{typ: zack, data: le32(uint32(received))}); err != nil {
				return "", err
			}
		case zcrcw:
			// zcrcw closes the sender's current ZDATA run and expects a
			// ZACK before it continues -- with either a fresh ZDATA header
			// (more data) or ZEOF (this file is done).
			if err := writeBin32Header(rw, header{typ: zack, data: le32(uint32(received))}); err != nil {
				return "", err
			}
			next, err := waitForHeader(ctx, fr, sendZRPOS, zdata, zeof)
			if err != nil {
				return "", err
			}
			if next.typ == zeof {
				break readLoop
			}
		case zcrce:
			// Frame end, no ZACK expected -- ZEOF follows immediately.
			if _, err := waitForHeader(ctx, fr, nil, zeof); err != nil {
				return "", err
			}
			break readLoop
		}
	}

	return destPath, nil
}

// parseFileHeader reads the ZFILE data subpacket: "name\0size ...", where
// everything after size is optional metadata (mtime/mode/etc) we ignore.
func parseFileHeader(payload []byte) (name string, size int64) {
	parts := bytes.SplitN(payload, []byte{0}, 2)
	name = string(parts[0])
	if len(parts) > 1 {
		fmt.Sscanf(string(parts[1]), "%d", &size)
	}
	return name, size
}
