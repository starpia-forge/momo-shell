package aicontrol

import (
	"bytes"
	"encoding/json"
	"testing"

	"momo-shell/internal/core/domain"
)

func TestListHosts_ReturnsNameAndLabelsOnly(t *testing.T) {
	host := domain.Host{
		ID: "host-1", Name: "web-1", Address: "10.0.1.15", Port: 22,
		Username: "deploy", AuthType: domain.AuthPassword, Labels: []string{"prod", "web"},
	}
	svc, _, _ := newTestService(host)

	refs, err := svc.ListHosts("client-1")
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}
	want := []domain.HostRef{{Name: "web-1", Labels: []string{"prod", "web"}}}
	if len(refs) != 1 || refs[0].Name != want[0].Name {
		t.Fatalf("ListHosts() = %+v, want %+v", refs, want)
	}
}

func TestListSessions_OverlaysControlledFromDelegations(t *testing.T) {
	svc, _, sessions := newTestService()
	sessions.snapshotFunc = func() []domain.Session {
		return []domain.Session{
			{ID: "sess-1", Kind: domain.KindSSH, HostID: "host-1", State: domain.StateRunning},
			{ID: "sess-2", Kind: domain.KindLocal, State: domain.StateStarting},
		}
	}
	svc.mu.Lock()
	svc.delegations["sess-1"] = domain.NewDelegation("sess-1", "client-1", domain.ControlScope{})
	svc.mu.Unlock()

	views, err := svc.ListSessions("client-1")
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("ListSessions() returned %d views, want 2", len(views))
	}

	byID := make(map[string]domain.SessionView, len(views))
	for _, v := range views {
		byID[v.ID] = v
	}

	s1 := byID["sess-1"]
	if s1.Kind != domain.KindSSH || s1.HostID != "host-1" || s1.State != domain.StateRunning || !s1.Controlled {
		t.Errorf("sess-1 = %+v, want Kind=ssh HostID=host-1 State=running Controlled=true", s1)
	}
	s2 := byID["sess-2"]
	if s2.Kind != domain.KindLocal || s2.State != domain.StateStarting || s2.Controlled {
		t.Errorf("sess-2 = %+v, want Kind=local State=starting Controlled=false", s2)
	}
}

// TestCheckpointA_NoCredentialsInAIFacingResources is doc 18's checkpoint-A
// security gate: an external MCP client's list_sessions/list_hosts must
// never surface credentials, no matter how the underlying data is
// populated. It seeds a host with every credential field set and a session
// carrying that host's ID, then asserts none of those secrets appear in
// the JSON actually returned to the AI-facing caller -- a regression guard
// that fails loudly if HostRef/SessionView are ever widened to carry them.
func TestCheckpointA_NoCredentialsInAIFacingResources(t *testing.T) {
	const (
		secretAddress  = "10.55.66.77"
		secretUsername = "root-admin"
		secretKeyPath  = "/home/user/.ssh/id_super_secret"
	)
	host := domain.Host{
		ID:       "host-1",
		Name:     "prod-db",
		Address:  secretAddress,
		Port:     22,
		Username: secretUsername,
		AuthType: domain.AuthPrivateKey,
		KeyPath:  secretKeyPath,
		Labels:   []string{"prod"},
	}
	svc, _, sessions := newTestService(host)
	sessions.snapshotFunc = func() []domain.Session {
		return []domain.Session{{ID: "sess-1", Kind: domain.KindSSH, HostID: "host-1", State: domain.StateRunning}}
	}

	hostRefs, err := svc.ListHosts("client-1")
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}
	sessionViews, err := svc.ListSessions("client-1")
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	hostsJSON, err := json.Marshal(hostRefs)
	if err != nil {
		t.Fatalf("json.Marshal(hostRefs) error = %v", err)
	}
	sessionsJSON, err := json.Marshal(sessionViews)
	if err != nil {
		t.Fatalf("json.Marshal(sessionViews) error = %v", err)
	}

	for _, secret := range []string{secretAddress, secretUsername, secretKeyPath} {
		if bytes.Contains(hostsJSON, []byte(secret)) {
			t.Errorf("ListHosts() JSON contains credential material %q: %s", secret, hostsJSON)
		}
		if bytes.Contains(sessionsJSON, []byte(secret)) {
			t.Errorf("ListSessions() JSON contains credential material %q: %s", secret, sessionsJSON)
		}
	}
}
