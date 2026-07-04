//go:build !windows

package sshconn

import (
	"errors"
	"net"
	"os"
)

var errAgentSocketNotSet = errors.New("sshconn: SSH_AUTH_SOCK is not set")

func dialAgent() (net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errAgentSocketNotSet
	}
	return net.Dial("unix", sock)
}
