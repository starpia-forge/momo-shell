//go:build !windows

package pty

// resolveShell picks the shell to launch when the caller doesn't specify one.
// env/goos are injected (rather than read directly) so this stays a pure,
// table-testable function.
func resolveShell(env string, goos string) string {
	if env != "" {
		return env
	}
	if goos == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/bash"
}
