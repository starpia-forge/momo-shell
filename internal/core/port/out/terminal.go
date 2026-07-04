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
// It returns the resolved shell name (e.g. when shell == "" and the adapter
// auto-detected one) alongside the stream, so callers can report what's
// actually running instead of the empty/auto-detect request they made.
type LocalTerminalOpener interface {
	Open(shell string, args, env []string, cwd string, cols, rows int) (stream TerminalStream, resolvedShell string, err error)
}
