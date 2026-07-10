// Package shellintegration implements out.ShellScriptBuilder: pure,
// stateless generation of the runtime shell-integration hook scripts
// verified against real shells in the S4 spike
// (.claudedocs/plan/19-mcp-s4-shellintegration-spike.md §3). No I/O and no
// session access here -- injection is performed by the core
// session/shellintegration service via its own ShellInjector back-reference.
package shellintegration

import (
	"fmt"

	"momo-shell/internal/core/port/out"
)

// Builder implements out.ShellScriptBuilder by dispatching to a per-dialect
// hook script template.
type Builder struct{}

func New() *Builder { return &Builder{} }

var _ out.ShellScriptBuilder = (*Builder)(nil)

func (b *Builder) HookScript(dialect out.ShellDialect, nonce string) ([]byte, error) {
	switch dialect {
	case out.DialectBash:
		return bashHookScript(nonce), nil
	case out.DialectPowerShell:
		return powershellHookScript(nonce), nil
	default:
		return nil, fmt.Errorf("shellintegration: unsupported dialect %q", dialect)
	}
}

func (b *Builder) ProbeScript(dialect out.ShellDialect, nonce string, vars []string) ([]byte, error) {
	switch dialect {
	case out.DialectBash:
		return bashProbeScript(nonce, vars), nil
	case out.DialectPowerShell:
		return powershellProbeScript(nonce, vars), nil
	default:
		return nil, fmt.Errorf("shellintegration: unsupported dialect %q", dialect)
	}
}

func (b *Builder) SpawnArgs(dialect out.ShellDialect, nonce string) ([]string, error) {
	if dialect != out.DialectPowerShell {
		return nil, fmt.Errorf("shellintegration: no spawn-time injection for dialect %q", dialect)
	}
	return powershellSpawnArgs(nonce), nil
}
