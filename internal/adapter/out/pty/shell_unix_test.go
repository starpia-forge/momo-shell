//go:build !windows

package pty

import "testing"

func TestResolveShell(t *testing.T) {
	cases := []struct {
		name string
		env  string
		goos string
		want string
	}{
		{"explicit $SHELL wins", "/usr/bin/fish", "linux", "/usr/bin/fish"},
		{"macOS fallback is zsh", "", "darwin", "/bin/zsh"},
		{"linux fallback is bash", "", "linux", "/bin/bash"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveShell(tc.env, tc.goos); got != tc.want {
				t.Fatalf("resolveShell(%q, %q) = %q, want %q", tc.env, tc.goos, got, tc.want)
			}
		})
	}
}
