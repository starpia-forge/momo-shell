package session

import (
	"testing"
	"time"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/in"
)

func testSSHHost() domain.Host {
	return domain.Host{ID: "h1", Address: "10.0.1.15", Port: 22, Username: "deploy", AuthType: domain.AuthPassword}
}

func TestCreateSSH_KnownMatchingHostKey_ConnectsWithoutPrompt(t *testing.T) {
	knownHosts := newFakeKnownHostsRepo()
	_ = knownHosts.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:match")
	hostRepo := newFakeHostRepo(testSSHHost())
	secrets := newFakeSecretStore()
	_ = secrets.Set("host:h1", []byte("hunter2"))
	stream := newFakeStream()
	opener := &fakeSSHOpener{stream: stream, probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:match"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: secrets, KnownHosts: knownHosts, Publisher: pub})
	defer svc.CloseAll()

	info, err := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("CreateSSH() error = %v", err)
	}
	if info.Kind != domain.KindSSH || info.HostID != "h1" {
		t.Fatalf("unexpected SessionInfo: %+v", info)
	}

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "running"
	})

	states := stateEventsFor(pub.all(), info.ID)
	if states[0].State != "connecting" {
		t.Fatalf("expected first state to be connecting, got %+v", states[0])
	}

	for _, e := range pub.all() {
		if e.topic == "session:hostkey:"+info.ID {
			t.Fatal("expected no host key prompt for a known, matching key")
		}
	}

	waitFor(t, time.Second, func() bool { return len(hostRepo.touchedIDs()) > 0 })
	if opener.secretSeen() != "hunter2" {
		t.Fatalf("expected opener to receive stored secret, got %q", opener.secretSeen())
	}
}

func TestCreateSSH_MismatchedHostKey_FailsWithoutEverRunning(t *testing.T) {
	knownHosts := newFakeKnownHostsRepo()
	_ = knownHosts.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:original")
	hostRepo := newFakeHostRepo(testSSHHost())
	opener := &fakeSSHOpener{stream: newFakeStream(), probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:tampered"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: newFakeSecretStore(), KnownHosts: knownHosts, Publisher: pub})
	defer svc.CloseAll()

	info, err := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("CreateSSH() error = %v", err)
	}

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "error"
	})

	for _, st := range stateEventsFor(pub.all(), info.ID) {
		if st.State == "running" {
			t.Fatal("session must never reach running on a host key mismatch")
		}
	}

	if err := svc.Write(info.ID, []byte("x")); err != ErrSessionNotFound {
		t.Fatalf("expected session removed after failed connect, got %v", err)
	}
}

func TestCreateSSH_UnknownHostKey_TrustPersistsAndConnects(t *testing.T) {
	knownHosts := newFakeKnownHostsRepo()
	hostRepo := newFakeHostRepo(testSSHHost())
	stream := newFakeStream()
	opener := &fakeSSHOpener{stream: stream, probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:new-key"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: newFakeSecretStore(), KnownHosts: knownHosts, Publisher: pub})
	defer svc.CloseAll()

	info, err := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("CreateSSH() error = %v", err)
	}

	waitFor(t, time.Second, func() bool {
		for _, e := range pub.all() {
			if e.topic == "session:hostkey:"+info.ID {
				return true
			}
		}
		return false
	})
	var prompt HostKeyPromptPayload
	for _, e := range pub.all() {
		if e.topic == "session:hostkey:"+info.ID {
			prompt = e.payload.(HostKeyPromptPayload)
		}
	}
	if prompt.Fingerprint != "SHA256:new-key" || prompt.Algo != "ssh-ed25519" {
		t.Fatalf("unexpected prompt payload: %+v", prompt)
	}

	if err := svc.RespondHostKey(info.ID, "trust"); err != nil {
		t.Fatalf("RespondHostKey() error = %v", err)
	}

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "running"
	})

	fp, found, err := knownHosts.Get("10.0.1.15", 22, "ssh-ed25519")
	if err != nil || !found || fp != "SHA256:new-key" {
		t.Fatalf("expected trust to persist the host key, got fp=%q found=%v err=%v", fp, found, err)
	}
}

func TestCreateSSH_UnknownHostKey_OnceConnectsWithoutPersisting(t *testing.T) {
	knownHosts := newFakeKnownHostsRepo()
	hostRepo := newFakeHostRepo(testSSHHost())
	opener := &fakeSSHOpener{stream: newFakeStream(), probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:new-key"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: newFakeSecretStore(), KnownHosts: knownHosts, Publisher: pub})
	defer svc.CloseAll()

	info, _ := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})

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

	if _, found, _ := knownHosts.Get("10.0.1.15", 22, "ssh-ed25519"); found {
		t.Fatal("expected 'once' to not persist the host key")
	}
}

func TestCreateSSH_UnknownHostKey_CancelFailsConnect(t *testing.T) {
	knownHosts := newFakeKnownHostsRepo()
	hostRepo := newFakeHostRepo(testSSHHost())
	opener := &fakeSSHOpener{stream: newFakeStream(), probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:new-key"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: newFakeSecretStore(), KnownHosts: knownHosts, Publisher: pub})
	defer svc.CloseAll()

	info, _ := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})

	waitFor(t, time.Second, func() bool {
		for _, e := range pub.all() {
			if e.topic == "session:hostkey:"+info.ID {
				return true
			}
		}
		return false
	})
	if err := svc.RespondHostKey(info.ID, "cancel"); err != nil {
		t.Fatalf("RespondHostKey() error = %v", err)
	}

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "error"
	})
}

func TestCreateSSH_HostKeyPromptTimeout_FailsConnect(t *testing.T) {
	original := hostKeyPromptTimeout
	hostKeyPromptTimeout = 20 * time.Millisecond
	defer func() { hostKeyPromptTimeout = original }()

	knownHosts := newFakeKnownHostsRepo()
	hostRepo := newFakeHostRepo(testSSHHost())
	opener := &fakeSSHOpener{stream: newFakeStream(), probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:new-key"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: newFakeSecretStore(), KnownHosts: knownHosts, Publisher: pub})
	defer svc.CloseAll()

	info, _ := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "error"
	})
}

func TestCreateSSH_AgentAuth_SkipsSecretLookup(t *testing.T) {
	hostRepo := newFakeHostRepo(domain.Host{ID: "h1", Address: "10.0.1.15", Port: 22, Username: "deploy", AuthType: domain.AuthAgent})
	opener := &fakeSSHOpener{stream: newFakeStream()}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: newFakeSecretStore(), KnownHosts: newFakeKnownHostsRepo(), Publisher: pub})
	defer svc.CloseAll()

	info, err := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("CreateSSH() error = %v", err)
	}

	waitFor(t, time.Second, func() bool {
		states := stateEventsFor(pub.all(), info.ID)
		return len(states) > 0 && states[len(states)-1].State == "running"
	})
	if opener.secretSeen() != "" {
		t.Fatalf("expected empty secret for agent auth, got %q", opener.secretSeen())
	}
}

func TestCreateSSH_UnknownHostID_ReturnsErrorImmediately(t *testing.T) {
	svc := New(Deps{
		SSHOpener:  &fakeSSHOpener{stream: newFakeStream()},
		HostRepo:   newFakeHostRepo(),
		Secrets:    newFakeSecretStore(),
		KnownHosts: newFakeKnownHostsRepo(),
		Publisher:  &recordingPublisher{},
	})

	if _, err := svc.CreateSSH(in.SSHOpts{HostID: "missing"}); err == nil {
		t.Fatal("expected error for unknown host ID")
	}
}

func TestClose_DuringConnecting_CancelsAndRemovesImmediately(t *testing.T) {
	knownHosts := newFakeKnownHostsRepo() // empty -> verifier will block on the prompt
	hostRepo := newFakeHostRepo(testSSHHost())
	opener := &fakeSSHOpener{stream: newFakeStream(), probeAlgo: "ssh-ed25519", probeFingerprint: "SHA256:new-key"}
	pub := &recordingPublisher{}

	svc := New(Deps{SSHOpener: opener, HostRepo: hostRepo, Secrets: newFakeSecretStore(), KnownHosts: knownHosts, Publisher: pub})
	defer svc.CloseAll()

	info, _ := svc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})

	waitFor(t, time.Second, func() bool {
		for _, e := range pub.all() {
			if e.topic == "session:hostkey:"+info.ID {
				return true
			}
		}
		return false
	})

	if err := svc.Close(info.ID); err != nil {
		t.Fatalf("Close() during connecting error = %v", err)
	}
	if err := svc.Write(info.ID, []byte("x")); err != ErrSessionNotFound {
		t.Fatalf("expected session removed immediately after Close(), got %v", err)
	}
}

func TestRespondHostKey_UnknownSession(t *testing.T) {
	svc := New(Deps{Publisher: &recordingPublisher{}})
	if err := svc.RespondHostKey("nope", "trust"); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}
