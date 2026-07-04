package domain

import "testing"

func TestSession_TransitionTo(t *testing.T) {
	cases := []struct {
		name    string
		from    SessionState
		to      SessionState
		wantErr bool
	}{
		{"starting to running", StateStarting, StateRunning, false},
		{"starting to error", StateStarting, StateError, false},
		{"starting to closed", StateStarting, StateClosed, false},
		{"connecting to running", StateConnecting, StateRunning, false},
		{"connecting to error", StateConnecting, StateError, false},
		{"connecting to closed", StateConnecting, StateClosed, false},
		{"connecting to starting", StateConnecting, StateStarting, true},
		{"running to closed", StateRunning, StateClosed, false},
		{"running to error", StateRunning, StateError, false},
		{"running to starting", StateRunning, StateStarting, true},
		{"closed to running", StateClosed, StateRunning, true},
		{"closed to error", StateClosed, StateError, true},
		{"error to running", StateError, StateRunning, true},
		{"error to closed", StateError, StateClosed, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSession("id", KindLocal, "bash", 80, 24)
			s.State = tc.from

			err := s.TransitionTo(tc.to)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error transitioning %s -> %s, got nil", tc.from, tc.to)
				}
				if s.State != tc.from {
					t.Fatalf("state must be unchanged after rejected transition, got %s", s.State)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error transitioning %s -> %s: %v", tc.from, tc.to, err)
			}
			if s.State != tc.to {
				t.Fatalf("expected state %s, got %s", tc.to, s.State)
			}
		})
	}
}

func TestNewSSHSession_StartsConnecting(t *testing.T) {
	s := NewSSHSession("id", "host-1", 80, 24)

	if s.Kind != KindSSH {
		t.Fatalf("expected Kind=ssh, got %s", s.Kind)
	}
	if s.State != StateConnecting {
		t.Fatalf("expected State=connecting, got %s", s.State)
	}
	if s.HostID != "host-1" {
		t.Fatalf("expected HostID=host-1, got %s", s.HostID)
	}
}

func TestSession_Info(t *testing.T) {
	s := NewSession("abc", KindLocal, "zsh", 100, 30)

	info := s.Info()

	want := SessionInfo{ID: "abc", Kind: KindLocal, Shell: "zsh", Cols: 100, Rows: 30}
	if info != want {
		t.Fatalf("Info() = %+v, want %+v", info, want)
	}
}
