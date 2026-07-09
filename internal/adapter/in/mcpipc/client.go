package mcpipc

import (
	"errors"
	"net"
)

// ErrPairRefused is returned by Pair when the server responds with
// OK=false -- denial, timeout, or any other pairing failure collapse to
// this single sentinel (doc 20 D1: no distinguishable failure modes).
var ErrPairRefused = errors.New("mcpipc: pairing refused")

// ErrAuthRefused is returned by Authenticate when the server responds with
// OK=false (invalid or revoked token).
var ErrAuthRefused = errors.New("mcpipc: authentication refused")

// Pair runs the client side of the stage-1 "pair" handshake (see
// server.go:serveConn): it sends clientName and waits for a human to
// approve or deny the request on the GUI side, returning the issued token
// on approval. The connection never hands off to a session on this path
// (doc 20 D1) -- callers should close conn afterward and open a fresh one
// to Authenticate with the returned token.
func Pair(conn net.Conn, clientName string) (string, error) {
	if err := writeFrame(conn, authRequest{Type: frameTypePair, ClientName: clientName}); err != nil {
		return "", err
	}
	var resp authResponse
	if err := readFrame(conn, &resp); err != nil {
		return "", err
	}
	if !resp.OK {
		return "", ErrPairRefused
	}
	return resp.Token, nil
}

// Authenticate runs the client side of the stage-1 "auth" handshake,
// redeeming token for a session. On success conn becomes the stage-2 MCP
// stream end-to-end (doc 20 D3) -- the caller owns conn from this point on
// (e.g. cmd/momo-mcp pumps it against stdin/stdout).
func Authenticate(conn net.Conn, token string) error {
	if err := writeFrame(conn, authRequest{Type: frameTypeAuth, Token: token}); err != nil {
		return err
	}
	var resp authResponse
	if err := readFrame(conn, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return ErrAuthRefused
	}
	return nil
}
