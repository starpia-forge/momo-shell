package share

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// Info implements in.ShareServerCallbacks, answering GET /api/v1/info.
func (s *Service) Info() in.ShareInfo {
	instanceID, _ := s.settings.InstanceID()
	hostIDs, _ := s.settings.SharedHostIDs()
	return in.ShareInfo{Ver: 1, ID: instanceID, Name: s.deviceName(), HostCount: len(hostIDs)}
}

// HandlePair implements in.ShareServerCallbacks, answering POST
// /api/v1/pair. It validates pin, then blocks for the local user's
// approve/deny decision (via RespondPairing) before issuing a token.
func (s *Service) HandlePair(ctx context.Context, pin, clientName, remoteAddr string) (string, error) {
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return "", in.ErrShareUnauthorized
	}
	if time.Now().Before(s.lockedUntil) {
		s.mu.Unlock()
		return "", in.ErrLockedOut
	}
	expectedPin := s.pin
	s.mu.Unlock()

	if subtle.ConstantTimeCompare([]byte(pin), []byte(expectedPin)) != 1 {
		s.mu.Lock()
		s.pinFailures++
		if s.pinFailures >= maxPinFailures {
			s.lockedUntil = time.Now().Add(pairLockoutDuration)
			s.pinFailures = 0
		}
		s.mu.Unlock()
		return "", in.ErrPinMismatch
	}

	s.mu.Lock()
	s.pinFailures = 0
	s.mu.Unlock()

	approved, err := s.awaitPairApproval(ctx, clientName, remoteAddr)
	if err != nil {
		return "", err
	}
	if !approved {
		return "", in.ErrPairDenied
	}

	return s.issueToken(clientName)
}

// awaitPairApproval publishes share:pair-request and blocks (up to
// pairApprovalTimeout) for the matching RespondPairing call. It also
// unblocks early if ctx is canceled (the inbound HTTP request's client
// disconnected) so a token is never issued for a peer that already gave up
// -- without this, a late RespondPairing(true) after the caller's own
// client-side timeout would silently mint and persist a client no one is
// there to receive.
func (s *Service) awaitPairApproval(ctx context.Context, clientName, remoteAddr string) (approved bool, err error) {
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
			s.pub.Publish(out.TopicSharePairRequestResolved(), pairRequestResolvedPayload{RequestID: requestID})
		}
	}()

	if s.pub != nil {
		s.pub.Publish(out.TopicSharePairRequest(), pairRequestPayload{
			RequestID:  requestID,
			ClientName: clientName,
			RemoteAddr: remoteAddr,
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

// RespondPairing implements in.ShareUseCase, answering a pending
// share:pair-request raised by awaitPairApproval.
func (s *Service) RespondPairing(requestID string, approve bool) error {
	s.mu.Lock()
	ch, ok := s.pending[requestID]
	s.mu.Unlock()
	if !ok {
		return errors.New("share: no pending pairing request")
	}
	select {
	case ch <- approve:
		return nil
	default:
		return errors.New("share: pairing request already answered")
	}
}

func (s *Service) issueToken(clientName string) (string, error) {
	token, tokenHash, err := generateToken()
	if err != nil {
		return "", err
	}
	client := domain.ShareClient{ID: uuid.NewString(), Name: clientName, PairedAt: time.Now()}
	if err := s.clients.Save(client, tokenHash); err != nil {
		return "", err
	}
	return token, nil
}

// HostsForToken implements in.ShareServerCallbacks, answering GET
// /api/v1/hosts. Hosts that no longer exist (deleted since being selected
// for sharing) are silently skipped.
func (s *Service) HostsForToken(token string) ([]domain.SharedHost, error) {
	client, found, err := s.clients.FindByTokenHash(hashToken(token))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, in.ErrShareUnauthorized
	}
	_ = s.clients.TouchSeen(client.ID)

	hostIDs, err := s.settings.SharedHostIDs()
	if err != nil {
		return nil, err
	}

	hosts := make([]domain.SharedHost, 0, len(hostIDs))
	for _, id := range hostIDs {
		h, err := s.hostRepo.Get(id)
		if err != nil {
			continue
		}
		hosts = append(hosts, domain.SharedHost{
			Name:     h.Name,
			Address:  h.Address,
			Port:     h.Port,
			Labels:   h.Labels,
			Username: h.Username,
		})
	}
	return hosts, nil
}

func generatePIN() (string, error) {
	const digits = "0123456789"
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	pin := make([]byte, 6)
	for i, b := range buf {
		pin[i] = digits[int(b)%len(digits)]
	}
	return string(pin), nil
}

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
