//go:build windows

package sshconn

import (
	"net"

	winio "github.com/Microsoft/go-winio"
)

// agentPipeName is the well-known named pipe exposed by the Windows
// OpenSSH agent service (Set-Service ssh-agent -StartupType Automatic).
const agentPipeName = `\\.\pipe\openssh-ssh-agent`

func dialAgent() (net.Conn, error) {
	return winio.DialPipe(agentPipeName, nil)
}
