package sshconn

import (
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func dialTestServer(t *testing.T, srv *testSSHServer, password string) *ssh.Client {
	t.Helper()
	config := &ssh.ClientConfig{
		User:            "tester",
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}
	client, err := ssh.Dial("tcp", srv.addr, config)
	if err != nil {
		t.Fatalf("dial test server: %v", err)
	}
	return client
}

func TestStartKeepalive_DeadConnectionTriggersOnLostAfterThreeFailures(t *testing.T) {
	original := keepaliveInterval
	keepaliveInterval = 20 * time.Millisecond
	defer func() { keepaliveInterval = original }()

	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	client := dialTestServer(t, srv, "correct-password")
	client.Close() // simulate a dropped connection: every SendRequest now fails

	lost := make(chan struct{})
	stop := startKeepalive(client, func() { close(lost) })
	defer stop()

	select {
	case <-lost:
	case <-time.After(2 * time.Second):
		t.Fatal("expected onLost to fire after repeated keepalive failures on a dead connection")
	}
}

func TestStartKeepalive_HealthyConnectionNeverCallsOnLost(t *testing.T) {
	original := keepaliveInterval
	keepaliveInterval = 20 * time.Millisecond
	defer func() { keepaliveInterval = original }()

	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	client := dialTestServer(t, srv, "correct-password")
	defer client.Close()

	lost := make(chan struct{})
	stop := startKeepalive(client, func() { close(lost) })
	defer stop()

	select {
	case <-lost:
		t.Fatal("onLost fired for a healthy connection")
	case <-time.After(200 * time.Millisecond):
		// expected: no failures over ~10 healthy keepalive rounds
	}
}

func TestStartKeepalive_StopPreventsOnLost(t *testing.T) {
	original := keepaliveInterval
	keepaliveInterval = 20 * time.Millisecond
	defer func() { keepaliveInterval = original }()

	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	client := dialTestServer(t, srv, "correct-password")
	client.Close()

	lost := make(chan struct{})
	stop := startKeepalive(client, func() { close(lost) })
	stop() // cancel before any failures accumulate

	select {
	case <-lost:
		t.Fatal("onLost fired after stop() was called")
	case <-time.After(150 * time.Millisecond):
	}
}
