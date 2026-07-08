package custodian

import (
	"errors"
	"reflect"
	"testing"

	"momo-shell/internal/core/service/mask"
)

var _ mask.SecretLister = (*Service)(nil)

// fakeHosts is a hand-rolled HostIDProvider (house style: no mock
// framework -- cf. mask_test.go's fakeSecretLister).
type fakeHosts struct {
	hostID string
	ok     bool
}

func (f *fakeHosts) HostID(sessionID string) (string, bool) {
	return f.hostID, f.ok
}

// fakeSecrets is a hand-rolled SecretReader that records the ref it was
// called with, so tests can assert the exact "host:"+id key.
type fakeSecrets struct {
	gotRef string
	secret []byte
	err    error
}

func (f *fakeSecrets) Get(ref string) ([]byte, error) {
	f.gotRef = ref
	return f.secret, f.err
}

func TestSecretsForSession_RemoteHostWithStoredSecret_ReturnsItAndUsesHostRefConvention(t *testing.T) {
	hosts := &fakeHosts{hostID: "host-42", ok: true}
	secrets := &fakeSecrets{secret: []byte("hunter2")}
	s := New(hosts, secrets)

	got := s.SecretsForSession("sess-1")

	want := []string{"hunter2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SecretsForSession() = %v, want %v", got, want)
	}
	if secrets.gotRef != "host:host-42" {
		t.Errorf("Get() called with ref %q, want %q", secrets.gotRef, "host:host-42")
	}
}

func TestSecretsForSession_LocalSession_ReturnsNil(t *testing.T) {
	hosts := &fakeHosts{hostID: "", ok: true}
	secrets := &fakeSecrets{secret: []byte("should-not-be-read")}
	s := New(hosts, secrets)

	got := s.SecretsForSession("local-sess")

	if got != nil {
		t.Errorf("SecretsForSession() = %v, want nil for a local session", got)
	}
}

func TestSecretsForSession_UnknownSession_ReturnsNil(t *testing.T) {
	hosts := &fakeHosts{ok: false}
	secrets := &fakeSecrets{secret: []byte("should-not-be-read")}
	s := New(hosts, secrets)

	got := s.SecretsForSession("never-attached")

	if got != nil {
		t.Errorf("SecretsForSession() = %v, want nil for an unknown session", got)
	}
}

func TestSecretsForSession_SecretReadError_ReturnsNil(t *testing.T) {
	hosts := &fakeHosts{hostID: "host-1", ok: true}
	secrets := &fakeSecrets{err: errors.New("not found")} // e.g. agent auth, nothing stored
	s := New(hosts, secrets)

	got := s.SecretsForSession("sess-1")

	if got != nil {
		t.Errorf("SecretsForSession() = %v, want nil when the secret store errors", got)
	}
}

func TestSecretsForSession_EmptyStoredSecret_ReturnsNil(t *testing.T) {
	hosts := &fakeHosts{hostID: "host-1", ok: true}
	secrets := &fakeSecrets{secret: []byte{}}
	s := New(hosts, secrets)

	got := s.SecretsForSession("sess-1")

	if got != nil {
		t.Errorf("SecretsForSession() = %v, want nil for an empty secret (never enter the redact set)", got)
	}
}
