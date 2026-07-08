// Package custodian provides a session's custodian secrets -- its host's
// stored SSH credential -- for mask.Service's layer-1 exact-match redaction
// (doc 17 §6.3, doc 18 E2 2부-b). It satisfies mask.SecretLister
// structurally (no mask import, cf. scrollback.Service ~ session.CommandTap).
package custodian

// HostIDProvider resolves a session to its host id ("" for local sessions),
// and whether the session is known. Satisfied by scrollback.Service.HostID.
type HostIDProvider interface {
	HostID(sessionID string) (hostID string, ok bool)
}

// SecretReader reads a stored credential by ref. Satisfied by
// out.SecretStore (we only need Get, not Set/Delete).
type SecretReader interface {
	Get(ref string) ([]byte, error)
}

// secretRefPrefix mirrors host/service.go's secretRef and session/ssh.go:40
// -- a host's credential is stored under "host:"+id (out.SecretStore doc,
// secret.go:5). No exported constant exists to reuse; kept in sync by this
// comment.
const secretRefPrefix = "host:"

// Service resolves a session to its host's stored plaintext credential, if
// any. It satisfies mask.SecretLister structurally.
type Service struct {
	hosts   HostIDProvider
	secrets SecretReader
}

func New(hosts HostIDProvider, secrets SecretReader) *Service {
	return &Service{hosts: hosts, secrets: secrets}
}

// SecretsForSession returns the plaintext credentials to redact from this
// session's output. Local sessions, unknown sessions, agent-auth hosts (no
// stored secret), read failures, and empty secrets all yield nil -- an
// empty secret must never enter the redact set (mask does ReplaceAll, so ""
// would "match" everywhere; mask.go:77 also skips "", this is belt-and-
// suspenders on the producer side).
func (s *Service) SecretsForSession(sessionID string) []string {
	hostID, ok := s.hosts.HostID(sessionID)
	if !ok || hostID == "" {
		return nil // unknown session, or local (no host credential)
	}
	secret, err := s.secrets.Get(secretRefPrefix + hostID)
	if err != nil || len(secret) == 0 {
		return nil // agent auth / no stored secret / read failure
	}
	return []string{string(secret)}
}
