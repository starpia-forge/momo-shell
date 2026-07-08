// Package mcpipc is the driving adapter (S3's secured named-pipe/unix-
// socket transport) for MCP client connections: a client's stage-1
// authFrame is the trigger, and it calls back into core via
// in.MCPServerCallbacks -- the same relationship adapter/in/wails has to
// port/in, just triggered over the local IPC transport instead of from the
// local UI (mirrors adapter/in/sharehttp for the LAN-sharing surface).
//
// Wire protocol (see frame.go): each connection begins with one
// length-prefixed JSON authFrame, either "pair" (obtain a token via human
// approval) or "auth" (redeem a token for a session). A pairing
// connection ends once the token is issued or refused -- a fresh
// connection then authenticates with that token to obtain a session. An
// authenticated connection is handed to a SessionHandler (A6) as a raw
// stream, becoming a single MCP session end to end (doc 20 D3) -- mcpipc
// parses nothing past stage 1.
package mcpipc

import (
	"context"
	"errors"
	"net"
	"sync"

	"momo-shell/internal/core/port/in"
)

// SessionHandler takes ownership of an authenticated connection bound to
// clientID and hosts the per-client MCP session on it (doc 20 D3: A6
// wraps conn in the SDK's IOTransport and runs a per-client mcp.Server
// instance). A2's tests supply a fake; A6 supplies the real
// implementation. HandleSession returns when the session ends; the
// server closes conn afterward (or forces it closed from Stop).
type SessionHandler interface {
	HandleSession(clientID string, conn net.Conn)
}

// errAlreadyStarted is returned by Start if the server is already running.
var errAlreadyStarted = errors.New("mcpipc: already started")

// Option configures a Server at construction, primarily for tests that
// need to inject an in-memory/loopback listener instead of the real
// platform transport (S3's listen()).
type Option func(*Server)

// WithListener injects a listener instead of calling the platform's
// listen() (S3), mirroring sharehttp's WithListenAddrs test seam.
func WithListener(l net.Listener) Option {
	return func(s *Server) { s.listener = l }
}

// Server is the mcpipc driving adapter: it accepts connections on the S3
// transport, runs the stage-1 authFrame handshake against callbacks, and
// on successful auth hands the raw connection to handler for the stage-2
// MCP session.
type Server struct {
	callbacks in.MCPServerCallbacks
	handler   SessionHandler

	mu       sync.Mutex
	listener net.Listener
	started  bool
	closed   bool
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
}

func New(callbacks in.MCPServerCallbacks, handler SessionHandler, opts ...Option) *Server {
	s := &Server{callbacks: callbacks, handler: handler, conns: make(map[net.Conn]struct{})}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start opens the transport (S3's listen(), unless a test injected one via
// WithListener) and begins accepting connections in the background.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errAlreadyStarted
	}

	if s.listener == nil {
		l, err := listen()
		if err != nil {
			return err
		}
		s.listener = l
	}

	s.started = true
	go s.acceptLoop(s.listener)
	return nil
}

func (s *Server) acceptLoop(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}

		s.mu.Lock()
		s.conns[conn] = struct{}{}
		s.mu.Unlock()

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.conns, conn)
				s.mu.Unlock()
			}()
			s.serveConn(conn)
		}()
	}
}

// Stop closes the listener (no further Accepts), force-closes every
// active connection (so a blocked pairing wait or in-flight MCP session
// unblocks instead of leaking its goroutine), and waits for those
// goroutines to finish, up to ctx's deadline.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	l := s.listener
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()

	var firstErr error
	if l != nil {
		if err := l.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, c := range conns {
		_ = c.Close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return firstErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// serveConn runs the stage-1 handshake for one connection and, on
// successful auth, hands it off for the stage-2 MCP session.
func (s *Server) serveConn(conn net.Conn) {
	defer conn.Close()

	var req authRequest
	if err := readFrame(conn, &req); err != nil {
		return
	}

	switch req.Type {
	case frameTypePair:
		token, err := handlePair(conn, s.callbacks, req.ClientName)
		if err != nil {
			_ = writeFrame(conn, authResponse{OK: false})
			return
		}
		_ = writeFrame(conn, authResponse{OK: true, Token: token})

	case frameTypeAuth:
		clientID, err := s.callbacks.AuthClient(req.Token)
		if err != nil {
			_ = writeFrame(conn, authResponse{OK: false})
			return
		}
		if err := writeFrame(conn, authResponse{OK: true}); err != nil {
			return
		}
		s.handler.HandleSession(clientID, conn)

	default:
		_ = writeFrame(conn, authResponse{OK: false})
	}
}
