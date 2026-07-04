//go:build windows

package pty

// lookPathFunc mirrors exec.LookPath, injected so resolveShellWindows stays
// a pure, table-testable function.
type lookPathFunc func(file string) (string, error)

// resolveShellWindows picks the shell to launch when the caller doesn't
// specify one: pwsh (PowerShell 7) -> powershell.exe -> %COMSPEC% -> cmd.exe.
func resolveShellWindows(lookPath lookPathFunc, comspec string) string {
	if p, err := lookPath("pwsh"); err == nil {
		return p
	}
	if p, err := lookPath("powershell.exe"); err == nil {
		return p
	}
	if comspec != "" {
		return comspec
	}
	return "cmd.exe"
}
