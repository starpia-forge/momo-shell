package mcpipc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

// maxAuthFrameBytes caps a single authFrame's JSON payload -- generous for
// {type, clientName, token} but small enough to reject a hostile/garbled
// length prefix before allocating.
const maxAuthFrameBytes = 64 << 10

// Frame types for the stage-1 handshake.
const (
	frameTypePair = "pair"
	frameTypeAuth = "auth"
)

// authRequest is the stage-1 frame a client sends immediately after
// connecting: either "pair" (clientName, no token yet) or "auth" (an
// already-issued token, no clientName needed).
type authRequest struct {
	Type       string `json:"type"`
	ClientName string `json:"clientName,omitempty"`
	Token      string `json:"token,omitempty"`
}

// authResponse is the stage-1 reply. OK=false is returned uniformly for
// every failure (pairing denied, pairing timed out, invalid/revoked
// token, unknown frame type) -- doc 20 D1 has no lockout, so there is no
// failure mode that needs to be distinguishable to the caller. Token is
// set only on a successful pair response.
type authResponse struct {
	OK    bool   `json:"ok"`
	Token string `json:"token,omitempty"`
}

// errFrameTooLarge is returned by readFrame/writeFrame when a frame's
// length exceeds maxAuthFrameBytes.
var errFrameTooLarge = errors.New("mcpipc: frame exceeds maxAuthFrameBytes")

// readFrame reads one length-prefixed JSON frame: a uint32 big-endian byte
// count followed by that many bytes of JSON, decoded into v. Reading
// exactly the frame's bytes (io.ReadFull, no buffered reader) means an
// authenticated conn can be handed off afterward with no risk of having
// consumed stage-2 MCP bytes.
func readFrame(r io.Reader, v any) error {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n > maxAuthFrameBytes {
		return errFrameTooLarge
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}

// writeFrame writes v as one length-prefixed JSON frame (see readFrame).
func writeFrame(w io.Writer, v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(buf) > maxAuthFrameBytes {
		return errFrameTooLarge
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(buf)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}
