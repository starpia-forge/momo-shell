package sshconn

import (
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	keepaliveInterval    = 30 * time.Second
	keepaliveMaxFailures = 3
)

// startKeepalive pings the server periodically via the keepalive@openssh.com
// global request. After keepaliveMaxFailures consecutive failures it calls
// onLost (expected to close the connection, unblocking the stream's Read so
// the session's normal shutdown path takes over) and stops. The returned
// stop func cancels the ticker for an expected, user-initiated close.
func startKeepalive(client *ssh.Client, onLost func()) (stop func()) {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(keepaliveInterval)
		defer ticker.Stop()
		failures := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					failures++
					if failures >= keepaliveMaxFailures {
						onLost()
						return
					}
					continue
				}
				failures = 0
			}
		}
	}()
	return func() { close(done) }
}
