// Package shareclient implements out.PeerClient by calling another
// instance's LAN share API (adapter/in/sharehttp) over TLS with
// TOFU-pinned trust.
package shareclient

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

const requestTimeout = 10 * time.Second

// pairRequestTimeout applies only to Pair: the provider's HandlePair blocks
// for up to the local user's approval decision (share.pairApprovalTimeout,
// 60s; sharehttp.approvalWriteTimeout gives the server 90s to accommodate
// it), so the shared 10s requestTimeout would abort every pairing attempt
// before a human has a realistic chance to click approve/deny.
const pairRequestTimeout = 90 * time.Second

var (
	// errCertMismatchInternal marks a handshake failure caused by our own
	// VerifyPeerCertificate rejecting a pinned-cert mismatch, so the
	// exported methods can translate it to out.ErrPeerCertMismatch --
	// distinct from a generic network error, which they pass through as-is.
	errCertMismatchInternal = errors.New("shareclient: certificate fingerprint mismatch (peer's certificate changed)")
	// ErrPairRejected covers wrong PIN, user denial, and approval timeout
	// -- the provider intentionally makes these indistinguishable.
	ErrPairRejected  = errors.New("shareclient: pairing rejected (wrong pin, denied, or timed out)")
	ErrPairLockedOut = errors.New("shareclient: too many failed pairing attempts, try again later")
)

// Client implements out.PeerClient.
type Client struct{}

func New() *Client {
	return &Client{}
}

var _ out.PeerClient = (*Client)(nil)

func (c *Client) Info(address string, port int, certFP string) (out.PeerInfo, string, error) {
	httpClient, observed := pinnedClient(certFP)

	resp, err := httpClient.Get(fmt.Sprintf("https://%s:%d/api/v1/info", address, port))
	if err != nil {
		return out.PeerInfo{}, "", observed.translateErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out.PeerInfo{}, "", fmt.Errorf("shareclient: info: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		Ver       int    `json:"ver"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		HostCount int    `json:"hostCount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return out.PeerInfo{}, "", err
	}
	return out.PeerInfo{Ver: body.Ver, ID: body.ID, Name: body.Name, HostCount: body.HostCount}, observed.fingerprint, nil
}

func (c *Client) Pair(address string, port int, certFP, pin, clientName string) (token, observedFP string, err error) {
	httpClient, observed := pinnedClient(certFP)
	httpClient.Timeout = pairRequestTimeout

	reqBody, err := json.Marshal(struct {
		PIN        string `json:"pin"`
		ClientName string `json:"clientName"`
	}{PIN: pin, ClientName: clientName})
	if err != nil {
		return "", "", err
	}

	resp, err := httpClient.Post(fmt.Sprintf("https://%s:%d/api/v1/pair", address, port), "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", observed.translateErr(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		return "", "", ErrPairLockedOut
	default:
		return "", "", ErrPairRejected
	}

	var respBody struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return "", "", err
	}
	return respBody.Token, observed.fingerprint, nil
}

// FetchHosts returns out.ErrPeerUnauthorized on a 401 response.
func (c *Client) FetchHosts(address string, port int, certFP, token string) ([]domain.SharedHost, error) {
	httpClient, observed := pinnedClient(certFP)

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s:%d/api/v1/hosts", address, port), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, observed.translateErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, out.ErrPeerUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shareclient: hosts: unexpected status %d", resp.StatusCode)
	}

	var hosts []domain.SharedHost
	if err := json.NewDecoder(resp.Body).Decode(&hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

// observedCert captures the fingerprint of whatever certificate the peer
// presents during a request (so callers can persist it for future pinning)
// and whether it mismatched a pinned certFP.
type observedCert struct {
	fingerprint string
	mismatch    bool
}

// translateErr replaces a pinned-cert-mismatch handshake failure with
// out.ErrPeerCertMismatch; any other error (network failure, timeout,
// etc.) passes through unchanged.
func (o *observedCert) translateErr(err error) error {
	if o.mismatch {
		return out.ErrPeerCertMismatch
	}
	return err
}

// pinnedClient builds an http.Client whose TLS verification is entirely
// custom: certFP == "" (TOFU) accepts any certificate and records its
// fingerprint; a non-empty certFP rejects anything but an exact match.
// Self-signed certificates always fail standard chain verification, so
// InsecureSkipVerify is required -- VerifyPeerCertificate is what actually
// enforces trust here.
func pinnedClient(certFP string) (*http.Client, *observedCert) {
	observed := &observedCert{}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("shareclient: no certificate presented")
			}
			sum := sha256.Sum256(rawCerts[0])
			fp := hex.EncodeToString(sum[:])
			observed.fingerprint = fp
			if certFP != "" && fp != certFP {
				observed.mismatch = true
				return errCertMismatchInternal
			}
			return nil
		},
	}
	client := &http.Client{
		Timeout:   requestTimeout,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}
	return client, observed
}
