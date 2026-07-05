// Package sharehttp is the driving adapter (out.ShareServer) for the LAN
// host-sharing API: a peer's HTTP request is the trigger, and it calls
// back into core via in.ShareServerCallbacks -- the same relationship
// adapter/in/wails has to port/in, just triggered over the network instead
// of from the local UI.
package sharehttp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"momo-shell/internal/core/port/in"
)

// approvalWriteTimeout must exceed the share service's pairing-approval
// wait (up to 60s, see share.pairApprovalTimeout) so http.Server doesn't
// kill the connection out from under a blocked /pair handler.
const approvalWriteTimeout = 90 * time.Second

// maxPairBodyBytes caps the /pair request body -- generous for {pin, clientName}.
const maxPairBodyBytes = 4 << 10

// Option configures a Server at construction, primarily for tests that
// need to bypass real network interface enumeration.
type Option func(*Server)

// WithListenAddrs overrides RFC1918 interface enumeration with an explicit
// address list (e.g. "127.0.0.1:0" for tests -- port 0 asks the OS for an
// ephemeral port, discovered afterward via Start's return value).
func WithListenAddrs(addrs []string) Option {
	return func(s *Server) { s.fixedAddrs = addrs }
}

// Server implements out.ShareServer. TLS uses a self-signed certificate
// (see cert.go); trust is established out of band via TOFU + PIN pairing,
// not a CA.
type Server struct {
	callbacks  in.ShareServerCallbacks
	cert       tls.Certificate
	fixedAddrs []string

	mu        sync.Mutex
	listeners []net.Listener
	servers   []*http.Server
}

func New(callbacks in.ShareServerCallbacks, cert tls.Certificate, opts ...Option) *Server {
	s := &Server{callbacks: callbacks, cert: cert}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start implements out.ShareServer.
func (s *Server) Start() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.listeners) > 0 {
		return 0, errors.New("sharehttp: already started")
	}

	plainListeners, boundPort, err := s.bind()
	if err != nil {
		return 0, err
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{s.cert}}
	listeners := make([]net.Listener, 0, len(plainListeners))
	for _, l := range plainListeners {
		listeners = append(listeners, tls.NewListener(l, tlsConfig))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/info", s.handleInfo)
	mux.HandleFunc("/api/v1/pair", s.handlePair)
	mux.HandleFunc("/api/v1/hosts", s.handleHosts)

	servers := make([]*http.Server, 0, len(listeners))
	for _, l := range listeners {
		srv := &http.Server{Handler: mux, WriteTimeout: approvalWriteTimeout}
		servers = append(servers, srv)
		go srv.Serve(l)
	}

	s.listeners = listeners
	s.servers = servers
	return boundPort, nil
}

func (s *Server) bind() (listeners []net.Listener, port int, err error) {
	if s.fixedAddrs != nil {
		for _, addr := range s.fixedAddrs {
			l, err := net.Listen("tcp", addr)
			if err != nil {
				closeAll(listeners)
				return nil, 0, fmt.Errorf("sharehttp: listen %s: %w", addr, err)
			}
			listeners = append(listeners, l)
		}
	} else {
		var addrs []string
		addrs, err = privateIPv4Addrs()
		if err != nil {
			return nil, 0, err
		}
		listeners, port, err = bindAll(addrs)
		if err != nil {
			return nil, 0, err
		}
	}
	if len(listeners) == 0 {
		return nil, 0, errNoPrivateInterface
	}
	if port == 0 {
		port = listeners[0].Addr().(*net.TCPAddr).Port
	}
	return listeners, port, nil
}

// Stop implements out.ShareServer.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	servers := s.servers
	s.servers = nil
	s.listeners = nil
	s.mu.Unlock()

	var firstErr error
	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type infoResponse struct {
	Ver       int    `json:"ver"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	HostCount int    `json:"hostCount"`
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	info := s.callbacks.Info()
	writeJSON(w, http.StatusOK, infoResponse{Ver: info.Ver, ID: info.ID, Name: info.Name, HostCount: info.HostCount})
}

type pairRequestBody struct {
	PIN        string `json:"pin"`
	ClientName string `json:"clientName"`
}

type pairResponseBody struct {
	Token string `json:"token"`
}

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req pairRequestBody
	if err := json.NewDecoder(io.LimitReader(r.Body, maxPairBodyBytes)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	token, err := s.callbacks.HandlePair(r.Context(), req.PIN, req.ClientName, r.RemoteAddr)
	if err != nil {
		if errors.Is(err, in.ErrLockedOut) {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// PIN mismatch, denial, and timeout are intentionally
		// indistinguishable to the caller.
		w.WriteHeader(http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, pairResponseBody{Token: token})
}

func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	token := bearerToken(r)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	hosts, err := s.callbacks.HostsForToken(token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, hosts)
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimPrefix(auth, prefix)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
