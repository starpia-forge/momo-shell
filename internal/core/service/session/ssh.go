package session

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// hostKeyPromptTimeout bounds how long CreateSSH waits for RespondHostKey
// on an unrecognized host key before giving up and failing the connect.
// Var (not const) so tests can shrink it rather than waiting the full 60s.
var hostKeyPromptTimeout = 60 * time.Second

// HostKeyPromptPayload is published on session:hostkey:{id} when an SSH
// connection meets a host key with no (or a mismatched) known_hosts entry.
type HostKeyPromptPayload struct {
	Address     string `json:"address"`
	Port        int    `json:"port"`
	Algo        string `json:"algo"`
	Fingerprint string `json:"fingerprint"`
}

// CreateSSH registers the session as Connecting and returns immediately;
// the dial/handshake/auth happens in connectSSH and is reported via
// session:state events, matching how the UI shows a tab before it's live.
func (s *Service) CreateSSH(opts in.SSHOpts) (domain.SessionInfo, error) {
	host, err := s.hostRepo.Get(opts.HostID)
	if err != nil {
		return domain.SessionInfo{}, err
	}

	var secret string
	if host.AuthType != domain.AuthAgent {
		if b, err := s.secrets.Get("host:" + host.ID); err == nil {
			secret = string(b)
		}
	}

	id := uuid.NewString()
	sess := domain.NewSSHSession(id, host.ID, opts.Cols, opts.Rows)

	live := &liveSession{
		session:     sess,
		readCh:      make(chan []byte, 64),
		hostKeyResp: make(chan string, 1),
		cancelCh:    make(chan struct{}),
	}

	s.mu.Lock()
	s.sessions[id] = live
	s.mu.Unlock()

	s.pub.Publish(out.TopicSessionState(id), StatePayload{State: string(domain.StateConnecting)})

	go s.connectSSH(id, host, secret, live)

	return sess.Info(), nil
}

func (s *Service) connectSSH(id string, host domain.Host, secret string, live *liveSession) {
	verifier := s.sshHostKeyVerifier(id, host, live)
	stream, err := s.sshOpener.Open(host, secret, verifier, live.session.Cols, live.session.Rows)

	live, stillPresent := s.get(id)
	if !stillPresent {
		// Close(id) already cancelled and removed this session while we
		// were connecting; if we won the race and connected anyway, don't
		// leak the stream.
		if err == nil {
			_ = stream.Close()
		}
		return
	}

	if err != nil {
		s.remove(id)
		_ = live.session.TransitionTo(domain.StateError)
		s.pub.Publish(out.TopicSessionState(id), StatePayload{State: string(domain.StateError), Error: err.Error()})
		return
	}

	live.setStream(stream)
	_ = live.session.TransitionTo(domain.StateRunning)
	_ = s.hostRepo.TouchConnected(host.ID)

	if s.tap != nil {
		s.tap.Attach(id, host.ID)
	}
	if s.middleware != nil {
		s.middleware.Attach(id, domain.KindSSH)
	}

	s.wg.Add(2)
	go s.readLoop(live)
	go s.pump(id, live)

	s.pub.Publish(out.TopicSessionState(id), StatePayload{State: string(domain.StateRunning)})
}

// sshHostKeyVerifier builds the out.HostKeyVerifier for one connect attempt.
// A known, matching fingerprint proceeds silently; a recorded mismatch is
// rejected outright; an unrecognized key is surfaced to the user via
// session:hostkey:{id} and blocks (up to hostKeyPromptTimeout) for
// RespondHostKey.
func (s *Service) sshHostKeyVerifier(id string, host domain.Host, live *liveSession) out.HostKeyVerifier {
	return func(algo, fingerprint string) (out.HostKeyDecision, error) {
		known, found, err := s.knownHosts.Get(host.Address, host.Port, algo)
		if err != nil {
			return out.HostKeyCancel, err
		}
		if found {
			if known == fingerprint {
				return out.HostKeyOnce, nil
			}
			return out.HostKeyCancel, fmt.Errorf("host key mismatch for %s:%d (%s)", host.Address, host.Port, algo)
		}

		s.pub.Publish(out.TopicSessionHostKey(id), HostKeyPromptPayload{
			Address:     host.Address,
			Port:        host.Port,
			Algo:        algo,
			Fingerprint: fingerprint,
		})

		select {
		case decision := <-live.hostKeyResp:
			switch decision {
			case "trust":
				if err := s.knownHosts.Put(host.Address, host.Port, algo, fingerprint); err != nil {
					return out.HostKeyCancel, err
				}
				return out.HostKeyTrust, nil
			case "once":
				return out.HostKeyOnce, nil
			default:
				return out.HostKeyCancel, errors.New("session: host key rejected by user")
			}
		case <-live.cancelCh:
			return out.HostKeyCancel, errors.New("session: cancelled while awaiting host key confirmation")
		case <-time.After(hostKeyPromptTimeout):
			return out.HostKeyCancel, errors.New("session: host key confirmation timed out")
		}
	}
}

// RespondHostKey answers a pending session:hostkey:{id} prompt raised by
// sshHostKeyVerifier. decision is one of "trust", "once", or "cancel".
func (s *Service) RespondHostKey(sessionID string, decision string) error {
	live, ok := s.get(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	if live.hostKeyResp == nil {
		return errors.New("session: no pending host key prompt")
	}
	select {
	case live.hostKeyResp <- decision:
		return nil
	default:
		return errors.New("session: host key prompt already answered or not awaited")
	}
}
