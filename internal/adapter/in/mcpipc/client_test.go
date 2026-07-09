package mcpipc

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"momo-shell/internal/core/port/in"
)

// These tests drive the exported client helpers (Pair/Authenticate)
// against the same startTestServer harness server_test.go uses, so the
// client codec is exercised against the real serveConn rather than
// re-deriving expectations independently.

func TestPair_ClientHelperReturnsIssuedToken(t *testing.T) {
	callbacks := &fakeCallbacks{
		pairFunc: func(ctx context.Context, clientName string) (string, error) {
			if clientName != "claude-desktop" {
				t.Fatalf("unexpected clientName = %q", clientName)
			}
			return "the-token", nil
		},
	}
	addr := startTestServer(t, callbacks, newFakeSessionHandler())

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("net.Dial() error = %v", err)
	}
	defer conn.Close()

	token, err := Pair(conn, "claude-desktop")
	if err != nil {
		t.Fatalf("Pair() error = %v", err)
	}
	if token != "the-token" {
		t.Fatalf("token = %q, want the-token", token)
	}
}

func TestPair_ClientHelperRefused(t *testing.T) {
	callbacks := &fakeCallbacks{
		pairFunc: func(ctx context.Context, clientName string) (string, error) {
			return "", in.ErrPairDenied
		},
	}
	addr := startTestServer(t, callbacks, newFakeSessionHandler())

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("net.Dial() error = %v", err)
	}
	defer conn.Close()

	if _, err := Pair(conn, "peer"); !errors.Is(err, ErrPairRefused) {
		t.Fatalf("Pair() error = %v, want ErrPairRefused", err)
	}
}

func TestAuthenticate_ClientHelperHandsOffStage2Stream(t *testing.T) {
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
		t.Fatalf("net.Dial() error = %v", err)
	}
	defer conn.Close()

	if err := Authenticate(conn, "the-token"); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	select {
	case clientID := <-handler.sessions:
		if clientID != "client-1" {
			t.Fatalf("clientID = %q, want client-1", clientID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session handoff")
	}

	// Confirm conn is genuinely the raw stage-2 stream after Authenticate
	// returns (the fake handler echoes).
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

func TestAuthenticate_ClientHelperRefused(t *testing.T) {
	callbacks := &fakeCallbacks{
		authFunc: func(token string) (string, error) {
			return "", in.ErrUnauthorized
		},
	}
	addr := startTestServer(t, callbacks, newFakeSessionHandler())

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("net.Dial() error = %v", err)
	}
	defer conn.Close()

	if err := Authenticate(conn, "bogus"); !errors.Is(err, ErrAuthRefused) {
		t.Fatalf("Authenticate() error = %v, want ErrAuthRefused", err)
	}
}
