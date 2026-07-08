package mcpipc

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"momo-shell/internal/core/port/in"
)

// fakeCallbacks is an in.MCPServerCallbacks stub for driving the server
// under test without a real aicontrol service (A4).
type fakeCallbacks struct {
	pairFunc func(ctx context.Context, clientName string) (string, error)
	authFunc func(token string) (string, error)
}

func (f *fakeCallbacks) HandlePair(ctx context.Context, clientName string) (string, error) {
	return f.pairFunc(ctx, clientName)
}

func (f *fakeCallbacks) AuthClient(token string) (string, error) {
	return f.authFunc(token)
}

var _ in.MCPServerCallbacks = (*fakeCallbacks)(nil)

// fakeSessionHandler records the clientID it was handed off and echoes
// bytes on the connection until the peer closes it (or the server force-
// closes it from Stop), so tests can confirm the raw conn genuinely
// becomes the stage-2 stream.
type fakeSessionHandler struct {
	sessions chan string
}

func newFakeSessionHandler() *fakeSessionHandler {
	return &fakeSessionHandler{sessions: make(chan string, 8)}
}

func (f *fakeSessionHandler) HandleSession(clientID string, conn net.Conn) {
	f.sessions <- clientID
	_, _ = io.Copy(conn, conn)
}

var _ SessionHandler = (*fakeSessionHandler)(nil)

func startTestServer(t *testing.T, callbacks *fakeCallbacks, handler SessionHandler) net.Addr {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	srv := New(callbacks, handler, WithListener(l))
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Stop(ctx)
	})

	return l.Addr()
}

func sendFrame(t *testing.T, conn net.Conn, v any) {
	t.Helper()
	if err := writeFrame(conn, v); err != nil {
		t.Fatalf("writeFrame() error = %v", err)
	}
}

func recvFrame(t *testing.T, conn net.Conn) authResponse {
	t.Helper()
	var resp authResponse
	if err := readFrame(conn, &resp); err != nil {
		t.Fatalf("readFrame() error = %v", err)
	}
	return resp
}

func TestPair_ApprovedIssuesTokenWithoutSessionHandoff(t *testing.T) {
	callbacks := &fakeCallbacks{
		pairFunc: func(ctx context.Context, clientName string) (string, error) {
			if clientName != "claude-desktop" {
				t.Fatalf("unexpected clientName = %q", clientName)
			}
			return "the-token", nil
		},
	}
	handler := newFakeSessionHandler()
	addr := startTestServer(t, callbacks, handler)

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	sendFrame(t, conn, authRequest{Type: frameTypePair, ClientName: "claude-desktop"})
	resp := recvFrame(t, conn)
	if !resp.OK || resp.Token != "the-token" {
		t.Fatalf("resp = %+v, want OK=true Token=the-token", resp)
	}

	select {
	case clientID := <-handler.sessions:
		t.Fatalf("pair path must not hand off to SessionHandler, got clientID=%q", clientID)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestPair_FailuresAreIndistinguishable confirms doc 20 D1: with no PIN
// and no lockout, every pairing failure (denied, timed out) collapses to
// the same OK=false response.
func TestPair_FailuresAreIndistinguishable(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"denied", in.ErrPairDenied},
		{"timeout", in.ErrPairTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callbacks := &fakeCallbacks{
				pairFunc: func(ctx context.Context, clientName string) (string, error) {
					return "", tc.err
				},
			}
			addr := startTestServer(t, callbacks, newFakeSessionHandler())

			conn, err := net.Dial("tcp", addr.String())
			if err != nil {
				t.Fatalf("Dial() error = %v", err)
			}
			defer conn.Close()

			sendFrame(t, conn, authRequest{Type: frameTypePair, ClientName: "peer"})
			resp := recvFrame(t, conn)
			if resp.OK || resp.Token != "" {
				t.Fatalf("resp = %+v, want OK=false empty Token", resp)
			}
		})
	}
}

// TestPair_DisconnectDuringApprovalCancelsContext is this cycle's key
// contract test (user-confirmed disconnect guard): a client that gives up
// mid-approval-wait must not leave HandlePair blocked past its own
// lifetime, mirroring sharehttp's
// TestHandlePair_ClientDisconnectCancelsContext (HS-03).
func TestPair_DisconnectDuringApprovalCancelsContext(t *testing.T) {
	ctxCanceled := make(chan struct{})
	callbacks := &fakeCallbacks{
		pairFunc: func(ctx context.Context, clientName string) (string, error) {
			<-ctx.Done()
			close(ctxCanceled)
			return "", ctx.Err()
		},
	}
	addr := startTestServer(t, callbacks, newFakeSessionHandler())

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}

	sendFrame(t, conn, authRequest{Type: frameTypePair, ClientName: "peer"})

	// Give the server a moment to enter the approval wait, then simulate
	// the client giving up before any response arrives.
	time.Sleep(50 * time.Millisecond)
	conn.Close()

	select {
	case <-ctxCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server to observe the disconnect")
	}
}

func TestAuth_ValidTokenHandsOffSession(t *testing.T) {
	callbacks := &fakeCallbacks{
		authFunc: func(token string) (string, error) {
			if token != "the-token" {
				t.Fatalf("unexpected token = %q", token)
			}
			return "client-1", nil
		},
	}
	handler := newFakeSessionHandler()
	addr := startTestServer(t, callbacks, handler)

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	sendFrame(t, conn, authRequest{Type: frameTypeAuth, Token: "the-token"})
	resp := recvFrame(t, conn)
	if !resp.OK {
		t.Fatalf("resp = %+v, want OK=true", resp)
	}

	select {
	case clientID := <-handler.sessions:
		if clientID != "client-1" {
			t.Fatalf("clientID = %q, want client-1", clientID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session handoff")
	}

	// Confirm the raw conn is genuinely handed to the session -- stage-2
	// bytes flow through untouched (the fake handler echoes).
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("echo = %q, want ping", buf)
	}
}

func TestAuth_InvalidTokenIsRejectedWithoutHandoff(t *testing.T) {
	callbacks := &fakeCallbacks{
		authFunc: func(token string) (string, error) {
			return "", in.ErrUnauthorized
		},
	}
	handler := newFakeSessionHandler()
	addr := startTestServer(t, callbacks, handler)

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	sendFrame(t, conn, authRequest{Type: frameTypeAuth, Token: "bogus"})
	resp := recvFrame(t, conn)
	if resp.OK {
		t.Fatalf("resp = %+v, want OK=false", resp)
	}

	select {
	case clientID := <-handler.sessions:
		t.Fatalf("must not hand off session on auth failure, got clientID=%q", clientID)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServeConn_UnknownFrameTypeRejected(t *testing.T) {
	callbacks := &fakeCallbacks{}
	addr := startTestServer(t, callbacks, newFakeSessionHandler())

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	sendFrame(t, conn, authRequest{Type: "bogus"})
	resp := recvFrame(t, conn)
	if resp.OK {
		t.Fatalf("resp = %+v, want OK=false", resp)
	}
}

// TestStop_ClosesActiveSessionConnections confirms Stop force-closes an
// in-flight MCP session conn (rather than blocking on it indefinitely),
// so no goroutine is leaked when the GUI shuts down mid-session.
func TestStop_ClosesActiveSessionConnections(t *testing.T) {
	callbacks := &fakeCallbacks{
		authFunc: func(token string) (string, error) { return "client-1", nil },
	}
	handler := newFakeSessionHandler()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	srv := New(callbacks, handler, WithListener(l))
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	sendFrame(t, conn, authRequest{Type: frameTypeAuth, Token: "tok"})
	recvFrame(t, conn)

	select {
	case <-handler.sessions:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session handoff")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v (goroutine may not have unblocked in time)", err)
	}

	// The server force-closed the session conn from its side; our end
	// should observe an error rather than hang.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected read error after Stop force-closed the session conn")
	}
}
