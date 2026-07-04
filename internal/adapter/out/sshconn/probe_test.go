package sshconn

import (
	"net"
	"strconv"
	"testing"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/out"
)

func closedPort(t *testing.T) domain.Host {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listens here anymore, so dialing it is refused

	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	return domain.Host{Address: host, Port: port, Username: "tester", AuthType: domain.AuthPassword}
}

func TestProbe_TCPUnreachable(t *testing.T) {
	result := New().Probe(closedPort(t), "anything", trustAllVerifier)
	if result.OK || result.Stage != out.StageTCP {
		t.Fatalf("Probe() = %+v, want failing StageTCP", result)
	}
}

func TestProbe_HostKeyRejected_ReturnsHandshakeStage(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	host := srv.host(domain.AuthPassword, "")

	result := New().Probe(host, "correct-password", rejectingVerifier)
	if result.OK || result.Stage != out.StageHandshake {
		t.Fatalf("Probe() = %+v, want failing StageHandshake", result)
	}
}

func TestProbe_AuthFails_ReturnsAuthStage(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	host := srv.host(domain.AuthPassword, "")

	result := New().Probe(host, "wrong-password", trustAllVerifier)
	if result.OK || result.Stage != out.StageAuth {
		t.Fatalf("Probe() = %+v, want failing StageAuth", result)
	}
}

func TestProbe_Success(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	host := srv.host(domain.AuthPassword, "")

	result := New().Probe(host, "correct-password", trustAllVerifier)
	if !result.OK || result.Stage != out.StageAuth {
		t.Fatalf("Probe() = %+v, want OK at StageAuth", result)
	}
}
