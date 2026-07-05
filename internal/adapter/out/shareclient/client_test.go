package shareclient

import (
	"context"
	"errors"
	"net"
	"testing"

	"momo-shell/internal/adapter/in/sharehttp"
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// fakeCallbacks is an in.ShareServerCallbacks stub for driving a real
// sharehttp.Server as this client's counterpart.
type fakeCallbacks struct {
	info      in.ShareInfo
	pairFunc  func(pin, clientName, remoteAddr string) (string, error)
	hostsFunc func(token string) ([]domain.SharedHost, error)
}

func (f *fakeCallbacks) Info() in.ShareInfo { return f.info }

func (f *fakeCallbacks) HandlePair(pin, clientName, remoteAddr string) (string, error) {
	return f.pairFunc(pin, clientName, remoteAddr)
}

func (f *fakeCallbacks) HostsForToken(token string) ([]domain.SharedHost, error) {
	return f.hostsFunc(token)
}

func startTestServer(t *testing.T, callbacks *fakeCallbacks) (address string, port int) {
	t.Helper()

	dir := t.TempDir()
	cert, err := sharehttp.LoadOrCreateCert(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateCert() error = %v", err)
	}

	srv := sharehttp.New(callbacks, cert, sharehttp.WithListenAddrs([]string{"127.0.0.1:0"}))
	boundPort, err := srv.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	return "127.0.0.1", boundPort
}

func TestInfo_TOFUAcceptsFirstCertThenPinsIt(t *testing.T) {
	address, port := startTestServer(t, &fakeCallbacks{info: in.ShareInfo{Ver: 1, ID: "inst-1", Name: "Starpia-PC", HostCount: 2}})
	client := New()

	info, fp, err := client.Info(address, port, "")
	if err != nil {
		t.Fatalf("Info() (TOFU) error = %v", err)
	}
	if info.ID != "inst-1" || info.Name != "Starpia-PC" {
		t.Fatalf("Info() = %+v", info)
	}
	if fp == "" {
		t.Fatal("expected non-empty observed fingerprint")
	}

	if _, _, err := client.Info(address, port, fp); err != nil {
		t.Fatalf("Info() (pinned, matching) error = %v", err)
	}

	_, _, err = client.Info(address, port, "0000000000000000000000000000000000000000000000000000000000000000")
	if !errors.Is(err, out.ErrPeerCertMismatch) {
		t.Fatalf("err = %v, want ErrPeerCertMismatch", err)
	}
}

func TestPair_Success(t *testing.T) {
	address, port := startTestServer(t, &fakeCallbacks{
		pairFunc: func(pin, clientName, remoteAddr string) (string, error) {
			if pin != "123456" || clientName != "kim-laptop" {
				t.Fatalf("unexpected pair args: pin=%q clientName=%q", pin, clientName)
			}
			return "the-token", nil
		},
	})
	client := New()

	token, fp, err := client.Pair(address, port, "", "123456", "kim-laptop")
	if err != nil {
		t.Fatalf("Pair() error = %v", err)
	}
	if token != "the-token" {
		t.Fatalf("token = %q, want the-token", token)
	}
	if fp == "" {
		t.Fatal("expected non-empty observed fingerprint")
	}
}

func TestPair_RejectedAndLockedOut(t *testing.T) {
	address, port := startTestServer(t, &fakeCallbacks{
		pairFunc: func(pin, clientName, remoteAddr string) (string, error) {
			if pin == "locked" {
				return "", in.ErrLockedOut
			}
			return "", in.ErrPinMismatch
		},
	})
	client := New()

	if _, _, err := client.Pair(address, port, "", "wrong", "peer"); !errors.Is(err, ErrPairRejected) {
		t.Fatalf("err = %v, want ErrPairRejected", err)
	}
	if _, _, err := client.Pair(address, port, "", "locked", "peer"); !errors.Is(err, ErrPairLockedOut) {
		t.Fatalf("err = %v, want ErrPairLockedOut", err)
	}
}

func TestFetchHosts_ValidAndInvalidToken(t *testing.T) {
	want := []domain.SharedHost{{Name: "web-prod-01", Address: "10.0.1.15", Port: 22, Username: "deploy"}}
	address, port := startTestServer(t, &fakeCallbacks{
		hostsFunc: func(token string) ([]domain.SharedHost, error) {
			if token != "valid-token" {
				return nil, in.ErrShareUnauthorized
			}
			return want, nil
		},
	})
	client := New()

	hosts, err := client.FetchHosts(address, port, "", "valid-token")
	if err != nil {
		t.Fatalf("FetchHosts() error = %v", err)
	}
	if len(hosts) != 1 || hosts[0].Name != "web-prod-01" {
		t.Fatalf("hosts = %+v", hosts)
	}

	if _, err := client.FetchHosts(address, port, "", "bogus"); !errors.Is(err, out.ErrPeerUnauthorized) {
		t.Fatalf("err = %v, want ErrPeerUnauthorized", err)
	}
}

func TestFetchHosts_CertMismatchReturnsErrPeerCertMismatch(t *testing.T) {
	address, port := startTestServer(t, &fakeCallbacks{
		hostsFunc: func(token string) ([]domain.SharedHost, error) { return nil, nil },
	})
	client := New()

	wrongFP := "0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := client.FetchHosts(address, port, wrongFP, "any-token"); !errors.Is(err, out.ErrPeerCertMismatch) {
		t.Fatalf("err = %v, want ErrPeerCertMismatch", err)
	}
}

func TestPair_CertMismatchReturnsErrPeerCertMismatch(t *testing.T) {
	address, port := startTestServer(t, &fakeCallbacks{
		pairFunc: func(pin, clientName, remoteAddr string) (string, error) { return "unused", nil },
	})
	client := New()

	wrongFP := "0000000000000000000000000000000000000000000000000000000000000000"
	if _, _, err := client.Pair(address, port, wrongFP, "123456", "peer"); !errors.Is(err, out.ErrPeerCertMismatch) {
		t.Fatalf("err = %v, want ErrPeerCertMismatch", err)
	}
}

func TestInfo_ConnectionRefusedReturnsError(t *testing.T) {
	client := New()
	// Bind an ephemeral port and immediately close it so nothing listens.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	if _, _, err := client.Info("127.0.0.1", port, ""); err == nil {
		t.Fatal("expected connection error")
	}
}
