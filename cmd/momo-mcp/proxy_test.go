package main

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// pump's contract is "either direction hitting EOF ends the whole session
// and closes conn" -- these two tests each pin one direction's EOF as the
// trigger, using net.Pipe (a real, synchronous net.Conn pair) as conn so
// that Write/Read blocking gives deterministic ordering without sleeps.

func TestPump_ConnEOFEndsSessionEvenIfStdinStillOpen(t *testing.T) {
	clientConn, peerConn := net.Pipe()

	stdinR, stdinW := io.Pipe() // deliberately never closed here -- stdin has no EOF of its own
	defer stdinW.Close()
	var stdout bytes.Buffer

	done := make(chan int, 1)
	go func() { done <- pump(clientConn, stdinR, &stdout) }()

	// net.Pipe's Write blocks until the peer's Read has consumed the bytes,
	// so once this returns, stdout already contains "pong".
	if _, err := peerConn.Write([]byte("pong")); err != nil {
		t.Fatalf("peerConn.Write() error = %v", err)
	}
	peerConn.Close()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("pump() = %d, want 0", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pump() did not return after conn EOF")
	}

	if stdout.String() != "pong" {
		t.Fatalf("stdout = %q, want pong", stdout.String())
	}
}

func TestPump_StdinEOFEndsSessionAndClosesConn(t *testing.T) {
	clientConn, peerConn := net.Pipe()
	defer peerConn.Close()

	stdin := strings.NewReader("bye")
	var stdout bytes.Buffer

	recvCh := make(chan string, 1)
	go func() {
		buf := make([]byte, 3)
		io.ReadFull(peerConn, buf)
		recvCh <- string(buf)
	}()

	code := pump(clientConn, stdin, &stdout)
	if code != 0 {
		t.Fatalf("pump() = %d, want 0", code)
	}

	select {
	case got := <-recvCh:
		if got != "bye" {
			t.Fatalf("peer received = %q, want bye", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("peer never received stdin bytes")
	}

	// pump() must have closed clientConn on its way out -- the peer's next
	// Read should observe an error/EOF rather than hang.
	_ = peerConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := peerConn.Read(buf); err == nil {
		t.Fatal("expected read error on peerConn after pump() closed clientConn")
	}
}

// TestRun_UnknownFlagReturnsUsageError pins the flag.ContinueOnError exit
// code (2) without touching the real platform Dial(), which the other run()
// branches all reach through and so aren't hermetically testable here (see
// the A5 plan's manual verification step for the GUI-not-running / missing-
// token paths).
func TestRun_UnknownFlagReturnsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--bogus"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected flag package to write a usage error to stderr")
	}
}
