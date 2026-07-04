package sshconn

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"momo-terminal/internal/core/domain"
)

var errAgentUnavailable = errors.New("sshconn: ssh-agent is not running or unreachable")

func authMethods(host domain.Host, secret string) ([]ssh.AuthMethod, error) {
	switch host.AuthType {
	case domain.AuthPassword:
		return []ssh.AuthMethod{ssh.Password(secret)}, nil
	case domain.AuthPrivateKey:
		return privateKeyAuth(host.KeyPath, secret)
	case domain.AuthAgent:
		return agentAuth()
	default:
		return nil, fmt.Errorf("sshconn: unknown auth type %q", host.AuthType)
	}
}

func privateKeyAuth(keyPath, passphrase string) ([]ssh.AuthMethod, error) {
	pemBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("sshconn: read private key %s: %w", keyPath, err)
	}

	var signer ssh.Signer
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(pemBytes)
	}
	if err != nil {
		return nil, fmt.Errorf("sshconn: parse private key %s: %w", keyPath, err)
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

// agentAuth dials the platform's SSH agent (SSH_AUTH_SOCK on Unix, the
// OpenSSH agent named pipe on Windows via dialAgent, defined per-platform).
func agentAuth() ([]ssh.AuthMethod, error) {
	conn, err := dialAgent()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errAgentUnavailable, err)
	}
	client := agent.NewClient(conn)
	return []ssh.AuthMethod{ssh.PublicKeysCallback(client.Signers)}, nil
}
