package sshconn

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/crypto/ssh"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/out"
)

// testSSHServer is a minimal in-process SSH server for exercising the
// sshconn adapter without a real sshd: it accepts one connection per test,
// authenticates per the given ssh.ServerConfig, and on a pty+shell session
// echoes back whatever the client sends until the client closes its side,
// then reports exitStatus.
type testSSHServer struct {
	addr       string
	hostSigner ssh.Signer
	exitStatus uint32
}

func newTestSSHServer(t *testing.T, config *ssh.ServerConfig) *testSSHServer {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("signer from host key: %v", err)
	}
	config.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	srv := &testSSHServer{addr: ln.Addr().String(), hostSigner: signer}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.serve(conn, config)
		}
	}()

	return srv
}

func (s *testSSHServer) fingerprint() string {
	return ssh.FingerprintSHA256(s.hostSigner.PublicKey())
}

func (s *testSSHServer) host(authType domain.AuthType, keyPath string) domain.Host {
	hostPart, portPart, _ := net.SplitHostPort(s.addr)
	port, _ := strconv.Atoi(portPart)
	return domain.Host{
		ID:       "test",
		Name:     "test",
		Address:  hostPart,
		Port:     port,
		Username: "tester",
		AuthType: authType,
		KeyPath:  keyPath,
	}
}

func (s *testSSHServer) serve(conn net.Conn, config *ssh.ServerConfig) {
	sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, requests, err := newChan.Accept()
		if err != nil {
			return
		}
		go s.handleSession(channel, requests)
	}
}

func (s *testSSHServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()
	for req := range requests {
		if req.WantReply {
			req.Reply(true, nil)
		}
		if req.Type == "shell" {
			go s.echoLoop(channel)
		}
	}
}

// echoLoop echoes whatever the client writes back at it. A single 0x04
// (EOT) byte is treated as "exit now": it sends exitStatus and closes the
// channel from the server side, simulating the remote shell exiting on its
// own (as opposed to the client closing the stream first).
func (s *testSSHServer) echoLoop(channel ssh.Channel) {
	buf := make([]byte, 4096)
	for {
		n, err := channel.Read(buf)
		if n > 0 {
			data := buf[:n]
			if bytes.IndexByte(data, 0x04) >= 0 {
				s.sendExitStatus(channel)
				channel.Close()
				return
			}
			channel.Write(data)
		}
		if err != nil {
			s.sendExitStatus(channel)
			return
		}
	}
}

func (s *testSSHServer) sendExitStatus(channel ssh.Channel) {
	payload := ssh.Marshal(struct{ Status uint32 }{s.exitStatus})
	channel.SendRequest("exit-status", false, payload)
}

// trustAllVerifier accepts any host key without persisting -- the "quick
// test" style decision when there's no interactive prompt.
func trustAllVerifier(algo, fingerprint string) (out.HostKeyDecision, error) {
	return out.HostKeyOnce, nil
}

// rejectingVerifier simulates a user cancelling an unrecognized host key
// prompt.
func rejectingVerifier(algo, fingerprint string) (out.HostKeyDecision, error) {
	return out.HostKeyCancel, nil
}

// writeTestKey generates an ed25519 keypair, returns its public key (for
// the server's authorized-keys check) and the path to a PEM-encoded
// private key file (optionally passphrase-protected) for KeyPath.
func writeTestKey(t *testing.T, passphrase string) (ssh.PublicKey, string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("wrap public key: %v", err)
	}

	var block *pem.Block
	if passphrase != "" {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	} else {
		block, err = ssh.MarshalPrivateKey(priv, "")
	}
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}

	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	return sshPub, path
}
