package out

import "io"

// TerminalStream is the driven port implemented by any terminal transport
// (local PTY/ConPTY now, SSH in a later phase). The session service treats
// all implementations identically.
type TerminalStream interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	// Wait blocks until the underlying process exits and returns its exit code.
	Wait() (exitCode int, err error)
}

// LocalTerminalOpener opens a TerminalStream backed by a local shell process.
type LocalTerminalOpener interface {
	Open(shell string, args, env []string, cwd string, cols, rows int) (TerminalStream, error)
}
