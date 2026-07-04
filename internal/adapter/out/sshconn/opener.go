// Package sshconn implements the SSH-backed out.TerminalStream (via
// out.SSHTerminalOpener) and the staged connection test (out.SSHProber) on
// top of golang.org/x/crypto/ssh. The core session service treats this
// exactly like the local PTY adapter -- both just implement TerminalStream.
package sshconn

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

const dialTimeout = 10 * time.Second

// Opener implements both out.SSHTerminalOpener and out.SSHProber.
type Opener struct{}

var (
	_ out.SSHTerminalOpener = (*Opener)(nil)
	_ out.SSHProber         = (*Opener)(nil)
)

func New() *Opener {
	return &Opener{}
}

// hostKeyError distinguishes a HostKeyVerifier rejection from an ordinary
// authentication failure, both of which x/crypto/ssh otherwise reports as
// opaque handshake errors -- Probe needs the distinction to tell the user
// "호스트키 불일치" apart from "인증 실패".
type hostKeyError struct{ err error }

func (e *hostKeyError) Error() string { return e.err.Error() }
func (e *hostKeyError) Unwrap() error { return e.err }

func hostKeyCallback(verifier out.HostKeyVerifier) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		decision, err := verifier(key.Type(), ssh.FingerprintSHA256(key))
		if err != nil {
			return &hostKeyError{err: err}
		}
		if decision == out.HostKeyCancel {
			return &hostKeyError{err: errors.New("sshconn: host key rejected")}
		}
		return nil
	}
}

func clientConfig(host domain.Host, secret string, verifier out.HostKeyVerifier, timeout time.Duration) (*ssh.ClientConfig, error) {
	methods, err := authMethods(host, secret)
	if err != nil {
		return nil, err
	}
	return &ssh.ClientConfig{
		User:            host.Username,
		Auth:            methods,
		HostKeyCallback: hostKeyCallback(verifier),
		Timeout:         timeout,
	}, nil
}

func (o *Opener) Open(host domain.Host, secret string, verifier out.HostKeyVerifier, cols, rows int) (out.TerminalStream, error) {
	config, err := clientConfig(host, secret, verifier, dialTimeout)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("sshconn: connect to %s: %w", addr, err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("sshconn: open session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("sshconn: request pty: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("sshconn: stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("sshconn: stdout pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("sshconn: start shell: %w", err)
	}

	stop := startKeepalive(client, func() { client.Close() })
	return newStream(client, session, stdin, stdout, stop), nil
}
