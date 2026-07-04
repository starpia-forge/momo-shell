package sshconn

import (
	"errors"
	"io"
	"sync"

	"golang.org/x/crypto/ssh"

	"momo-terminal/internal/core/port/out"
)

// sshStream implements out.TerminalStream over an SSH session with an
// allocated pty. Remote stdout and stderr are already merged onto the pty
// by the server, so Read only needs the stdout channel.
type sshStream struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader

	stopKeepalive func()
	closeOnce     sync.Once
}

var _ out.TerminalStream = (*sshStream)(nil)

func newStream(client *ssh.Client, session *ssh.Session, stdin io.WriteCloser, stdout io.Reader, stopKeepalive func()) *sshStream {
	return &sshStream{client: client, session: session, stdin: stdin, stdout: stdout, stopKeepalive: stopKeepalive}
}

func (s *sshStream) Read(p []byte) (int, error) {
	return s.stdout.Read(p)
}

func (s *sshStream) Write(p []byte) (int, error) {
	return s.stdin.Write(p)
}

func (s *sshStream) Resize(cols, rows int) error {
	return s.session.WindowChange(rows, cols)
}

func (s *sshStream) Wait() (int, error) {
	err := s.session.Wait()
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// Close tears down the session and the underlying connection, unblocking
// any pending Read. Safe to call more than once (e.g. once explicitly and
// once from the pump's post-drain cleanup).
func (s *sshStream) Close() error {
	s.closeOnce.Do(func() {
		if s.stopKeepalive != nil {
			s.stopKeepalive()
		}
		s.session.Close()
		s.client.Close()
	})
	return nil
}
