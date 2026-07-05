package share

import (
	"context"
	"errors"
	"os"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// peerSyncInterval is how often paired peers are re-polled for their
// shared-host list. Var (not const) so tests can shrink it.
var peerSyncInterval = 30 * time.Second

// ErrPeerNotDiscovered is returned by PairWithPeer for a peerID not
// currently present in the mDNS browse results.
var ErrPeerNotDiscovered = errors.New("share: peer not discovered")

// Start begins mDNS discovery (feeding ListPeers/share:peers-updated) and
// the periodic paired-peer sync loop. Independent of the provider role --
// discovery runs regardless of whether this instance is also sharing.
func (s *Service) Start() error {
	if err := s.browser.Start(s.onDiscoveryUpdate); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.syncCancel = cancel
	s.syncWG.Add(1)
	go func() {
		defer s.syncWG.Done()
		s.syncLoop(ctx)
	}()
	return nil
}

// Close stops discovery and the sync loop, blocking until the sync loop's
// goroutine has actually exited -- required so it's safe to call
// immediately before process shutdown (or, in tests, before the next test
// mutates peerSyncInterval).
func (s *Service) Close() {
	if s.syncCancel != nil {
		s.syncCancel()
	}
	s.syncWG.Wait()
	s.browser.Stop()
}

func (s *Service) onDiscoveryUpdate(peers []out.DiscoveredPeer) {
	s.cmu.Lock()
	discovered := make(map[string]out.DiscoveredPeer, len(peers))
	for _, p := range peers {
		discovered[p.InstanceID] = p
	}
	s.discovered = discovered
	s.cmu.Unlock()
	s.publishPeersUpdated()
}

func (s *Service) publishPeersUpdated() {
	if s.pub != nil {
		s.pub.Publish(out.TopicSharePeersUpdated(), nil)
	}
}

// ListPeers implements in.ShareUseCase, merging paired peers (from
// storage) with currently-discovered-but-unpaired peers (from mDNS).
func (s *Service) ListPeers() ([]in.PeerView, error) {
	stored, err := s.peers.List()
	if err != nil {
		return nil, err
	}

	s.cmu.Lock()
	discovered := make(map[string]out.DiscoveredPeer, len(s.discovered))
	for k, v := range s.discovered {
		discovered[k] = v
	}
	s.cmu.Unlock()

	views := make([]in.PeerView, 0, len(stored)+len(discovered))
	seen := make(map[string]bool, len(stored))
	for _, p := range stored {
		_, online := discovered[p.ID]
		seen[p.ID] = true
		views = append(views, in.PeerView{
			ID: p.ID, Name: p.Name, Address: p.Address, Port: p.Port,
			Paired: true, Online: online, LastSyncAt: p.LastSyncAt, Hosts: p.Hosts,
		})
	}
	for id, d := range discovered {
		if seen[id] {
			continue
		}
		views = append(views, in.PeerView{ID: id, Name: d.Name, Address: d.Address, Port: d.Port, Paired: false, Online: true})
	}
	return views, nil
}

// PairWithPeer implements in.ShareUseCase for a peer found via mDNS.
func (s *Service) PairWithPeer(peerID string, pin string) error {
	s.cmu.Lock()
	d, ok := s.discovered[peerID]
	s.cmu.Unlock()
	if !ok {
		return ErrPeerNotDiscovered
	}
	return s.pairAndStore(d.Address, d.Port, pin)
}

// AddPeerByAddress implements in.ShareUseCase for the direct-IP fallback
// (networks that block multicast) -- it shares the same pairing path as
// PairWithPeer, just skipping the mDNS discovery lookup.
func (s *Service) AddPeerByAddress(address string, port int, pin string) error {
	return s.pairAndStore(address, port, pin)
}

func (s *Service) pairAndStore(address string, port int, pin string) error {
	info, certFP, err := s.peerClient.Info(address, port, "")
	if err != nil {
		return err
	}

	token, observedFP, err := s.peerClient.Pair(address, port, certFP, pin, s.localClientName())
	if err != nil {
		return err
	}

	if err := s.secrets.Set(peerTokenRef(info.ID), []byte(token)); err != nil {
		return err
	}
	return s.peers.Save(domain.Peer{
		ID:              info.ID,
		Name:            info.Name,
		Address:         address,
		Port:            port,
		CertFingerprint: observedFP,
		PairedAt:        time.Now(),
	})
}

// RemovePeer implements in.ShareUseCase, dropping a peer's pairing and its
// stored token.
func (s *Service) RemovePeer(peerID string) error {
	_ = s.secrets.Delete(peerTokenRef(peerID))
	return s.peers.Delete(peerID)
}

// FetchSharedHosts implements in.ShareUseCase, force-refreshing and
// persisting the cache for an already-paired peer.
func (s *Service) FetchSharedHosts(peerID string) ([]domain.SharedHost, error) {
	peer, err := s.peers.Get(peerID)
	if err != nil {
		return nil, err
	}
	hosts, err := s.fetchAndCache(peer)
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

func (s *Service) fetchAndCache(peer domain.Peer) ([]domain.SharedHost, error) {
	tokenBytes, err := s.secrets.Get(peerTokenRef(peer.ID))
	if err != nil {
		return nil, err
	}

	hosts, err := s.peerClient.FetchHosts(peer.Address, peer.Port, peer.CertFingerprint, string(tokenBytes))
	if err != nil {
		return nil, err
	}

	peer.Hosts = hosts
	now := time.Now()
	peer.LastSyncAt = &now
	if err := s.peers.Save(peer); err != nil {
		return nil, err
	}
	return hosts, nil
}

func (s *Service) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(peerSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncOnce()
		}
	}
}

// syncOnce re-fetches every paired peer's shared-host list. A peer that
// rejects our token (revoked) is dropped entirely; any other failure just
// leaves the peer's cache in place -- ListPeers reports it offline via the
// discovery set, not via a persisted flag.
func (s *Service) syncOnce() {
	peers, err := s.peers.List()
	if err != nil {
		return
	}

	for _, peer := range peers {
		if _, err := s.fetchAndCache(peer); err != nil {
			if errors.Is(err, out.ErrPeerUnauthorized) {
				_ = s.RemovePeer(peer.ID)
			}
			continue
		}
	}
	s.publishPeersUpdated()
}

func (s *Service) localClientName() string {
	name, _ := os.Hostname()
	return name
}

func peerTokenRef(peerID string) string {
	return "sharetoken:" + peerID
}
