package aicontrol

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// delegationPayload is TopicMCPDelegation's payload, defined next to its
// publisher (house convention -- cf. connectApprovalPayload). State mirrors
// domain.DelegationState's two endpoints this event ever reports
// ("delegated" on grant, "none" on release/kill/expire) -- the transient
// awaiting_*/tui_handoff substates aren't published here (no consumer yet).
// Reason distinguishes *why* a "none" transition happened, which State
// alone can't: "granted" | "released" | "killed" | "expired".
type delegationPayload struct {
	SessionID string `json:"sessionId"`
	ClientID  string `json:"clientId"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
	AICreated bool   `json:"aiCreated"`
}

// publishDelegation emits TopicMCPDelegation for the frontend's
// delegation/control-panel store (B5a). Callers invoke this after releasing
// s.mu -- Publish may synchronously reach frontend subscriber code, and
// every other awaitX/publish call in this package already follows the
// unlock-then-publish order (see connect.go/state.go).
func (s *Service) publishDelegation(sessionID, clientID, state, reason string, aiCreated bool) {
	if s.pub == nil {
		return
	}
	s.pub.Publish(out.TopicMCPDelegation(), delegationPayload{
		SessionID: sessionID,
		ClientID:  clientID,
		State:     state,
		Reason:    reason,
		AICreated: aiCreated,
	})
}

// ListDelegations implements in.AIApprovalUseCase: every currently active
// delegation, for the frontend's control-panel snapshot on load (B5a) --
// mcp:delegation only reports transitions from that point on, so a fresh
// subscriber needs this to see delegations already in progress.
func (s *Service) ListDelegations() ([]domain.Delegation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]domain.Delegation, 0, len(s.delegations))
	for _, d := range s.delegations {
		list = append(list, *d)
	}
	return list, nil
}
