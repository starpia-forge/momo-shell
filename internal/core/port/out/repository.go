package out

import "momo-terminal/internal/core/domain"

// HostRepository persists Host records. Secret material is never passed
// through this port -- see SecretStore.
type HostRepository interface {
	List() ([]domain.Host, error)
	Get(id string) (domain.Host, error)
	Save(h domain.Host) (domain.Host, error)
	Delete(id string) error
	TouchConnected(id string) error
	ListLabels() ([]string, error)
}

// KnownHostsRepository persists SSH host key fingerprints keyed by
// address+port+algorithm, mirroring OpenSSH's known_hosts semantics.
type KnownHostsRepository interface {
	Get(address string, port int, algo string) (fingerprint string, found bool, err error)
	Put(address string, port int, algo string, fingerprint string) error
	Delete(address string, port int, algo string) error
}
