package out

import (
	"io"

	"momo-terminal/internal/core/domain"
)

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

// HostKeyDecision is the user's response to an unrecognized SSH host key
// prompt.
type HostKeyDecision string

const (
	HostKeyTrust  HostKeyDecision = "trust" // trust and persist to KnownHostsRepository
	HostKeyOnce   HostKeyDecision = "once"  // proceed without persisting
	HostKeyCancel HostKeyDecision = "cancel"
)

// HostKeyVerifier is called by the SSH adapter for every host key it
// receives. The core (which owns KnownHostsRepository) decides whether the
// connection may proceed; the adapter never touches the repository
// directly, keeping adapters from referencing each other.
type HostKeyVerifier func(algo, fingerprint string) (HostKeyDecision, error)

// SSHTerminalOpener opens a TerminalStream backed by an SSH session. secret
// is the password or private key passphrase resolved from SecretStore
// ("" for agent auth).
type SSHTerminalOpener interface {
	Open(host domain.Host, secret string, verifier HostKeyVerifier, cols, rows int) (stream TerminalStream, err error)
}

// ProbeStage identifies which step of establishing an SSH connection a
// TestResult describes.
type ProbeStage string

const (
	StageTCP       ProbeStage = "tcp"
	StageHandshake ProbeStage = "handshake"
	StageAuth      ProbeStage = "auth"
)

// TestResult reports the outcome of a staged connection test, pinpointing
// which stage failed (unreachable / host key mismatch / auth rejected).
type TestResult struct {
	Stage   ProbeStage
	OK      bool
	Message string
}

// SSHProber runs a staged connection test without creating a live session.
type SSHProber interface {
	Probe(host domain.Host, secret string, verifier HostKeyVerifier) TestResult
}
