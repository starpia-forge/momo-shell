package share

import (
	"os"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func TestEnableSharing_StartsServerAndGeneratesPIN(t *testing.T) {
	ts := newTestService()

	status, err := ts.svc.EnableSharing([]string{"h1", "h2"})
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
	if ts.server.startCount() != 1 {
		t.Fatalf("server.Start() called %d times, want 1", ts.server.startCount())
	}
	if ts.announcer.announced != 1 {
		t.Fatalf("announcer.Announce() called %d times, want 1", ts.announcer.announced)
	}
	if ts.announcer.lastPort != 47800 {
		t.Fatalf("announcer.Announce() port = %d, want 47800", ts.announcer.lastPort)
	}
}

func TestEnableSharing_CalledAgainRegeneratesPINWithoutRestartingServer(t *testing.T) {
	ts := newTestService()

	first, err := ts.svc.EnableSharing([]string{"h1"})
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	second, err := ts.svc.EnableSharing([]string{"h2", "h3"})
	if err != nil {
		t.Fatalf("EnableSharing() (again) error = %v", err)
	}

	if first.PIN == second.PIN {
		t.Fatal("expected PIN to be regenerated on second EnableSharing call")
	}
	if len(second.SharedHostIDs) != 2 {
		t.Fatalf("SharedHostIDs = %v, want replaced with 2 entries", second.SharedHostIDs)
	}
	if ts.server.startCount() != 1 {
		t.Fatalf("server.Start() called %d times, want 1 (no restart)", ts.server.startCount())
	}
	if ts.announcer.announced != 1 {
		t.Fatalf("announcer.Announce() called %d times, want 1 (no re-announce)", ts.announcer.announced)
	}
}

func TestDisableSharing_StopsServerAndClearsPIN(t *testing.T) {
	ts := newTestService()

	if _, err := ts.svc.EnableSharing([]string{"h1"}); err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}
	if err := ts.svc.DisableSharing(); err != nil {
		t.Fatalf("DisableSharing() error = %v", err)
	}

	status, err := ts.svc.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Enabled {
		t.Fatal("expected Enabled = false after DisableSharing")
	}
	if status.PIN != "" {
		t.Fatalf("expected PIN cleared, got %q", status.PIN)
	}
	if ts.server.stopCount() != 1 {
		t.Fatalf("server.Stop() called %d times, want 1", ts.server.stopCount())
	}
	if ts.announcer.stopCount() != 1 {
		t.Fatalf("announcer.Stop() called %d times, want 1", ts.announcer.stopCount())
	}
}

func TestDisableSharing_NoopWhenNotEnabled(t *testing.T) {
	ts := newTestService()

	if err := ts.svc.DisableSharing(); err != nil {
		t.Fatalf("DisableSharing() error = %v", err)
	}
	if ts.server.stopCount() != 0 {
		t.Fatalf("server.Stop() called %d times, want 0", ts.server.stopCount())
	}
}

func TestSetSharedHosts_UpdatesSelectionWithoutTouchingPIN(t *testing.T) {
	ts := newTestService()

	status, err := ts.svc.EnableSharing([]string{"h1"})
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	if err := ts.svc.SetSharedHosts([]string{"h2", "h3"}); err != nil {
		t.Fatalf("SetSharedHosts() error = %v", err)
	}

	after, err := ts.svc.Status()
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

func TestDeviceName_FallsBackToHostnameWhenUnset(t *testing.T) {
	ts := newTestService()

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname() error = %v", err)
	}

	name, err := ts.svc.DeviceName()
	if err != nil {
		t.Fatalf("DeviceName() error = %v", err)
	}
	if name != hostname {
		t.Fatalf("DeviceName() = %q, want hostname %q", name, hostname)
	}
}

func TestDeviceName_UsesCustomOverride(t *testing.T) {
	ts := newTestService()

	if err := ts.svc.SetDeviceName("kim-laptop"); err != nil {
		t.Fatalf("SetDeviceName() error = %v", err)
	}
	name, err := ts.svc.DeviceName()
	if err != nil {
		t.Fatalf("DeviceName() error = %v", err)
	}
	if name != "kim-laptop" {
		t.Fatalf("DeviceName() = %q, want kim-laptop", name)
	}
}

func TestSetDeviceName_RejectsTooLong(t *testing.T) {
	ts := newTestService()

	long := make([]byte, 64)
	for i := range long {
		long[i] = 'a'
	}
	if err := ts.svc.SetDeviceName(string(long)); err == nil {
		t.Fatal("expected error for device name exceeding 63 bytes")
	}
}

func TestSetDeviceName_NoAnnounceWhenDisabled(t *testing.T) {
	ts := newTestService()

	if err := ts.svc.SetDeviceName("kim-laptop"); err != nil {
		t.Fatalf("SetDeviceName() error = %v", err)
	}
	if ts.announcer.announced != 0 {
		t.Fatalf("announcer.Announce() called %d times, want 0 (sharing disabled)", ts.announcer.announced)
	}
}

func TestSetDeviceName_ReannouncesWhenEnabled(t *testing.T) {
	ts := newTestService()

	if _, err := ts.svc.EnableSharing([]string{"h1"}); err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}
	if ts.announcer.announced != 1 {
		t.Fatalf("announcer.Announce() called %d times, want 1 (initial)", ts.announcer.announced)
	}

	if err := ts.svc.SetDeviceName("kim-laptop"); err != nil {
		t.Fatalf("SetDeviceName() error = %v", err)
	}
	if ts.announcer.stopCount() != 1 {
		t.Fatalf("announcer.Stop() called %d times, want 1", ts.announcer.stopCount())
	}
	if ts.announcer.announced != 2 {
		t.Fatalf("announcer.Announce() called %d times, want 2 (re-announced)", ts.announcer.announced)
	}
	if ts.announcer.lastName != "kim-laptop" {
		t.Fatalf("announcer.Announce() lastName = %q, want kim-laptop", ts.announcer.lastName)
	}
	if ts.announcer.lastPort != 47800 {
		t.Fatalf("announcer.Announce() lastPort = %d, want unchanged 47800", ts.announcer.lastPort)
	}
}

func TestStatus_InstanceNameReflectsCustomDeviceName(t *testing.T) {
	ts := newTestService()

	if err := ts.svc.SetDeviceName("kim-laptop"); err != nil {
		t.Fatalf("SetDeviceName() error = %v", err)
	}
	status, err := ts.svc.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.InstanceName != "kim-laptop" {
		t.Fatalf("Status().InstanceName = %q, want kim-laptop", status.InstanceName)
	}
}

func TestListAndRevokeClients(t *testing.T) {
	ts := newTestService()

	if err := ts.clients.Save(domain.ShareClient{ID: "c1", Name: "kim-laptop", PairedAt: time.Now()}, "hash1"); err != nil {
		t.Fatalf("clients.Save() error = %v", err)
	}

	list, err := ts.svc.ListClients()
	if err != nil {
		t.Fatalf("ListClients() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != "c1" {
		t.Fatalf("ListClients() = %+v, want [c1]", list)
	}

	if err := ts.svc.RevokeClient("c1"); err != nil {
		t.Fatalf("RevokeClient() error = %v", err)
	}
	if ts.clients.count() != 0 {
		t.Fatalf("expected client removed, count = %d", ts.clients.count())
	}
}
