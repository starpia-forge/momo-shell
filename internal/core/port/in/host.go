package in

import "momo-terminal/internal/core/domain"

// HostInput carries every Host field except secret material (password/key
// passphrase), which flows only through SetHostSecret so it never appears
// in a log or a general-purpose DTO.
type HostInput struct {
	ID       string
	Name     string
	Address  string
	Port     int
	Labels   []string
	Username string
	AuthType domain.AuthType
	KeyPath  string
}

// HostUseCase is the driving port for host CRUD and connection testing.
type HostUseCase interface {
	ListHosts() ([]domain.Host, error)
	GetHost(id string) (domain.Host, error)
	// SaveHost creates a host when in.ID == "", otherwise updates it.
	SaveHost(in HostInput) (domain.Host, error)
	DeleteHost(id string) error
	// SetHostSecret stores the password or key passphrase for a host.
	SetHostSecret(id string, secret string) error
	TestConnection(id string) (TestResult, error)
	ListLabels() ([]string, error)
}

// TestResult mirrors port/out.TestResult across the in-port boundary so
// driving adapters don't need to import port/out directly.
type TestResult struct {
	Stage   string
	OK      bool
	Message string
}
