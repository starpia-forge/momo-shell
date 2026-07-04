package transfer

import "bytes"

// ZMODEM auto-start signatures, as sent by real lrzsz (verified against
// captured rz/sz output in the zmodem package's interop tests): both are
// the hex-header encoding of "**" ZDLE 'B' <type>, where type "01" is
// ZRINIT (remote rz announcing it's ready to receive -- we should upload)
// and type "00" is ZRQINIT (remote sz announcing it wants to send -- we
// should download).
var (
	uploadSignature   = []byte{0x2a, 0x2a, 0x18, 0x42, 0x30, 0x31} // **\x18B01 -> remote rz waiting
	downloadSignature = []byte{0x2a, 0x2a, 0x18, 0x42, 0x30, 0x30} // **\x18B00 -> remote sz starting
)

const maxSignatureLen = 6

// signatureDetector scans a session's output stream for either signature,
// carrying a small tail buffer across calls so a signature split across two
// pump-flushed chunks is never missed. Bytes are only released via scan's
// pass return once they're far enough from the end of the buffer that they
// can no longer be the start of a signature -- withholding them (rather
// than releasing eagerly and un-sending) is what keeps a detected signature
// from ever leaking a partial prefix to the terminal.
type signatureDetector struct {
	tail []byte
}

// scan returns pass (bytes confirmed clear of any signature -- safe to
// forward), and if a signature was found, direction ("upload"/"download")
// and rest (everything from the signature's first byte onward, still
// unprocessed and not part of pass).
func (d *signatureDetector) scan(chunk []byte) (pass []byte, direction string, rest []byte) {
	combined := append(d.tail, chunk...)
	d.tail = nil

	if idx := bytes.Index(combined, uploadSignature); idx >= 0 {
		return combined[:idx], "upload", combined[idx:]
	}
	if idx := bytes.Index(combined, downloadSignature); idx >= 0 {
		return combined[:idx], "download", combined[idx:]
	}

	holdLen := maxSignatureLen - 1
	if len(combined) <= holdLen {
		d.tail = combined
		return nil, "", nil
	}
	pass = combined[:len(combined)-holdLen]
	d.tail = append([]byte{}, combined[len(combined)-holdLen:]...)
	return pass, "", nil
}
