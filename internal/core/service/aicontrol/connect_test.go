package aicontrol

import (
	"errors"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// withShrunkApprovalTimeout shrinks connectApprovalTimeout for the duration
// of a test so timeout tests don't wait the full 60s (share/provider_test.go
// mirror).
func withShrunkApprovalTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	original := connectApprovalTimeout
	connectApprovalTimeout = d
	t.Cleanup(func() { connectApprovalTimeout = original })
}

func waitForConnectApprovalRequest(t *testing.T, pub *recordingPublisher) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range pub.all() {
			if payload, ok := e.payload.(connectApprovalPayload); ok {
				return payload.RequestID
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for mcp:connect-approval event")
	return ""
}

func newTestService(hosts ...domain.Host) (*Service, *recordingPublisher, *fakeSessionCreator) {
	pub := &recordingPublisher{}
	sessions := &fakeSessionCreator{
		createFunc: func(opts in.SSHOpts) (domain.SessionInfo, error) {
			return domain.SessionInfo{ID: "sess-1", Kind: domain.KindSSH, HostID: opts.HostID}, nil
		},
	}
	svc := New(Deps{Hosts: newFakeHostRepo(hosts...), Sessions: sessions, Publisher: pub})
	return svc, pub, sessions
}

func TestConnectHost_ApproveOpensSessionWithDelegation(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, _ := newTestService(host)

	var wg sync.WaitGroup
	var view domain.SessionView
	var connectErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		view, connectErr = svc.ConnectHost("client-1", "web-1")
	}()

	requestID := waitForConnectApprovalRequest(t, pub)
	if err := svc.RespondConnectApproval(requestID, true); err != nil {
		t.Fatalf("RespondConnectApproval() error = %v", err)
	}
	wg.Wait()

	if connectErr != nil {
		t.Fatalf("ConnectHost() error = %v", connectErr)
	}
	want := domain.SessionView{ID: "sess-1", Kind: domain.KindSSH, HostID: "host-1", State: domain.StateConnecting, Controlled: true}
	if view != want {
		t.Fatalf("ConnectHost() = %+v, want %+v", view, want)
	}

	svc.mu.Lock()
	deleg, ok := svc.delegations["sess-1"]
	svc.mu.Unlock()
	if !ok {
		t.Fatal("expected a delegation to be recorded for sess-1")
	}
	if deleg.State != domain.DelegActive {
		t.Errorf("delegation.State = %v, want %v", deleg.State, domain.DelegActive)
	}
	if deleg.ClientID != "client-1" {
		t.Errorf("delegation.ClientID = %q, want %q", deleg.ClientID, "client-1")
	}
}

func TestConnectHost_DenyReturnsErrConnectDenied(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, _ := newTestService(host)

	var wg sync.WaitGroup
	var connectErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, connectErr = svc.ConnectHost("client-1", "web-1")
	}()

	requestID := waitForConnectApprovalRequest(t, pub)
	if err := svc.RespondConnectApproval(requestID, false); err != nil {
		t.Fatalf("RespondConnectApproval() error = %v", err)
	}
	wg.Wait()

	if !errors.Is(connectErr, ErrConnectDenied) {
		t.Fatalf("ConnectHost() error = %v, want ErrConnectDenied", connectErr)
	}
}

func TestConnectHost_UnknownHostIsRejected(t *testing.T) {
	svc, _, _ := newTestService() // no saved hosts

	_, err := svc.ConnectHost("client-1", "does-not-exist")

	if !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("ConnectHost() error = %v, want ErrHostNotFound", err)
	}
}

func TestConnectHost_ApprovalTimeout(t *testing.T) {
	withShrunkApprovalTimeout(t, 20*time.Millisecond)
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, _, _ := newTestService(host)

	_, err := svc.ConnectHost("client-1", "web-1")

	if !errors.Is(err, ErrConnectApprovalTimeout) {
		t.Fatalf("ConnectHost() error = %v, want ErrConnectApprovalTimeout", err)
	}
}

func TestRespondConnectApproval_UnknownRequestID(t *testing.T) {
	svc, _, _ := newTestService()

	err := svc.RespondConnectApproval("no-such-request", true)

	if !errors.Is(err, ErrNoPendingConnectApproval) {
		t.Fatalf("RespondConnectApproval() error = %v, want ErrNoPendingConnectApproval", err)
	}
}

// TestRespondConnectApproval_AlreadyAnswered seeds the pending map directly
// (rather than going through ConnectHost) so the second answer racing
// against awaitConnectApproval's cleanup goroutine isn't a factor -- with
// nothing draining the channel, the first send fills its buffer-of-1 and
// the second deterministically hits the "already answered" branch.
func TestRespondConnectApproval_AlreadyAnswered(t *testing.T) {
	svc, _, _ := newTestService()
	requestID := "req-1"
	svc.mu.Lock()
	svc.pending[requestID] = make(chan bool, 1)
	svc.mu.Unlock()

	if err := svc.RespondConnectApproval(requestID, true); err != nil {
		t.Fatalf("first RespondConnectApproval() error = %v", err)
	}
	if err := svc.RespondConnectApproval(requestID, true); !errors.Is(err, ErrConnectApprovalAlreadyAnswered) {
		t.Fatalf("second RespondConnectApproval() error = %v, want ErrConnectApprovalAlreadyAnswered", err)
	}
}

func TestConnectHost_CreateSSHFailurePropagates(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, sessions := newTestService(host)
	wantErr := errors.New("dial failed")
	sessions.createFunc = func(opts in.SSHOpts) (domain.SessionInfo, error) {
		return domain.SessionInfo{}, wantErr
	}

	var wg sync.WaitGroup
	var connectErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, connectErr = svc.ConnectHost("client-1", "web-1")
	}()

	requestID := waitForConnectApprovalRequest(t, pub)
	if err := svc.RespondConnectApproval(requestID, true); err != nil {
		t.Fatalf("RespondConnectApproval() error = %v", err)
	}
	wg.Wait()

	if !errors.Is(connectErr, wantErr) {
		t.Fatalf("ConnectHost() error = %v, want %v", connectErr, wantErr)
	}
}
