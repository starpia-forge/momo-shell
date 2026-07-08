package domain

import (
	"testing"
	"time"
)

func TestDelegation_TransitionTo(t *testing.T) {
	cases := []struct {
		name    string
		from    DelegationState
		to      DelegationState
		wantErr bool
	}{
		{"none to delegated", DelegNone, DelegActive, false},
		{"delegated to none", DelegActive, DelegNone, false},
		{"delegated to expired", DelegActive, DelegExpired, false},
		{"delegated to awaiting_approval", DelegActive, DelegAwaitingApprove, false},
		{"delegated to awaiting_secret", DelegActive, DelegAwaitingSecret, false},
		{"delegated to tui_handoff", DelegActive, DelegTUIHandoff, false},
		{"awaiting_approval to delegated", DelegAwaitingApprove, DelegActive, false},
		{"awaiting_approval to none (revoke)", DelegAwaitingApprove, DelegNone, false},
		{"awaiting_secret to delegated", DelegAwaitingSecret, DelegActive, false},
		{"awaiting_secret to none (revoke)", DelegAwaitingSecret, DelegNone, false},
		{"tui_handoff to delegated", DelegTUIHandoff, DelegActive, false},
		{"tui_handoff to none (revoke)", DelegTUIHandoff, DelegNone, false},
		{"expired to none", DelegExpired, DelegNone, false},
		{"none to awaiting_approval", DelegNone, DelegAwaitingApprove, true},
		{"none to expired", DelegNone, DelegExpired, true},
		{"expired to delegated", DelegExpired, DelegActive, true},
		{"awaiting_approval to expired", DelegAwaitingApprove, DelegExpired, true},
		{"awaiting_secret to awaiting_approval", DelegAwaitingSecret, DelegAwaitingApprove, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDelegation("sess-1", "client-1", ControlScope{})
			d.State = tc.from

			err := d.TransitionTo(tc.to)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error transitioning %s -> %s, got nil", tc.from, tc.to)
				}
				if d.State != tc.from {
					t.Fatalf("state must be unchanged after rejected transition, got %s", d.State)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error transitioning %s -> %s: %v", tc.from, tc.to, err)
			}
			if d.State != tc.to {
				t.Fatalf("expected state %s, got %s", tc.to, d.State)
			}
		})
	}
}

func TestNewDelegation_StartsNone(t *testing.T) {
	scope := ControlScope{HostOnly: true, PathPrefix: "/srv", ReadOnly: true}
	d := NewDelegation("sess-1", "client-1", scope)

	if d.State != DelegNone {
		t.Fatalf("expected State=none, got %s", d.State)
	}
	if d.SessionID != "sess-1" || d.ClientID != "client-1" {
		t.Fatalf("expected SessionID/ClientID to be set, got %+v", d)
	}
	if d.Scope != scope {
		t.Fatalf("expected Scope=%+v, got %+v", scope, d.Scope)
	}
}

func TestDelegation_IsExpired(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	timeout := 5 * time.Minute

	cases := []struct {
		name      string
		lastActAt time.Time
		want      bool
	}{
		{"well within timeout", now.Add(-1 * time.Minute), false},
		{"exactly at timeout", now.Add(-timeout), true},
		{"past timeout", now.Add(-10 * time.Minute), true},
		{"never active (zero value)", time.Time{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDelegation("sess-1", "client-1", ControlScope{})
			d.LastActAt = tc.lastActAt

			if got := d.IsExpired(now, timeout); got != tc.want {
				t.Errorf("IsExpired() = %v, want %v", got, tc.want)
			}
		})
	}
}
