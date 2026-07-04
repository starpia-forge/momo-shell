package out

import (
	"context"
	"io"
)

// TransferProgress reports incremental byte-count progress during a
// StreamTransfer send/receive.
type TransferProgress struct {
	File         string
	Bytes, Total int64
}

// StreamTransfer runs the ZMODEM protocol over an arbitrary byte conduit.
// It never sees SSH or any other transport -- the caller supplies a
// io.ReadWriter bridged to wherever the bytes actually need to go (a shell
// stream, a pipe to lrzsz for testing, etc).
type StreamTransfer interface {
	// Send drives our sender against a remote rz. Blocks until ZFIN, a
	// protocol error, or ctx cancellation.
	Send(ctx context.Context, rw io.ReadWriter, localPaths []string, progress func(TransferProgress)) error
	// Receive drives our receiver against a remote sz, saving into destDir.
	// Blocks until ZFIN, a protocol error, or ctx cancellation.
	Receive(ctx context.Context, rw io.ReadWriter, destDir string, progress func(TransferProgress)) (saved []string, err error)
}
