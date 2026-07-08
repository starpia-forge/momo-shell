//go:build windows

package mcpipc

import (
	"fmt"
	"net"

	winio "github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// currentUserSID returns the SID string of the current process's token
// user (e.g. "S-1-5-21-..."), used to scope the MCP pipe's DACL to this
// user alone (doc 17 §3.1 -- transport SID-gate, S3 spike).
func currentUserSID() (string, error) {
	tok := windows.GetCurrentProcessToken() // pseudo-token; no Close needed
	tu, err := tok.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("mcpipc: get token user: %w", err)
	}
	return tu.User.Sid.String(), nil
}

// pipeName returns the well-known per-user MCP named pipe path
// (doc 17 §3.1: `\\.\pipe\momo-shell-mcp-<userSID>`).
func pipeName(sid string) string {
	return `\\.\pipe\momo-shell-mcp-` + sid
}

// sddl returns a security descriptor string that grants full access to sid
// alone and denies every other SID by omission: a protected DACL (no
// inherited ACEs) with a single Allow/GENERIC_ALL ACE (doc 20 §4).
func sddl(sid string) string {
	return "D:P(A;;GA;;;" + sid + ")"
}

// listen opens the current user's MCP named pipe, restricted by SDDL to
// that user's SID alone. Remote clients are always rejected by go-winio
// regardless of the DACL (FILE_PIPE_REJECT_REMOTE_CLIENTS), so this SDDL
// only needs to gate other local users/processes.
func listen() (net.Listener, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(pipeName(sid), &winio.PipeConfig{SecurityDescriptor: sddl(sid)})
}
