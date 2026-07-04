//go:build !windows

package pty

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"momo-terminal/internal/core/port/out"
)

// Opener implements out.LocalTerminalOpener for Unix-like systems via creack/pty.
type Opener struct{}

func NewOpener() *Opener { return &Opener{} }

func (o *Opener) Open(shell string, args, env []string, cwd string, cols, rows int) (out.TerminalStream, string, error) {
	if shell == "" {
		shell = resolveShell(os.Getenv("SHELL"), runtime.GOOS)
	}

	cmd := exec.Command(shell, args...)
	cmd.Dir = cwd
	cmd.Env = append(append(os.Environ(), env...), "TERM=xterm-256color", "COLORTERM=truecolor")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, "", err
	}

	return &unixStream{ptmx: ptmx, cmd: cmd}, shell, nil
}

// unixStream implements out.TerminalStream over a creack/pty master file.
type unixStream struct {
	ptmx      *os.File
	cmd       *exec.Cmd
	closeOnce sync.Once
	closeErr  error
}

func (s *unixStream) Read(p []byte) (int, error)  { return s.ptmx.Read(p) }
func (s *unixStream) Write(p []byte) (int, error) { return s.ptmx.Write(p) }

func (s *unixStream) Resize(cols, rows int) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (s *unixStream) Wait() (int, error) {
	err := s.cmd.Wait()
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}

// Close closes the PTY master (unblocking any pending Read) and asks the
// shell to exit, escalating to SIGKILL if it hasn't exited after a grace period.
func (s *unixStream) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.ptmx.Close()
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Signal(syscall.SIGHUP)
			go func(proc *os.Process) {
				time.Sleep(3 * time.Second)
				_ = proc.Kill()
			}(s.cmd.Process)
		}
	})
	return s.closeErr
}
