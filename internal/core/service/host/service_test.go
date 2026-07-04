package host

import (
	"testing"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

func validInput() in.HostInput {
	return in.HostInput{
		Name:     "web-prod-01",
		Address:  "10.0.1.15",
		Port:     22,
		Labels:   []string{"prod"},
		Username: "deploy",
		AuthType: domain.AuthPassword,
	}
}

func TestSaveHost_CreatesWithGeneratedID(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, newFakeSecretStore(), newFakeKnownHostsRepo(), &fakeProber{})

	h, err := svc.SaveHost(validInput())
	if err != nil {
		t.Fatalf("SaveHost() error = %v", err)
	}
	if h.ID == "" {
		t.Fatal("expected generated ID, got empty string")
	}
	if h.CreatedAt.IsZero() || h.UpdatedAt.IsZero() {
		t.Fatal("expected CreatedAt/UpdatedAt to be set")
	}
}

func TestSaveHost_UpdatePreservesCreatedAt(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, newFakeSecretStore(), newFakeKnownHostsRepo(), &fakeProber{})

	created, err := svc.SaveHost(validInput())
	if err != nil {
		t.Fatalf("initial SaveHost() error = %v", err)
	}

	update := validInput()
	update.ID = created.ID
	update.Name = "web-prod-01-renamed"

	updated, err := svc.SaveHost(update)
	if err != nil {
		t.Fatalf("update SaveHost() error = %v", err)
	}
	if updated.Name != "web-prod-01-renamed" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("expected CreatedAt preserved, got %v want %v", updated.CreatedAt, created.CreatedAt)
	}
}

func TestSaveHost_UpdateUnknownID(t *testing.T) {
	svc := New(newFakeRepo(), newFakeSecretStore(), newFakeKnownHostsRepo(), &fakeProber{})

	input := validInput()
	input.ID = "does-not-exist"

	if _, err := svc.SaveHost(input); err == nil {
		t.Fatal("expected error updating unknown host ID")
	}
}

func TestSaveHost_ValidationRejected(t *testing.T) {
	svc := New(newFakeRepo(), newFakeSecretStore(), newFakeKnownHostsRepo(), &fakeProber{})

	input := validInput()
	input.Name = ""

	if _, err := svc.SaveHost(input); err != domain.ErrHostNameRequired {
		t.Fatalf("expected ErrHostNameRequired, got %v", err)
	}
}

func TestSaveHost_PrivateKeyRequiresKeyPath(t *testing.T) {
	svc := New(newFakeRepo(), newFakeSecretStore(), newFakeKnownHostsRepo(), &fakeProber{})

	input := validInput()
	input.AuthType = domain.AuthPrivateKey

	if _, err := svc.SaveHost(input); err != domain.ErrHostKeyPathRequired {
		t.Fatalf("expected ErrHostKeyPathRequired, got %v", err)
	}
}

func TestDeleteHost_RemovesHostAndSecret(t *testing.T) {
	repo := newFakeRepo()
	secrets := newFakeSecretStore()
	svc := New(repo, secrets, newFakeKnownHostsRepo(), &fakeProber{})

	h, _ := svc.SaveHost(validInput())
	if err := svc.SetHostSecret(h.ID, "hunter2"); err != nil {
		t.Fatalf("SetHostSecret() error = %v", err)
	}
	if !secrets.has(secretRef(h.ID)) {
		t.Fatal("expected secret to be stored before delete")
	}

	if err := svc.DeleteHost(h.ID); err != nil {
		t.Fatalf("DeleteHost() error = %v", err)
	}

	if _, err := repo.Get(h.ID); err == nil {
		t.Fatal("expected host to be removed from repo")
	}
	if secrets.has(secretRef(h.ID)) {
		t.Fatal("expected secret to be removed alongside host")
	}
}

func TestSetHostSecret_UnknownHost(t *testing.T) {
	svc := New(newFakeRepo(), newFakeSecretStore(), newFakeKnownHostsRepo(), &fakeProber{})

	if err := svc.SetHostSecret("nope", "secret"); err == nil {
		t.Fatal("expected error setting secret for unknown host")
	}
}

func TestTestConnection_PassesStoredSecretToProber(t *testing.T) {
	repo := newFakeRepo()
	secrets := newFakeSecretStore()
	prober := &fakeProber{result: out.TestResult{Stage: out.StageAuth, OK: true}}
	svc := New(repo, secrets, newFakeKnownHostsRepo(), prober)

	h, _ := svc.SaveHost(validInput())
	_ = svc.SetHostSecret(h.ID, "hunter2")

	result, err := svc.TestConnection(h.ID)
	if err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if !result.OK || result.Stage != string(out.StageAuth) {
		t.Fatalf("expected passthrough result, got %+v", result)
	}
	if prober.lastSecret != "hunter2" {
		t.Fatalf("expected prober to receive stored secret, got %q", prober.lastSecret)
	}
	if prober.lastHost.ID != h.ID {
		t.Fatalf("expected prober to receive the saved host, got %+v", prober.lastHost)
	}
}

func TestTestConnection_AgentAuthSkipsSecretLookup(t *testing.T) {
	repo := newFakeRepo()
	prober := &fakeProber{result: out.TestResult{Stage: out.StageAuth, OK: true}}
	svc := New(repo, newFakeSecretStore(), newFakeKnownHostsRepo(), prober)

	input := validInput()
	input.AuthType = domain.AuthAgent
	h, _ := svc.SaveHost(input)

	if _, err := svc.TestConnection(h.ID); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if prober.lastSecret != "" {
		t.Fatalf("expected empty secret for agent auth, got %q", prober.lastSecret)
	}
}

func TestTestConnection_VerifierAcceptsKnownMatchAndUnknownHostKey(t *testing.T) {
	repo := newFakeRepo()
	knownHosts := newFakeKnownHostsRepo()
	_ = knownHosts.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:matches")
	prober := &fakeProber{result: out.TestResult{Stage: out.StageAuth, OK: true}}
	svc := New(repo, newFakeSecretStore(), knownHosts, prober)

	h, _ := svc.SaveHost(validInput())
	if _, err := svc.TestConnection(h.ID); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}

	decision, err := prober.lastVerifier("ssh-ed25519", "SHA256:matches")
	if err != nil || decision != out.HostKeyOnce {
		t.Fatalf("expected known-matching key to be accepted, got decision=%v err=%v", decision, err)
	}

	decision, err = prober.lastVerifier("rsa-sha2-512", "SHA256:never-seen-before")
	if err != nil || decision != out.HostKeyOnce {
		t.Fatalf("expected unknown algo to be accepted (no one to prompt), got decision=%v err=%v", decision, err)
	}
}

func TestTestConnection_VerifierRejectsMismatchedHostKey(t *testing.T) {
	repo := newFakeRepo()
	knownHosts := newFakeKnownHostsRepo()
	_ = knownHosts.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:original")
	prober := &fakeProber{result: out.TestResult{Stage: out.StageAuth, OK: true}}
	svc := New(repo, newFakeSecretStore(), knownHosts, prober)

	h, _ := svc.SaveHost(validInput())
	if _, err := svc.TestConnection(h.ID); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}

	if _, err := prober.lastVerifier("ssh-ed25519", "SHA256:tampered"); err == nil {
		t.Fatal("expected mismatched host key to be rejected")
	}
}

func TestListLabels_DedupesAcrossHosts(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, newFakeSecretStore(), newFakeKnownHostsRepo(), &fakeProber{})

	a := validInput()
	a.Labels = []string{"prod", "web"}
	_, _ = svc.SaveHost(a)

	b := validInput()
	b.Name = "web-prod-02"
	b.Labels = []string{"prod"}
	_, _ = svc.SaveHost(b)

	labels, err := svc.ListLabels()
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 distinct labels, got %v", labels)
	}
}
