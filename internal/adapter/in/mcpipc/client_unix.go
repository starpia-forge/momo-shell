//go:build !windows

package mcpipc

import "net"

// Dial connects to the MCP unix domain socket (see listener_unix.go), used
// by cmd/momo-mcp (A5) to reach the GUI's mcpipc server.
func Dial() (net.Conn, error) {
	path, err := socketPath()
	if err != nil {
		return nil, err
	}
	return net.Dial("unix", path)
}
