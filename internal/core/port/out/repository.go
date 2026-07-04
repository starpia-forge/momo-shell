package out

import "momo-shell/internal/core/domain"

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

// HistoryRepository persists captured command-history entries.
type HistoryRepository interface {
	// Append inserts a new entry and returns it with its assigned ID.
	Append(hostID *string, command string, executedAt int64) (domain.HistoryEntry, error)
	// TouchLast bumps an existing entry's executedAt, used to update a
	// consecutive-duplicate command in place instead of inserting a new row.
	TouchLast(id int64, executedAt int64) error
	// LastForHost returns the most recent entry for hostID (nil = local),
	// used to detect consecutive-duplicate commands.
	LastForHost(hostID *string) (entry domain.HistoryEntry, found bool, err error)
	// List filters per domain.HistoryQuery's three-state HostID (nil = all,
	// pointer to "" = local only, pointer to an ID = that host only).
	List(q domain.HistoryQuery) ([]domain.HistoryEntry, error)
	Delete(id int64) error
	// Clear deletes local entries (hostID points to ""), one host's entries
	// (hostID points to its ID), or every entry (hostID is nil).
	Clear(hostID *string) error
}
