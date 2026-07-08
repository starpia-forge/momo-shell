package out

// ShellDialect identifies which shell-integration hook script to emit.
// DialectUnknown means the session's shell couldn't be matched to a
// supported dialect -- callers should skip injection entirely rather than
// guess (see session/shellintegration for the safe-degradation contract).
type ShellDialect string

const (
	DialectBash       ShellDialect = "bash"
	DialectPowerShell ShellDialect = "powershell"
	DialectUnknown    ShellDialect = ""
)

// ShellScriptBuilder is the driven port for generating shell-integration
// injection scripts. Implementations are pure (no I/O, no session access) --
// the core service decides when/whether to write the returned bytes to a
// session via its own ShellInjector back-reference.
type ShellScriptBuilder interface {
	// HookScript returns the runtime-injected line that installs OSC133
	// prompt/command/exit hooks for dialect, wrapped so its own installation
	// (echo + execution) can be distinguished from real shell output: the
	// returned bytes begin by emitting a bootstrap-start sentinel containing
	// nonce and end by emitting a "hookinstalled;<nonce>" marker once the
	// hooks are live.
	HookScript(dialect ShellDialect, nonce string) ([]byte, error)
}
