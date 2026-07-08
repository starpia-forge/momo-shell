package aicontrol

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// delegationExpiryTimeout is how long a delegation may sit inactive before
// SweepExpired reaps it. var, not const -- test-shrinkable (mirrors
// connectApprovalTimeout).
var delegationExpiryTimeout = 30 * time.Minute

var controlApprovalTimeout = 60 * time.Second // var, not const -- test-shrinkable

var (
	ErrSessionNotFound         = errors.New("aicontrol: session not found")
	ErrSessionAlreadyDelegated = errors.New("aicontrol: session already delegated to another client")
	ErrControlDenied           = errors.New("aicontrol: control request denied")
	ErrControlApprovalTimeout  = errors.New("aicontrol: control approval timed out")
	ErrNotDelegated            = errors.New("aicontrol: session has no active delegation")
	ErrNotYourDelegation       = errors.New("aicontrol: delegation belongs to a different client")

	ErrNoPendingControlApproval       = errors.New("aicontrol: no pending control approval request")
	ErrControlApprovalAlreadyAnswered = errors.New("aicontrol: control approval request already answered")
)

// controlApprovalPayload is TopicMCPControlApproval's payload, defined next
// to its publisher (house convention -- cf. connectApprovalPayload).
type controlApprovalPayload struct {
	RequestID string `json:"requestId"`
	ClientID  string `json:"clientId"`
	SessionID string `json:"sessionId"`
}

// RequestControl implements in.AIControlUseCase. It delegates an
// already-open session (opened by the human via the GUI, unlike
// ConnectHost which opens one) to clientID, blocking for a human grant
// decision. One active delegation per session: a different client's
// request is rejected outright (no prompt -- silently handing off a
// session someone else already controls is unsafe); the same client's
// repeat request is idempotent.
func (s *Service) RequestControl(clientID, sessionID string, scope domain.ControlScope) (domain.Delegation, error) {
	if _, _, ok := s.sessions.SessionShell(sessionID); !ok {
		return domain.Delegation{}, ErrSessionNotFound
	}

	s.mu.Lock()
	existing, has := s.delegations[sessionID]
	s.mu.Unlock()
	if has {
		if existing.ClientID != clientID {
			return domain.Delegation{}, ErrSessionAlreadyDelegated
		}
		return *existing, nil
	}

	approved, err := s.awaitControlApproval(clientID, sessionID)
	if err != nil {
		return domain.Delegation{}, err
	}
	if !approved {
		return domain.Delegation{}, ErrControlDenied
	}

	deleg := domain.NewDelegation(sessionID, clientID, scope)
	_ = deleg.TransitionTo(domain.DelegActive) // none->delegated is always legal (delegation.go)
	now := time.Now()
	deleg.GrantedAt, deleg.LastActAt = now, now

	s.mu.Lock()
	s.delegations[sessionID] = deleg
	s.mu.Unlock()

	return *deleg, nil
}

// ReleaseControl implements in.AIControlUseCase: clientID voluntarily gives
// up its delegation over sessionID. It does not touch any command the
// session may be running -- see KillCommand (added once F2/signal delivery
// lands) for that.
func (s *Service) ReleaseControl(clientID, sessionID string) error {
	s.mu.Lock()
	deleg, ok := s.delegations[sessionID]
	s.mu.Unlock()
	if !ok {
		return ErrNotDelegated
	}
	if deleg.ClientID != clientID {
		return ErrNotYourDelegation
	}

	if err := deleg.TransitionTo(domain.DelegNone); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.delegations, sessionID)
	s.mu.Unlock()
	return nil
}

// awaitControlApproval publishes mcp:control-approval and blocks (up to
// controlApprovalTimeout) for the matching RespondControlApproval call.
// Structurally identical to awaitConnectApproval (share.awaitPairApproval's
// pattern) but a separate topic/payload so the frontend can distinguish
// which kind of approval dialog to show. Shares the same s.pending map --
// requestIDs are UUIDs, so there's no collision risk across request kinds.
func (s *Service) awaitControlApproval(clientID, sessionID string) (bool, error) {
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
		s.pub.Publish(out.TopicMCPControlApproval(), controlApprovalPayload{
			RequestID: requestID,
			ClientID:  clientID,
			SessionID: sessionID,
		})
	}

	select {
	case approved := <-respCh:
		return approved, nil
	case <-time.After(controlApprovalTimeout):
		return false, ErrControlApprovalTimeout
	}
}

// RespondControlApproval implements in.AIApprovalUseCase, answering a
// pending mcp:control-approval request raised by awaitControlApproval.
func (s *Service) RespondControlApproval(requestID string, approve bool) error {
	s.mu.Lock()
	ch, ok := s.pending[requestID]
	s.mu.Unlock()
	if !ok {
		return ErrNoPendingControlApproval
	}
	select {
	case ch <- approve:
		return nil
	default:
		return ErrControlApprovalAlreadyAnswered
	}
}

// SweepExpired transitions every delegation inactive for
// delegationExpiryTimeout to Expired and removes it, returning the reaped
// session IDs. Pure and deterministic (now is injected) -- it does not
// start any timer itself. Callers decide when/how often to invoke it;
// wiring a periodic call lands with whichever phase starts
// aicontrol.Service's lifecycle (e.g. A7's composition-root wiring), since
// nothing owns starting/stopping such a goroutine yet.
func (s *Service) SweepExpired(now time.Time) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var reaped []string
	for id, d := range s.delegations {
		if d.IsExpired(now, delegationExpiryTimeout) {
			_ = d.TransitionTo(domain.DelegExpired)
			delete(s.delegations, id)
			reaped = append(reaped, id)
		}
	}
	return reaped
}
