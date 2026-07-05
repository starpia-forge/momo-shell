package transfer

import (
	"bytes"
	"testing"
)

func TestSignatureDetector_UploadInSingleChunk(t *testing.T) {
	var d signatureDetector
	before := []byte("rz waiting to receive.")
	chunk := append(append([]byte{}, before...), uploadSignature...)
	chunk = append(chunk, []byte("30303030300d8a11")...)

	pass, direction, rest := d.scan(chunk)
	if !bytes.Equal(pass, before) {
		t.Fatalf("pass = %q, want %q", pass, before)
	}
	if direction != "upload" {
		t.Fatalf("direction = %q, want upload", direction)
	}
	if !bytes.HasPrefix(rest, uploadSignature) {
		t.Fatalf("rest should start with the signature, got %q", rest)
	}
}

func TestSignatureDetector_DownloadInSingleChunk(t *testing.T) {
	var d signatureDetector
	chunk := append(append([]byte{}, []byte("rz\r")...), downloadSignature...)

	pass, direction, rest := d.scan(chunk)
	if !bytes.Equal(pass, []byte("rz\r")) {
		t.Fatalf("pass = %q, want %q", pass, "rz\r")
	}
	if direction != "download" {
		t.Fatalf("direction = %q, want download", direction)
	}
	if !bytes.HasPrefix(rest, downloadSignature) {
		t.Fatalf("rest should start with the signature, got %q", rest)
	}
}

func TestSignatureDetector_SplitAcrossChunks(t *testing.T) {
	var d signatureDetector
	before := []byte("normal shell output ")
	splitPoint := 3 // split inside the 6-byte signature

	first := append(append([]byte{}, before...), uploadSignature[:splitPoint]...)
	second := uploadSignature[splitPoint:]

	pass1, direction1, rest1 := d.scan(first)
	if direction1 != "" || rest1 != nil {
		t.Fatalf("expected no match yet on first chunk, got direction=%q rest=%q", direction1, rest1)
	}
	// The partial signature bytes must be withheld, not leaked in pass1.
	if bytes.Contains(pass1, uploadSignature[:splitPoint]) {
		t.Fatalf("pass1 leaked partial signature bytes: %q", pass1)
	}
	if !bytes.HasPrefix(before, pass1) {
		t.Fatalf("pass1 = %q should be a prefix of the pre-signature text %q", pass1, before)
	}

	pass2, direction2, rest2 := d.scan(second)
	if direction2 != "upload" {
		t.Fatalf("direction2 = %q, want upload (signature completed across chunks)", direction2)
	}
	// pass2 may legitimately contain a few bytes of "before" text that were
	// held back generically (not yet confirmed clear of a signature start)
	// on the first call -- what matters is rest2 is exactly the signature,
	// with nothing from it leaked into either pass.
	if !bytes.Equal(rest2, uploadSignature) {
		t.Fatalf("rest2 = %q, want the full signature %q", rest2, uploadSignature)
	}

	// Reassembling pass1+pass2+rest2 must reproduce the original stream
	// exactly -- no bytes lost or duplicated across the split.
	full := append(append([]byte{}, pass1...), pass2...)
	full = append(full, rest2...)
	want := append(append([]byte{}, before...), uploadSignature...)
	if !bytes.Equal(full, want) {
		t.Fatalf("reassembled stream = %q, want %q", full, want)
	}
}

func TestSignatureDetector_NoFalsePositiveOnPlainOutput(t *testing.T) {
	var d signatureDetector
	chunks := [][]byte{
		[]byte("$ ls -la\r\n"),
		[]byte("total 42\r\ndrwxr-xr-x  5 user user 4096 Jul  4 12:00 .\r\n"),
		bytes.Repeat([]byte("x"), 100),
	}
	var reassembled []byte
	for _, c := range chunks {
		pass, direction, rest := d.scan(c)
		if direction != "" {
			t.Fatalf("unexpected signature match on plain output: direction=%q", direction)
		}
		if rest != nil {
			t.Fatalf("unexpected rest on plain output: %q", rest)
		}
		reassembled = append(reassembled, pass...)
	}
	// Flush: nothing left held back forever except the trailing <=5 bytes,
	// which is expected (would be released once more data confirms it's
	// not a signature start).
	want := bytes.Join(chunks, nil)
	if !bytes.Equal(append(reassembled, d.tail...), want) {
		t.Fatalf("reassembled+tail = %q, want %q", append(reassembled, d.tail...), want)
	}
}

func TestSignatureDetector_OnlyHoldsBytesThatCouldStartASignature(t *testing.T) {
	var d signatureDetector
	// "abc" cannot be the start of either signature (both start with "**"),
	// so nothing should be withheld even though the chunk is shorter than
	// maxSignatureLen -- unlike the old flat "always hold the last 5 bytes"
	// behavior, which delayed rendering of ordinary short output for no
	// reason.
	pass, direction, rest := d.scan([]byte("abc"))
	if direction != "" || rest != nil {
		t.Fatalf("unexpected match on short chunk")
	}
	if !bytes.Equal(pass, []byte("abc")) {
		t.Fatalf("pass = %q, want everything released immediately", pass)
	}
	if len(d.tail) != 0 {
		t.Fatalf("tail = %q, want empty (nothing here could start a signature)", d.tail)
	}
}

func TestSignatureDetector_HoldsBackGenuineSignaturePrefix(t *testing.T) {
	var d signatureDetector
	// "**" is a genuine prefix of both signatures -- it must be withheld in
	// case the next chunk completes one.
	pass, direction, rest := d.scan([]byte("ok $ **"))
	if direction != "" || rest != nil {
		t.Fatalf("unexpected match on partial signature")
	}
	if !bytes.Equal(pass, []byte("ok $ ")) {
		t.Fatalf("pass = %q, want %q", pass, "ok $ ")
	}
	if !bytes.Equal(d.tail, []byte("**")) {
		t.Fatalf("tail = %q, want %q", d.tail, "**")
	}
}

func TestSignatureDetector_OneByteAtATime(t *testing.T) {
	var d signatureDetector
	before := []byte("$ sz medium.bin\r\n")
	full := append(append([]byte{}, before...), downloadSignature...)
	full = append(full, []byte("30303030300d8a11")...)

	var reassembledPass []byte
	var gotDirection string
	var gotRest []byte
	for i, b := range full {
		pass, direction, rest := d.scan([]byte{b})
		reassembledPass = append(reassembledPass, pass...)
		if direction != "" {
			gotDirection = direction
			gotRest = rest
			if i < len(before)+len(downloadSignature)-1 {
				t.Fatalf("signature detected too early at byte %d", i)
			}
			break
		}
	}
	if gotDirection != "download" {
		t.Fatalf("direction = %q, want download (byte-at-a-time delivery must still detect the signature)", gotDirection)
	}
	if !bytes.Equal(reassembledPass, before) {
		t.Fatalf("reassembled pass = %q, want %q", reassembledPass, before)
	}
	if !bytes.HasPrefix(gotRest, downloadSignature) {
		t.Fatalf("rest should start with the signature, got %q", gotRest)
	}
}
