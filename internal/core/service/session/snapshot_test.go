package session

import (
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// TestSnapshot_ReturnsLiveSessionFields is Snapshot()'s data-shape contract
// (A4b's ListSessions consumes this): Kind/HostID/State must reflect each
// live session, for both local and SSH sessions.
func TestSnapshot_ReturnsLiveSessionFields(t *testing.T) {
	hostRepo := newFakeHostRepo(testSSHHost())
	knownHosts := newFakeKnownHostsRepo()
	_ = knownHosts.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:match")
	opener := &fakeSSHOpener{stream: newFakeStream(), probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:match"}
	pub := &recordingPublisher{}

	svc := New(Deps{
		LocalOpener: &fakeOpener{stream: newFakeStream()},
		SSHOpener:   opener,
		HostRepo:    hostRepo,
		Secrets:     newFakeSecretStore(),
		KnownHosts:  knownHosts,
		Publisher:   pub,
	})
	defer svc.CloseAll()

	localInfo, err := svc.CreateLocal(in.LocalOpts{Shell: "bash"})
	if err != nil {
		t.Fatalf("CreateLocal() error = %v", err)
	}
	waitFor(t, time.Second, func() bool {
		return len(stateEventsFor(pub.all(), localInfo.ID)) > 0
	})

	sshInfo, err := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("CreateSSH() error = %v", err)
	}
	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), sshInfo.ID)
		return len(states) > 0 && states[len(states)-1].State == "running"
	})

	live := svc.Snapshot()
	byID := make(map[string]domain.Session, len(live))
	for _, s := range live {
		byID[s.ID] = s
	}

	local, ok := byID[localInfo.ID]
	if !ok {
		t.Fatalf("Snapshot() missing local session %s", localInfo.ID)
	}
	if local.Kind != domain.KindLocal || local.State != domain.StateRunning {
		t.Errorf("local session = %+v, want Kind=local State=running", local)
	}

	ssh, ok := byID[sshInfo.ID]
	if !ok {
		t.Fatalf("Snapshot() missing SSH session %s", sshInfo.ID)
	}
	if ssh.Kind != domain.KindSSH || ssh.HostID != "h1" || ssh.State != domain.StateRunning {
		t.Errorf("ssh session = %+v, want Kind=ssh HostID=h1 State=running", ssh)
	}
}

// TestSnapshot_ConcurrentWithSSHConnect is a regression guard for the
// ssh.go connectSSH State-transition race (A4b): Snapshot() reads State
// under s.mu, and connectSSH's StateRunning transition must be guarded by
// the same mutex, or a concurrent Snapshot could observe a torn string
// header. This environment has no cgo/-race support, so this test cannot
// itself detect the race -- it exercises the concurrent access pattern so
// the guard is regression-tested wherever -race is available.
func TestSnapshot_ConcurrentWithSSHConnect(t *testing.T) {
	hostRepo := newFakeHostRepo(testSSHHost())
	knownHosts := newFakeKnownHostsRepo()
	_ = knownHosts.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:match")
	opener := &fakeSSHOpener{stream: newFakeStream(), probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:match"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: newFakeSecretStore(), KnownHosts: knownHosts, Publisher: pub})
	defer svc.CloseAll()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				svc.Snapshot()
			}
		}
	}()

	info, err := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})
	if err != nil {
		close(stop)
		wg.Wait()
		t.Fatalf("CreateSSH() error = %v", err)
	}
	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "running"
	})

	close(stop)
	wg.Wait()
}
