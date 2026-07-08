//go:build !windows

package mcpipc

import (
	"net"
	"os"
	"path/filepath"
)

// socketPath returns the MCP unix domain socket path: $XDG_RUNTIME_DIR/
// momo-shell/mcp.sock, falling back to ~/.momo-shell/mcp.sock if
// $XDG_RUNTIME_DIR is unset (doc 17 §3.1).
func socketPath() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".momo-shell")
	} else {
		dir = filepath.Join(dir, "momo-shell")
	}
	return filepath.Join(dir, "mcp.sock"), nil
}

// listen opens the MCP unix domain socket, gated by a 0700 containing
// directory (closes the TOCTOU window between socket creation and the
// chmod below, since other users can't even enter the directory) and a
// 0600 socket file (defense in depth) -- doc 17 §3.1's same-user-only
// requirement translated to filesystem permissions.
func listen() (net.Listener, error) {
	path, err := socketPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	// A prior crashed process can leave the socket file behind; net.Listen
	// refuses to bind an existing path.
	_ = os.Remove(path)

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}
