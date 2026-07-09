//go:build windows

package mcpipc

import (
	"net"

	winio "github.com/Microsoft/go-winio"
)

// Dial connects to the current user's MCP named pipe (see
// listener_windows.go), used by cmd/momo-mcp (A5) to reach the GUI's
// mcpipc server.
func Dial() (net.Conn, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	return winio.DialPipe(pipeName(sid), nil)
}
