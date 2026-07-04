package sshconn

import (
	"errors"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

const probeTimeout = 5 * time.Second

// Probe runs a staged connection test (TCP reachability, host key, then
// authentication) without leaving a live session behind, so the host CRUD
// dialog can report exactly which stage failed.
func (o *Opener) Probe(host domain.Host, secret string, verifier out.HostKeyVerifier) out.TestResult {
	addr := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))

	conn, err := net.DialTimeout("tcp", addr, probeTimeout)
	if err != nil {
		return out.TestResult{Stage: out.StageTCP, OK: false, Message: err.Error()}
	}

	config, err := clientConfig(host, secret, verifier, probeTimeout)
	if err != nil {
		conn.Close()
		return out.TestResult{Stage: out.StageAuth, OK: false, Message: err.Error()}
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		var hkErr *hostKeyError
		if errors.As(err, &hkErr) {
			return out.TestResult{Stage: out.StageHandshake, OK: false, Message: hkErr.Error()}
		}
		return out.TestResult{Stage: out.StageAuth, OK: false, Message: err.Error()}
	}
	ssh.NewClient(sshConn, chans, reqs).Close()

	return out.TestResult{Stage: out.StageAuth, OK: true}
}
