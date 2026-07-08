package aicontrol

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// pairApprovalTimeout bounds how long HandlePair waits for the local
// user's approve/deny decision. Var, not const -- test-shrinkable (mirrors
// connectApprovalTimeout/controlApprovalTimeout).
var pairApprovalTimeout = 60 * time.Second

var (
	ErrNoPendingPairApproval       = errors.New("aicontrol: no pending pair approval request")
	ErrPairApprovalAlreadyAnswered = errors.New("aicontrol: pair approval request already answered")
)

// mcpPairRequestPayload is TopicMCPPairRequest's payload, defined next to
// its publisher (house convention -- cf. connectApprovalPayload). Unlike
// share's pairRequestPayload, there is no RemoteAddr: MCP IPC is a local
// named-pipe/unix-socket connection, not a LAN peer.
type mcpPairRequestPayload struct {
	RequestID  string `json:"requestId"`
	ClientName string `json:"clientName"`
}

// mcpPairRequestResolvedPayload is TopicMCPPairRequestResolved's payload.
type mcpPairRequestResolvedPayload struct {
	RequestID string `json:"requestId"`
}

// HandlePair implements in.MCPServerCallbacks. Unlike share's HandlePair,
// there is no PIN and no lockout to check (doc 20 D1: the mcpipc
// transport's SID-gate already proves same-user, so pairing is human
// approval alone) -- it goes straight to the approval wait.
func (s *Service) HandlePair(ctx context.Context, clientName string) (string, error) {
	approved, err := s.awaitPairApproval(ctx, clientName)
	if err != nil {
		return "", err
	}
	if !approved {
		return "", in.ErrPairDenied
	}
	return s.issueToken(clientName)
}

// awaitPairApproval publishes mcp:pair-request and blocks (up to
// pairApprovalTimeout) for the matching RespondPairing call. It also
// unblocks early if ctx is canceled -- the mcpipc adapter's pair.go cancels
// this ctx when the connecting client disconnects before a decision
// arrives, so a token is never issued for a peer that already gave up
// (share.awaitPairApproval mirror).
func (s *Service) awaitPairApproval(ctx context.Context, clientName string) (approved bool, err error) {
	requestID := uuid.NewString()
	respCh := make(chan bool, 1)

	s.mu.Lock()
	s.pending[requestID] = respCh
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, requestID)
		s.mu.Unlock()
		if s.pub != nil {
			s.pub.Publish(out.TopicMCPPairRequestResolved(), mcpPairRequestResolvedPayload{RequestID: requestID})
		}
	}()

	if s.pub != nil {
		s.pub.Publish(out.TopicMCPPairRequest(), mcpPairRequestPayload{
			RequestID:  requestID,
			ClientName: clientName,
		})
	}

	select {
	case approved = <-respCh:
		return approved, nil
	case <-time.After(pairApprovalTimeout):
		return false, in.ErrPairTimeout
	case <-ctx.Done():
		return false, in.ErrPairTimeout
	}
}

// RespondPairing implements in.AIApprovalUseCase, answering a pending
// mcp:pair-request raised by awaitPairApproval.
func (s *Service) RespondPairing(requestID string, approve bool) error {
	s.mu.Lock()
	ch, ok := s.pending[requestID]
	s.mu.Unlock()
	if !ok {
		return ErrNoPendingPairApproval
	}
	select {
	case ch <- approve:
		return nil
	default:
		return ErrPairApprovalAlreadyAnswered
	}
}

// issueToken mints a fresh token, persists its hash (never the token
// itself) against a new MCPClient row, and returns the raw token.
func (s *Service) issueToken(clientName string) (string, error) {
	token, tokenHash, err := generateToken()
	if err != nil {
		return "", err
	}
	client := domain.MCPClient{ClientID: uuid.NewString(), Name: clientName, PairedAt: time.Now()}
	if err := s.clients.Save(client, tokenHash); err != nil {
		return "", err
	}
	return token, nil
}

// AuthClient implements in.MCPServerCallbacks. It resolves token to the
// clientID it authorizes, rejecting both unknown and revoked tokens with
// the same in.ErrUnauthorized -- FindByTokenHash deliberately returns
// revoked rows as-is, so this is where revocation is actually enforced.
func (s *Service) AuthClient(token string) (string, error) {
	client, found, err := s.clients.FindByTokenHash(hashToken(token))
	if err != nil {
		return "", err
	}
	if !found || client.Revoked {
		return "", in.ErrUnauthorized
	}
	_ = s.clients.TouchSeen(client.ClientID)
	return client.ClientID, nil
}

var _ in.MCPServerCallbacks = (*Service)(nil)

func generateToken() (token, tokenHash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
