package mcpipc

import (
	"context"
	"net"
	"time"

	"momo-shell/internal/core/port/in"
)

// handlePair runs the pairing path for one connection: it calls
// callbacks.HandlePair, but wraps it in a ctx that's canceled if the
// client disconnects while the human-approval wait is still pending. This
// is the port contract in in.MCPServerCallbacks.HandlePair's doc comment,
// mirroring sharehttp's HS-03 guard (see
// sharehttp.TestHandlePair_ClientDisconnectCancelsContext) -- a late
// approval after the caller has given up must not mint a token for an
// absent peer.
//
// A pairing client sends nothing on the connection until it receives the
// authResponse, so a background read watching for EOF/error on conn can't
// steal real request bytes.
func handlePair(conn net.Conn, callbacks in.MCPServerCallbacks, clientName string) (token string, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		var b [1]byte
		if _, err := conn.Read(b[:]); err != nil {
			cancel()
		}
	}()

	token, err = callbacks.HandlePair(ctx, clientName)

	// Unblock the watcher's Read without losing any stage-2 bytes: per the
	// protocol above the client hasn't sent anything post-response yet, so
	// forcing the pending Read to return via a deadline can't consume real
	// data.
	_ = conn.SetReadDeadline(time.Now())
	<-watcherDone
	_ = conn.SetReadDeadline(time.Time{})

	return token, err
}
