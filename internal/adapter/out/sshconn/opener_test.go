package sshconn

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"momo-shell/internal/core/domain"
)

func passwordServerConfig(want string) *ssh.ServerConfig {
	return &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if string(password) == want {
				return nil, nil
			}
			return nil, errors.New("invalid password")
		},
	}
}

func readWithTimeout(t *testing.T, r interface{ Read([]byte) (int, error) }, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	done := make(chan struct{})
	var got int
	var err error
	go func() {
		got, err = r.Read(buf)
		close(done)
	}()
	select {
	case <-done:
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		return buf[:got]
	case <-time.After(5 * time.Second):
		t.Fatal("Read() timed out")
		return nil
	}
}

func TestOpen_PasswordAuth_Success(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	host := srv.host(domain.AuthPassword, "")

	stream, err := New().Open(host, "correct-password", trustAllVerifier, 80, 24)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()

	if _, err := stream.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if got := readWithTimeout(t, stream, 5); !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("echo = %q, want %q", got, "hello")
	}
}

func TestOpen_PasswordAuth_WrongPassword_Fails(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	host := srv.host(domain.AuthPassword, "")

	if _, err := New().Open(host, "wrong-password", trustAllVerifier, 80, 24); err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestOpen_PrivateKeyAuth_Success(t *testing.T) {
	pub, keyPath := writeTestKey(t, "")
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), pub.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("unauthorized key")
		},
	}
	srv := newTestSSHServer(t, config)
	host := srv.host(domain.AuthPrivateKey, keyPath)

	stream, err := New().Open(host, "", trustAllVerifier, 80, 24)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()

	if _, err := stream.Write([]byte("hi")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if got := readWithTimeout(t, stream, 2); !bytes.Equal(got, []byte("hi")) {
		t.Fatalf("echo = %q, want %q", got, "hi")
	}
}

func TestOpen_PrivateKeyAuth_WithPassphrase_Success(t *testing.T) {
	pub, keyPath := writeTestKey(t, "s3cr3t-passphrase")
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), pub.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("unauthorized key")
		},
	}
	srv := newTestSSHServer(t, config)
	host := srv.host(domain.AuthPrivateKey, keyPath)

	stream, err := New().Open(host, "s3cr3t-passphrase", trustAllVerifier, 80, 24)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()
}

func TestOpen_PrivateKeyAuth_WrongPassphrase_Fails(t *testing.T) {
	_, keyPath := writeTestKey(t, "s3cr3t-passphrase")
	host := domain.Host{Address: "127.0.0.1", Port: 1, Username: "tester", AuthType: domain.AuthPrivateKey, KeyPath: keyPath}

	if _, err := New().Open(host, "wrong-passphrase", trustAllVerifier, 80, 24); err == nil {
		t.Fatal("expected error for wrong passphrase")
	}
}

func TestOpen_PrivateKeyAuth_UnauthorizedKey_Fails(t *testing.T) {
	_, keyPathA := writeTestKey(t, "")
	pubB, _ := writeTestKey(t, "")
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), pubB.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("unauthorized key")
		},
	}
	srv := newTestSSHServer(t, config)
	host := srv.host(domain.AuthPrivateKey, keyPathA)

	if _, err := New().Open(host, "", trustAllVerifier, 80, 24); err == nil {
		t.Fatal("expected error connecting with a key the server doesn't recognize")
	}
}

func TestOpen_HostKeyRejected_BlocksConnection(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	host := srv.host(domain.AuthPassword, "")

	_, err := New().Open(host, "correct-password", rejectingVerifier, 80, 24)
	if err == nil {
		t.Fatal("expected error when the verifier rejects the host key")
	}
	var hkErr *hostKeyError
	if !errors.As(err, &hkErr) {
		t.Fatalf("expected a hostKeyError in the chain, got %v", err)
	}
}

func TestOpen_Wait_ReturnsServerReportedExitCode(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	srv.exitStatus = 3
	host := srv.host(domain.AuthPassword, "")

	stream, err := New().Open(host, "correct-password", trustAllVerifier, 80, 24)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()

	// A single EOT byte tells the test server to report exit status and
	// close its side, simulating the remote shell exiting on its own
	// (rather than the client calling Close() first).
	if _, err := stream.Write([]byte{0x04}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	code, err := stream.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if code != 3 {
		t.Fatalf("Wait() exit code = %d, want 3", code)
	}
}

func TestOpen_Resize_DoesNotError(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	host := srv.host(domain.AuthPassword, "")

	stream, err := New().Open(host, "correct-password", trustAllVerifier, 80, 24)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()

	if err := stream.Resize(120, 40); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}
}
