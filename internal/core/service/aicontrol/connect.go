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

var connectApprovalTimeout = 60 * time.Second         // var, not const -- test-shrinkable (share.pairApprovalTimeout mirror)
var connectionScopeApprovalTimeout = 60 * time.Second // var, not const -- test-shrinkable

var (
	ErrHostNotFound                   = errors.New("aicontrol: host not found")
	ErrConnectDenied                  = errors.New("aicontrol: connect request denied")
	ErrConnectApprovalTimeout         = errors.New("aicontrol: connect approval timed out")
	ErrNoPendingConnectApproval       = errors.New("aicontrol: no pending connect approval request")
	ErrConnectApprovalAlreadyAnswered = errors.New("aicontrol: connect approval request already answered")

	ErrEmptyHostNames                         = errors.New("aicontrol: connection scope requires at least one host name")
	ErrConnectionScopeDenied                  = errors.New("aicontrol: connection scope request denied")
	ErrConnectionScopeApprovalTimeout         = errors.New("aicontrol: connection scope approval timed out")
	ErrNoPendingConnectionScopeApproval       = errors.New("aicontrol: no pending connection scope approval request")
	ErrConnectionScopeApprovalAlreadyAnswered = errors.New("aicontrol: connection scope approval request already answered")
)

// connectApprovalPayload is TopicMCPConnectApproval's payload, defined next
// to its publisher (house convention -- cf. session/pump.go's StatePayload).
type connectApprovalPayload struct {
	RequestID string `json:"requestId"`
	ClientID  string `json:"clientId"`
	HostName  string `json:"hostName"`
}

// connectionScopePayload is TopicMCPConnectionScopeApproval's payload,
// defined next to its publisher (house convention).
type connectionScopePayload struct {
	RequestID string   `json:"requestId"`
	ClientID  string   `json:"clientId"`
	HostNames []string `json:"hostNames"`
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
	// Snapshot returns every live session's current entity state
	// (ListSessions' data source). session.Service.Snapshot already
	// satisfies this.
	Snapshot() []domain.Session
	// Close tears down a single session (KillControl's cleanup step for
	// an AI-created session, doc 21 §K2). session.Service.Close already
	// satisfies this.
	Close(sessionID string) error
}

// CommandResolver is the narrow slice of resolve.Service RunCommand
// consumes -- a consumer-defined interface (SessionCreator's pattern) so
// tests can inject a canned Verdict instead of driving the real static
// analyzer.
type CommandResolver interface {
	Resolve(command, dialect string) resolve.Verdict
}

// CaptureController is the narrow, core-defined interface RunCommand/
// lifecycle.go/command_control.go need from the E3-b output-capture tap --
// consumer-side (SessionCreator's pattern). capture.Service satisfies it
// structurally.
type CaptureController interface {
	// Begin arms a fresh capture for sessionID linked to auditID.
	Begin(sessionID string, auditID int64)
	// End flags sessionID's in-flight capture as finished (capture.Service
	// defers the actual seal -- see that package's doc for why).
	End(sessionID string)
}

// Deps are the out-ports/collaborators Service needs.
type Deps struct {
	Hosts      out.HostRepository // reused directly -- already a port, no wrapper needed (cf. session.Deps.HostRepo)
	Sessions   SessionCreator
	Publisher  out.EventPublisher
	Resolver   CommandResolver
	Scrollback ScrollbackReader        // ReadScrollback's ring-buffer source (scrollback.Service satisfies it structurally)
	Masker     OutputMasker            // ReadScrollback's secret-redaction layer (mask.Service satisfies it structurally)
	Clients    out.MCPClientRepository // callbacks.go's token store (A4a) -- may be nil until A7 wires a real repo; unused until then
	ShellState ShellStateReader        // GetShellState's cwd/env probe source (state.go, B4) -- main.go adapts shellintegration.Service.Query to this
	Audit      AuditRecorder           // audit trail sink (doc 18 E3) -- nil-tolerant like Masker/Clients; recordAudit no-ops until wired
	Capture    CaptureController       // E3-b output-capture tap -- nil-tolerant like Audit; RunCommand/lifecycle.go/command_control.go no-op until wired
}

// Service implements the AI-control connect flow (in.AIControlUseCase's
// ConnectHost, in.AIApprovalUseCase's RespondConnectApproval), the
// masked-egress read (ReadScrollback), the read-only session/host listing
// (resources.go), and MCP client pairing/auth (in.MCPServerCallbacks,
// callbacks.go).
type Service struct {
	hosts      out.HostRepository
	sessions   SessionCreator
	pub        out.EventPublisher
	resolver   CommandResolver
	scrollback ScrollbackReader
	masker     OutputMasker
	clients    out.MCPClientRepository
	shellState ShellStateReader
	audit      AuditRecorder
	capture    CaptureController

	mu          sync.Mutex
	pending     map[string]chan bool               // requestID -> approval channel (share.Service.pending mirror; connect/control/command/pair/scope requests all share this map, keyed by uuid so there's no collision)
	delegations map[string]*domain.Delegation      // sessionID -> delegation
	commands    map[string]*domain.CommandHandle   // sessionID -> current in-flight command (D1's lifecycle.go drives its transitions)
	scopes      map[string]*domain.ConnectionScope // clientID -> active fan-out grant (one per client, MVP simplicity mirroring RequestControl's one-delegation-per-session rule); RequestConnectionScope overwrites a client's prior scope
}

func New(deps Deps) *Service {
	return &Service{
		hosts:       deps.Hosts,
		sessions:    deps.Sessions,
		pub:         deps.Publisher,
		resolver:    deps.Resolver,
		scrollback:  deps.Scrollback,
		masker:      deps.Masker,
		clients:     deps.Clients,
		shellState:  deps.ShellState,
		audit:       deps.Audit,
		capture:     deps.Capture,
		pending:     make(map[string]chan bool),
		delegations: make(map[string]*domain.Delegation),
		commands:    make(map[string]*domain.CommandHandle),
		scopes:      make(map[string]*domain.ConnectionScope),
	}
}

var _ in.AIApprovalUseCase = (*Service)(nil)

// ConnectHost implements in.AIControlUseCase. It resolves hostName to a
// saved Host, blocks for a human grant decision (every connect_host call is
// a fresh delegation -- design doc 17 §5.2 path A) unless clientID holds an
// active ConnectionScope covering hostName with room under its concurrent
// cap (doc 17 §5.2/FR-10 fan-out -- auto-delegated, no prompt), then opens
// the SSH session and records its delegation.
func (s *Service) ConnectHost(clientID, hostName string) (domain.SessionView, error) {
	host, found, err := s.findHostByName(hostName)
	if err != nil {
		return domain.SessionView{}, err
	}
	if !found {
		return domain.SessionView{}, ErrHostNotFound
	}

	scopeID, autoApprove := s.checkConnectionScope(clientID, hostName)
	if !autoApprove {
		approved, err := s.awaitConnectApproval(clientID, hostName)
		if err != nil {
			s.recordConnectAudit(clientID, "", hostName, "custodian", "timeout")
			return domain.SessionView{}, err
		}
		if !approved {
			s.recordConnectAudit(clientID, "", hostName, "custodian", "rejected")
			return domain.SessionView{}, ErrConnectDenied
		}
	}

	info, err := s.sessions.CreateSSH(in.SSHOpts{HostID: host.ID, Cols: defaultCols, Rows: defaultRows})
	if err != nil {
		return domain.SessionView{}, err
	}

	deleg := domain.NewDelegation(info.ID, clientID, domain.ControlScope{})
	_ = deleg.TransitionTo(domain.DelegActive) // none->delegated is always legal (delegation.go)
	now := time.Now()
	deleg.GrantedAt, deleg.LastActAt = now, now
	deleg.AICreated = true  // fresh session opened for the AI -- KillControl may Close it entirely (doc 21 §K2)
	deleg.ScopeID = scopeID // "" unless auto-approved via an active ConnectionScope

	s.mu.Lock()
	s.delegations[info.ID] = deleg
	s.mu.Unlock()

	s.publishDelegation(info.ID, clientID, "delegated", "granted", deleg.AICreated)
	if autoApprove {
		s.recordConnectAudit(clientID, info.ID, hostName, "", "auto")
	} else {
		s.recordConnectAudit(clientID, info.ID, hostName, "custodian", "granted")
	}

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

// RequestConnectionScope implements in.AIControlUseCase: the optional
// fan-out pre-authorization (design doc 17 §2.3/§5.2, FR-10). It validates
// hostNames against saved hosts up front -- rejecting before ever asking the
// local user to approve a request naming a host that doesn't exist, the
// same principle ConnectHost's own findHostByName check follows -- then
// blocks for a single human grant decision covering the whole batch. The
// concurrent-session cap defaults to the granted host count (deduplicated):
// doc 17's signature takes no separate cap parameter, and one session per
// granted host is the natural reading of "일괄 그랜트 대상 집합" +
// "동시-세션 상한" together.
func (s *Service) RequestConnectionScope(clientID string, hostNames []string) (domain.ConnectionScope, error) {
	if len(hostNames) == 0 {
		return domain.ConnectionScope{}, ErrEmptyHostNames
	}
	for _, name := range hostNames {
		_, found, err := s.findHostByName(name)
		if err != nil {
			return domain.ConnectionScope{}, err
		}
		if !found {
			return domain.ConnectionScope{}, ErrHostNotFound
		}
	}

	approved, err := s.awaitConnectionScopeApproval(clientID, hostNames)
	if err != nil {
		return domain.ConnectionScope{}, err
	}
	if !approved {
		return domain.ConnectionScope{}, ErrConnectionScopeDenied
	}

	hostSet := make(map[string]bool, len(hostNames))
	for _, name := range hostNames {
		hostSet[name] = true
	}

	scope := &domain.ConnectionScope{
		ID:            uuid.NewString(),
		ClientID:      clientID,
		HostNames:     hostSet,
		MaxConcurrent: len(hostSet),
		GrantedAt:     time.Now(),
	}

	s.mu.Lock()
	s.scopes[clientID] = scope
	s.mu.Unlock()

	return *scope, nil
}

// awaitConnectionScopeApproval publishes mcp:connection-scope-approval and
// blocks (up to connectionScopeApprovalTimeout) for the matching
// RespondConnectionScopeApproval call. Structurally identical to
// awaitConnectApproval -- shares the same s.pending map.
func (s *Service) awaitConnectionScopeApproval(clientID string, hostNames []string) (bool, error) {
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
		s.pub.Publish(out.TopicMCPConnectionScopeApproval(), connectionScopePayload{
			RequestID: requestID,
			ClientID:  clientID,
			HostNames: hostNames,
		})
	}

	select {
	case approved := <-respCh:
		return approved, nil
	case <-time.After(connectionScopeApprovalTimeout):
		return false, ErrConnectionScopeApprovalTimeout
	}
}

// RespondConnectionScopeApproval implements in.AIApprovalUseCase, answering
// a pending mcp:connection-scope-approval request raised by
// awaitConnectionScopeApproval.
func (s *Service) RespondConnectionScopeApproval(requestID string, approve bool) error {
	s.mu.Lock()
	ch, ok := s.pending[requestID]
	s.mu.Unlock()
	if !ok {
		return ErrNoPendingConnectionScopeApproval
	}
	select {
	case ch <- approve:
		return nil
	default:
		return ErrConnectionScopeApprovalAlreadyAnswered
	}
}

// liveDelegationCountForScope reports how many of scopeID's delegated
// sessions are still alive. There is no push notification aicontrol can
// subscribe to when a session closes (doc 18 B3(1부)'s original deferral
// reason for the concurrent-session cap) -- session.Service exposes no
// close observer, and out.EventPublisher is publish-only. Instead this
// pulls the live set from Snapshot() (already the data source ListSessions
// uses) on every check, which is exactly as accurate as a push observer
// would be for cap enforcement: the count only needs to be correct at the
// moment a new connect is attempted. It also lazily reaps delegations whose
// session no longer exists, so s.delegations doesn't leak entries for
// sessions that closed outside ReleaseControl/KillControl/SweepExpired.
func (s *Service) liveDelegationCountForScope(scopeID string) int {
	live := make(map[string]bool)
	for _, sess := range s.sessions.Snapshot() {
		live[sess.ID] = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for sessionID, d := range s.delegations {
		if d.ScopeID != scopeID {
			continue
		}
		if live[sessionID] {
			count++
		} else {
			delete(s.delegations, sessionID) // session died outside our control -- reap the stale delegation
		}
	}
	return count
}

// checkConnectionScope reports whether clientID holds an active
// ConnectionScope covering hostName with room under its concurrent cap. If
// so, ConnectHost may auto-delegate the session (scopeID is non-empty,
// autoApprove is true) instead of prompting for individual approval (doc 17
// §5.2 path A). A host outside the scope, no scope at all, or the cap
// already reached all fall back to the normal per-call approval path.
func (s *Service) checkConnectionScope(clientID, hostName string) (scopeID string, autoApprove bool) {
	s.mu.Lock()
	scope, ok := s.scopes[clientID]
	s.mu.Unlock()
	if !ok || !scope.HostNames[hostName] {
		return "", false
	}
	if s.liveDelegationCountForScope(scope.ID) >= scope.MaxConcurrent {
		return "", false
	}
	return scope.ID, true
}
