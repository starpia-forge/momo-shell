//go:build windows

package pty

import (
	"errors"
	"testing"
)

func TestResolveShellWindows(t *testing.T) {
	notFound := errors.New("not found")

	cases := []struct {
		name    string
		found   map[string]string // binary name -> resolved path (absent = not found)
		comspec string
		want    string
	}{
		{
			name:  "prefers pwsh",
			found: map[string]string{"pwsh": `C:\Program Files\PowerShell\7\pwsh.exe`, "powershell.exe": `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`},
			want:  `C:\Program Files\PowerShell\7\pwsh.exe`,
		},
		{
			name:  "falls back to powershell.exe",
			found: map[string]string{"powershell.exe": `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`},
			want:  `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		},
		{
			name:    "falls back to comspec",
			found:   map[string]string{},
			comspec: `C:\Windows\system32\cmd.exe`,
			want:    `C:\Windows\system32\cmd.exe`,
		},
		{
			name:  "falls back to cmd.exe when comspec unset",
			found: map[string]string{},
			want:  "cmd.exe",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookPath := func(file string) (string, error) {
				if p, ok := tc.found[file]; ok {
					return p, nil
				}
				return "", notFound
			}

			if got := resolveShellWindows(lookPath, tc.comspec); got != tc.want {
				t.Fatalf("resolveShellWindows(...) = %q, want %q", got, tc.want)
			}
		})
	}
}
