package aicontrol

import (
	"testing"

	"momo-shell/internal/core/domain"
)

func TestListMCPClients_DelegatesToRepo(t *testing.T) {
	svc, _, clients := newCallbacksTestService()
	clients.seed(domain.MCPClient{ClientID: "client-1", Name: "peer"}, hashToken("tok"))

	got, err := svc.ListMCPClients()
	if err != nil {
		t.Fatalf("ListMCPClients() error = %v", err)
	}
	if len(got) != 1 || got[0].ClientID != "client-1" {
		t.Fatalf("ListMCPClients() = %+v, want 1 client with ID client-1", got)
	}
}

func TestRevokeMCPClient_DelegatesToRepo(t *testing.T) {
	svc, _, clients := newCallbacksTestService()
	clients.seed(domain.MCPClient{ClientID: "client-1", Name: "peer"}, hashToken("tok"))

	if err := svc.RevokeMCPClient("client-1"); err != nil {
		t.Fatalf("RevokeMCPClient() error = %v", err)
	}

	all, err := clients.List()
	if err != nil {
		t.Fatalf("clients.List() error = %v", err)
	}
	if len(all) != 1 || !all[0].Revoked {
		t.Fatalf("clients.List() = %+v, want 1 client with Revoked=true", all)
	}
}
