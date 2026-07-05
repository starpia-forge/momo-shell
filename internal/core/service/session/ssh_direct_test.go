package session

import (
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

func TestCreateSSHDirect_ConnectsWithoutTouchingHostRepoOrSecrets(t *testing.T) {
	hostRepo := newFakeHostRepo() // deliberately empty -- CreateSSHDirect must never call Get
	secrets := newFakeSecretStore()
	stream := newFakeStream()
	opener := &fakeSSHOpener{stream: stream}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: secrets, KnownHosts: newFakeKnownHostsRepo(), Publisher: pub})
	defer svc.CloseAll()

	info, err := svc.CreateSSHDirect(in.SSHDirectOpts{
		Name: "web-prod-01", Address: "10.0.1.15", Port: 22,
		Username: "deploy", AuthType: domain.AuthPassword, Secret: "hunter2",
		Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("CreateSSHDirect() error = %v", err)
	}
	if info.Kind != domain.KindSSH {
		t.Fatalf("info.Kind = %v, want KindSSH", info.Kind)
	}
	if info.HostID != "" {
		t.Fatalf("info.HostID = %q, want empty (no saved host row)", info.HostID)
	}

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "running"
	})

	if opener.secretSeen() != "hunter2" {
		t.Fatalf("opener secret = %q, want the opts.Secret passed directly through", opener.secretSeen())
	}
	// hostRepo starts empty (no "h1" entry); CreateSSHDirect reaching the
	// running state at all proves it never called hostRepo.Get (that would
	// return errHostNotFound and fail CreateSSHDirect outright).
}

func TestCreateSSHDirect_UnknownHostKey_PromptsAndConnects(t *testing.T) {
	stream := newFakeStream()
	opener := &fakeSSHOpener{stream: stream, probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:new"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: newFakeHostRepo(), Secrets: newFakeSecretStore(), KnownHosts: newFakeKnownHostsRepo(), Publisher: pub})
	defer svc.CloseAll()

	info, err := svc.CreateSSHDirect(in.SSHDirectOpts{
		Name: "kim-laptop-host", Address: "10.0.1.9", Port: 22,
		Username: "kim", AuthType: domain.AuthPassword, Secret: "s3cret",
		Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("CreateSSHDirect() error = %v", err)
	}

	waitFor(t, time.Second, func() bool {
		for _, e := range pub.all() {
			if e.topic == "session:hostkey:"+info.ID {
				return true
			}
		}
		return false
	})

	if err := svc.RespondHostKey(info.ID, "once"); err != nil {
		t.Fatalf("RespondHostKey() error = %v", err)
	}

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "running"
	})
}
