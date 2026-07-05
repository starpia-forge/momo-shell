package share

import (
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func newTestService() (*Service, *fakeHostRepo, *fakeClients, *fakeSettings, *fakeServer, *recordingPublisher) {
	hostRepo := newFakeHostRepo()
	clients := newFakeClients()
	settings := newFakeSettings()
	server := &fakeServer{startPort: 47800}
	pub := &recordingPublisher{}

	svc := New(Deps{HostRepo: hostRepo, Clients: clients, Settings: settings, Pub: pub})
	svc.SetServer(server)
	return svc, hostRepo, clients, settings, server, pub
}

func TestEnableSharing_StartsServerAndGeneratesPIN(t *testing.T) {
	svc, _, _, _, server, _ := newTestService()

	status, err := svc.EnableSharing([]string{"h1", "h2"})
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}
	if !status.Enabled {
		t.Fatal("expected Enabled = true")
	}
	if len(status.PIN) != 6 {
		t.Fatalf("expected 6-digit PIN, got %q", status.PIN)
	}
	if status.Port != 47800 {
		t.Fatalf("Port = %d, want 47800", status.Port)
	}
	if len(status.SharedHostIDs) != 2 {
		t.Fatalf("SharedHostIDs = %v, want 2 entries", status.SharedHostIDs)
	}
	if server.startCount() != 1 {
		t.Fatalf("server.Start() called %d times, want 1", server.startCount())
	}
}

func TestEnableSharing_CalledAgainRegeneratesPINWithoutRestartingServer(t *testing.T) {
	svc, _, _, _, server, _ := newTestService()

	first, err := svc.EnableSharing([]string{"h1"})
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	second, err := svc.EnableSharing([]string{"h2", "h3"})
	if err != nil {
		t.Fatalf("EnableSharing() (again) error = %v", err)
	}

	if first.PIN == second.PIN {
		t.Fatal("expected PIN to be regenerated on second EnableSharing call")
	}
	if len(second.SharedHostIDs) != 2 {
		t.Fatalf("SharedHostIDs = %v, want replaced with 2 entries", second.SharedHostIDs)
	}
	if server.startCount() != 1 {
		t.Fatalf("server.Start() called %d times, want 1 (no restart)", server.startCount())
	}
}

func TestDisableSharing_StopsServerAndClearsPIN(t *testing.T) {
	svc, _, _, _, server, _ := newTestService()

	if _, err := svc.EnableSharing([]string{"h1"}); err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}
	if err := svc.DisableSharing(); err != nil {
		t.Fatalf("DisableSharing() error = %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Enabled {
		t.Fatal("expected Enabled = false after DisableSharing")
	}
	if status.PIN != "" {
		t.Fatalf("expected PIN cleared, got %q", status.PIN)
	}
	if server.stopCount() != 1 {
		t.Fatalf("server.Stop() called %d times, want 1", server.stopCount())
	}
}

func TestDisableSharing_NoopWhenNotEnabled(t *testing.T) {
	svc, _, _, _, server, _ := newTestService()

	if err := svc.DisableSharing(); err != nil {
		t.Fatalf("DisableSharing() error = %v", err)
	}
	if server.stopCount() != 0 {
		t.Fatalf("server.Stop() called %d times, want 0", server.stopCount())
	}
}

func TestSetSharedHosts_UpdatesSelectionWithoutTouchingPIN(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()

	status, err := svc.EnableSharing([]string{"h1"})
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	if err := svc.SetSharedHosts([]string{"h2", "h3"}); err != nil {
		t.Fatalf("SetSharedHosts() error = %v", err)
	}

	after, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if after.PIN != status.PIN {
		t.Fatalf("PIN changed after SetSharedHosts: %q -> %q", status.PIN, after.PIN)
	}
	if len(after.SharedHostIDs) != 2 {
		t.Fatalf("SharedHostIDs = %v, want 2 entries", after.SharedHostIDs)
	}
}

func TestListAndRevokeClients(t *testing.T) {
	svc, _, clients, _, _, _ := newTestService()

	if err := clients.Save(domain.ShareClient{ID: "c1", Name: "kim-laptop", PairedAt: time.Now()}, "hash1"); err != nil {
		t.Fatalf("clients.Save() error = %v", err)
	}

	list, err := svc.ListClients()
	if err != nil {
		t.Fatalf("ListClients() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != "c1" {
		t.Fatalf("ListClients() = %+v, want [c1]", list)
	}

	if err := svc.RevokeClient("c1"); err != nil {
		t.Fatalf("RevokeClient() error = %v", err)
	}
	if clients.count() != 0 {
		t.Fatalf("expected client removed, count = %d", clients.count())
	}
}
