package domain

import (
	"errors"
	"time"
)

// AuthType is the SSH authentication method configured for a Host.
type AuthType string

const (
	AuthPassword   AuthType = "password"
	AuthPrivateKey AuthType = "privateKey"
	AuthAgent      AuthType = "agent"
)

// Host is a saved SSH connection target. Secret material (password, key
// passphrase) is never stored here -- it lives behind the SecretStore port,
// referenced only by Host.ID.
type Host struct {
	ID              string
	Name            string
	Address         string
	Port            int
	Labels          []string
	Username        string
	AuthType        AuthType
	KeyPath         string
	Source          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastConnectedAt *time.Time
}

var (
	ErrHostNameRequired     = errors.New("domain: host name is required")
	ErrHostAddressRequired  = errors.New("domain: host address is required")
	ErrHostUsernameRequired = errors.New("domain: host username is required")
	ErrHostPortInvalid      = errors.New("domain: host port must be between 1 and 65535")
	ErrHostAuthTypeInvalid  = errors.New("domain: host auth type must be password, privateKey, or agent")
	ErrHostKeyPathRequired  = errors.New("domain: key path is required for privateKey auth")
)

// Validate checks the structural invariants a Host must satisfy regardless
// of storage backend. It does not check secret material -- that's the
// caller's responsibility via SecretStore.
func (h Host) Validate() error {
	if h.Name == "" {
		return ErrHostNameRequired
	}
	if h.Address == "" {
		return ErrHostAddressRequired
	}
	if h.Username == "" {
		return ErrHostUsernameRequired
	}
	if h.Port < 1 || h.Port > 65535 {
		return ErrHostPortInvalid
	}
	switch h.AuthType {
	case AuthPassword, AuthAgent:
	case AuthPrivateKey:
		if h.KeyPath == "" {
			return ErrHostKeyPathRequired
		}
	default:
		return ErrHostAuthTypeInvalid
	}
	return nil
}
