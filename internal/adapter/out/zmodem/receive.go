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

// Receive implements out.StreamTransfer: drives our receiver role against a
// remote sz sending one or more files.
func (e *Engine) Receive(ctx context.Context, rw io.ReadWriter, destDir string, progress func(out.TransferProgress)) ([]string, error) {
	fr := newFrameReader(rw)
	var saved []string

	for {
		if err := writeHexHeader(rw, header{typ: zrinit, data: [4]byte{0, 0, 0, canFC32}}); err != nil {
			return saved, err
		}
		h, err := readHeaderCtx(ctx, fr)
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
		default:
			// Unexpected frame at top level; ignore and re-advertise ZRINIT.
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

	if err := writeBin32Header(rw, header{typ: zrpos, data: le32(0)}); err != nil {
		return "", err
	}

	h, err := readHeaderCtx(ctx, fr)
	if err != nil {
		return "", err
	}
	if h.typ != zdata {
		return "", fmt.Errorf("zmodem: expected ZDATA, got frame type %d", h.typ)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var received int64
readLoop:
	for {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		data, term, err := fr.readSubpacket()
		if err != nil {
			return "", err
		}
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
		case zcrcw:
			// zcrcw closes the sender's current ZDATA run; it opens a fresh
			// one (a new ZDATA header) before the next chunk rather than
			// just continuing to stream subpackets.
			if err := writeBin32Header(rw, header{typ: zack, data: le32(uint32(received))}); err != nil {
				return "", err
			}
			next, err := readHeaderCtx(ctx, fr)
			if err != nil {
				return "", err
			}
			if next.typ != zdata {
				return "", fmt.Errorf("zmodem: expected ZDATA, got frame type %d", next.typ)
			}
		case zcrce:
			break readLoop
		}
	}

	eofHeader, err := readHeaderCtx(ctx, fr)
	if err != nil {
		return "", err
	}
	if eofHeader.typ != zeof {
		return "", fmt.Errorf("zmodem: expected ZEOF, got frame type %d", eofHeader.typ)
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
