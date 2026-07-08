// Package aicontrol implements the AI-facing control plane (design doc 17
// §4-5): connect_host today (this file); resolve/ already implements the
// command-safety gate (doc 18 C1-C3).
package aicontrol

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
	"momo-shell/internal/core/service/aicontrol/resolve"
)

// defaultCols/defaultRows size a session opened via connect_host: MCP
// callers have no terminal to size against, unlike the GUI's local/SSH
// dialogs which pass the pane's actual geometry.
const (
	defaultCols = 80
	defaultRows = 24
)

var connectApprovalTimeout = 60 * time.Second // var, not const -- test-shrinkable (share.pairApprovalTimeout mirror)

var (
	ErrHostNotFound                   = errors.New("aicontrol: host not found")
	ErrConnectDenied                  = errors.New("aicontrol: connect request denied")
	ErrConnectApprovalTimeout         = errors.New("aicontrol: connect approval timed out")
	ErrNoPendingConnectApproval       = errors.New("aicontrol: no pending connect approval request")
	ErrConnectApprovalAlreadyAnswered = errors.New("aicontrol: connect approval request already answered")
)

// connectApprovalPayload is TopicMCPConnectApproval's payload, defined next
// to its publisher (house convention -- cf. session/pump.go's StatePayload).
type connectApprovalPayload struct {
	RequestID string `json:"requestId"`
	ClientID  string `json:"clientId"`
	HostName  string `json:"hostName"`
}

// SessionCreator is the narrow, core-defined interface this service needs
// from the session service -- consumer-side, matching
// shellintegration.ShellInjector.
type SessionCreator interface {
	CreateSSH(opts in.SSHOpts) (domain.SessionInfo, error)
	// SessionShell reports whether sessionID is a live session -- reused
	// here purely for its ok bool (RequestControl's existence check).
	SessionShell(sessionID string) (shell string, kind domain.SessionKind, ok bool)
	// Write injects a command line into a live session (RunCommand's
	// execution path). session.Service.Write already satisfies this.
	Write(sessionID string, data []byte) error
}

// CommandResolver is the narrow slice of resolve.Service RunCommand
// consumes -- a consumer-defined interface (SessionCreator's pattern) so
// tests can inject a canned Verdict instead of driving the real static
// analyzer.
type CommandResolver interface {
	Resolve(command, dialect string) resolve.Verdict
}

// Deps are the out-ports/collaborators Service needs.
type Deps struct {
	Hosts      out.HostRepository // reused directly -- already a port, no wrapper needed (cf. session.Deps.HostRepo)
	Sessions   SessionCreator
	Publisher  out.EventPublisher
	Resolver   CommandResolver
	Scrollback ScrollbackReader // ReadScrollback's ring-buffer source (scrollback.Service satisfies it structurally)
	Masker     OutputMasker     // ReadScrollback's secret-redaction layer (mask.Service satisfies it structurally)
}

// Service implements the AI-control connect flow (in.AIControlUseCase's
// ConnectHost, in.AIApprovalUseCase's RespondConnectApproval) and the
// masked-egress read (ReadScrollback). It does not yet implement all of
// in.AIControlUseCase (ListSessions/ListHosts land in A4, blocked on their
// own missing data sources today), so no compile-time conformance assertion
// against that interface is made here -- it's added once a phase closes out
// the interface (A4 or later), rather than forcing speculative stub methods
// now.
type Service struct {
	hosts      out.HostRepository
	sessions   SessionCreator
	pub        out.EventPublisher
	resolver   CommandResolver
	scrollback ScrollbackReader
	masker     OutputMasker

	mu          sync.Mutex
	pending     map[string]chan bool             // requestID -> approval channel (share.Service.pending mirror)
	delegations map[string]*domain.Delegation    // sessionID -> delegation
	commands    map[string]*domain.CommandHandle // sessionID -> current in-flight command (D1's lifecycle.go drives its transitions)
}

func New(deps Deps) *Service {
	return &Service{
		hosts:       deps.Hosts,
		sessions:    deps.Sessions,
		pub:         deps.Publisher,
		resolver:    deps.Resolver,
		scrollback:  deps.Scrollback,
		masker:      deps.Masker,
		pending:     make(map[string]chan bool),
		delegations: make(map[string]*domain.Delegation),
		commands:    make(map[string]*domain.CommandHandle),
	}
}

var _ in.AIApprovalUseCase = (*Service)(nil)

// ConnectHost implements in.AIControlUseCase. It resolves hostName to a
// saved Host, blocks for a human grant decision (every connect_host call is
// a fresh delegation -- design doc 17 §5.2 path A), then opens the SSH
// session and records its delegation.
func (s *Service) ConnectHost(clientID, hostName string) (domain.SessionView, error) {
	host, found, err := s.findHostByName(hostName)
	if err != nil {
		return domain.SessionView{}, err
	}
	if !found {
		return domain.SessionView{}, ErrHostNotFound
	}

	approved, err := s.awaitConnectApproval(clientID, hostName)
	if err != nil {
		return domain.SessionView{}, err
	}
	if !approved {
		return domain.SessionView{}, ErrConnectDenied
	}

	info, err := s.sessions.CreateSSH(in.SSHOpts{HostID: host.ID, Cols: defaultCols, Rows: defaultRows})
	if err != nil {
		return domain.SessionView{}, err
	}

	deleg := domain.NewDelegation(info.ID, clientID, domain.ControlScope{})
	_ = deleg.TransitionTo(domain.DelegActive) // none->delegated is always legal (delegation.go)
	now := time.Now()
	deleg.GrantedAt, deleg.LastActAt = now, now

	s.mu.Lock()
	s.delegations[info.ID] = deleg
	s.mu.Unlock()

	// State is the creation-time invariant CreateSSH documents (always
	// Connecting, dial happens in background), not a live query -- there is
	// no live session-state accessor yet (that's ListSessions/A4's gap).
	return domain.SessionView{
		ID:         info.ID,
		Kind:       info.Kind,
		HostID:     info.HostID,
		State:      domain.StateConnecting,
		Controlled: true,
	}, nil
}

// findHostByName looks up a saved host by name. HostRepository has no
// by-name lookup (Get is by ID only), so this lists and matches -- the
// AI-facing surface only ever supplies a name (domain.HostRef carries no
// ID), never the internal Host.ID.
func (s *Service) findHostByName(name string) (domain.Host, bool, error) {
	hosts, err := s.hosts.List()
	if err != nil {
		return domain.Host{}, false, err
	}
	for _, h := range hosts {
		if h.Name == name {
			return h, true, nil
		}
	}
	return domain.Host{}, false, nil
}

// awaitConnectApproval publishes mcp:connect-approval and blocks (up to
// connectApprovalTimeout) for the matching RespondConnectApproval call.
// Unlike share.Service.awaitPairApproval, there is no ctx to cancel on
// early caller disconnect: ConnectHost's design signature (doc 17 §4) takes
// no context, and there's no MCP dispatch adapter (A6) yet to supply one.
func (s *Service) awaitConnectApproval(clientID, hostName string) (bool, error) {
	requestID := uuid.NewString()
	respCh := make(chan bool, 1)

	s.mu.Lock()
	s.pending[requestID] = respCh
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, requestID)
		s.mu.Unlock()
	}()

	if s.pub != nil {
		s.pub.Publish(out.TopicMCPConnectApproval(), connectApprovalPayload{
			RequestID: requestID,
			ClientID:  clientID,
			HostName:  hostName,
		})
	}

	select {
	case approved := <-respCh:
		return approved, nil
	case <-time.After(connectApprovalTimeout):
		return false, ErrConnectApprovalTimeout
	}
}

// RespondConnectApproval implements in.AIApprovalUseCase, answering a
// pending mcp:connect-approval request raised by awaitConnectApproval.
func (s *Service) RespondConnectApproval(requestID string, approve bool) error {
	s.mu.Lock()
	ch, ok := s.pending[requestID]
	s.mu.Unlock()
	if !ok {
		return ErrNoPendingConnectApproval
	}
	select {
	case ch <- approve:
		return nil
	default:
		return ErrConnectApprovalAlreadyAnswered
	}
}
