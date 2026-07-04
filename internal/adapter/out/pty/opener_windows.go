//go:build windows

package pty

import (
	"context"
	"os"
	"os/exec"
	"sync"

	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"

	"momo-terminal/internal/core/port/out"
)

// ErrConPtyUnsupported is returned when the OS doesn't support ConPTY
// (Windows versions older than 10 1809).
var ErrConPtyUnsupported = conpty.ErrConPtyUnsupported

// Opener implements out.LocalTerminalOpener for Windows via ConPTY.
type Opener struct{}

func NewOpener() *Opener { return &Opener{} }

func (o *Opener) Open(shell string, args, env []string, cwd string, cols, rows int) (out.TerminalStream, error) {
	if shell == "" {
		shell = resolveShellWindows(exec.LookPath, os.Getenv("COMSPEC"))
	}

	cmdLine := windows.EscapeArg(shell)
	for _, a := range args {
		cmdLine += " " + windows.EscapeArg(a)
	}

	fullEnv := append(append(os.Environ(), env...), "TERM=xterm-256color", "COLORTERM=truecolor")

	cpty, err := conpty.Start(cmdLine,
		conpty.ConPtyDimensions(cols, rows),
		conpty.ConPtyWorkDir(cwd),
		conpty.ConPtyEnv(fullEnv),
	)
	if err != nil {
		return nil, err
	}

	job, err := newKillOnCloseJob(uint32(cpty.Pid()))
	if err != nil {
		_ = cpty.Close()
		return nil, err
	}

	return newWindowsStream(cpty, job), nil
}

// windowsStream implements out.TerminalStream over UserExistsError/conpty.
type windowsStream struct {
	cpty *conpty.ConPty
	job  windows.Handle

	killOnce sync.Once

	exitCh   chan struct{}
	exitCode int
	exitErr  error
	closeErr error
}

func newWindowsStream(cpty *conpty.ConPty, job windows.Handle) *windowsStream {
	s := &windowsStream{cpty: cpty, job: job, exitCh: make(chan struct{})}
	go s.run()
	return s
}

// run is the sole owner of the ConPty's Wait/Close lifecycle. ConPTY keeps
// its pipes open (and Read blocked) until ClosePseudoConsole is called, even
// after the child process has exited, so the pseudo console must not be torn
// down before Wait() confirms the process is actually gone. Close() only
// requests termination (via the job object, which also reaches grandchild
// processes); this goroutine performs the real teardown once that
// termination is confirmed, which is what unblocks a pending Read().
func (s *windowsStream) run() {
	code, err := s.cpty.Wait(context.Background())
	s.exitCode, s.exitErr = int(code), err
	s.closeErr = s.cpty.Close()
	close(s.exitCh)
}

func (s *windowsStream) Read(p []byte) (int, error)  { return s.cpty.Read(p) }
func (s *windowsStream) Write(p []byte) (int, error) { return s.cpty.Write(p) }

func (s *windowsStream) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return s.cpty.Resize(cols, rows)
}

// Close terminates the whole process tree by closing the Job Object handle
// (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE). See run() for why pseudo-console
// teardown happens separately, once exit is confirmed.
func (s *windowsStream) Close() error {
	s.killOnce.Do(func() {
		if s.job != 0 {
			_ = windows.CloseHandle(s.job)
		}
	})
	return nil
}

func (s *windowsStream) Wait() (int, error) {
	<-s.exitCh
	return s.exitCode, s.exitErr
}
