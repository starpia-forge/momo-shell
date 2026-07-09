package aicontrol

import (
	"testing"

	"momo-shell/internal/core/domain"
)

func TestListConnectionScopes_NoScopes_ReturnsEmpty(t *testing.T) {
	svc, _, _ := newTestService()

	scopes, err := svc.ListConnectionScopes()

	if err != nil {
		t.Fatalf("ListConnectionScopes() error = %v", err)
	}
	if len(scopes) != 0 {
		t.Errorf("ListConnectionScopes() = %+v, want empty", scopes)
	}
}

func TestListConnectionScopes_ReturnsCapAndLiveActiveCount(t *testing.T) {
	web1 := domain.Host{ID: "host-1", Name: "web-1"}
	web2 := domain.Host{ID: "host-2", Name: "web-2"}
	svc, pub, sessions := newTestService(web1, web2)
	scope := grantScope(t, svc, pub, "client-1", []string{"web-1", "web-2"}) // MaxConcurrent == 2

	// One auto-granted connect within the scope -- ActiveCount should reflect
	// it once the fake session layer reports it as live.
	view, err := svc.ConnectHost("client-1", "web-1")
	if err != nil {
		t.Fatalf("ConnectHost() error = %v", err)
	}
	sessions.snapshotFunc = func() []domain.Session {
		return []domain.Session{{ID: view.ID}}
	}

	scopes, err := svc.ListConnectionScopes()

	if err != nil {
		t.Fatalf("ListConnectionScopes() error = %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("ListConnectionScopes() = %+v, want 1 entry", scopes)
	}
	got := scopes[0]
	if got.ScopeID != scope.ID {
		t.Errorf("ScopeID = %q, want %q", got.ScopeID, scope.ID)
	}
	if got.ClientID != "client-1" {
		t.Errorf("ClientID = %q, want client-1", got.ClientID)
	}
	if got.MaxConcurrent != 2 {
		t.Errorf("MaxConcurrent = %d, want 2", got.MaxConcurrent)
	}
	if got.ActiveCount != 1 {
		t.Errorf("ActiveCount = %d, want 1", got.ActiveCount)
	}
	wantHosts := map[string]bool{"web-1": true, "web-2": true}
	if len(got.HostNames) != len(wantHosts) {
		t.Errorf("HostNames = %+v, want %+v", got.HostNames, wantHosts)
	}
	for _, name := range got.HostNames {
		if !wantHosts[name] {
			t.Errorf("HostNames contains unexpected %q", name)
		}
	}
}

func TestListConnectionScopes_DeadSession_ActiveCountZero(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, sessions := newTestService(host)
	grantScope(t, svc, pub, "client-1", []string{"web-1"})

	if _, err := svc.ConnectHost("client-1", "web-1"); err != nil {
		t.Fatalf("ConnectHost() error = %v", err)
	}
	sessions.snapshotFunc = func() []domain.Session { return nil } // the session has since died

	scopes, err := svc.ListConnectionScopes()

	if err != nil {
		t.Fatalf("ListConnectionScopes() error = %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("ListConnectionScopes() = %+v, want 1 entry", scopes)
	}
	if scopes[0].ActiveCount != 0 {
		t.Errorf("ActiveCount = %d, want 0 (session no longer live)", scopes[0].ActiveCount)
	}
}
