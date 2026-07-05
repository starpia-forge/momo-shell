package sharehttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// fakeCallbacks is an in.ShareServerCallbacks stub for driving the HTTP
// layer under test without a real share.Service.
type fakeCallbacks struct {
	info      in.ShareInfo
	pairFunc  func(pin, clientName, remoteAddr string) (string, error)
	hostsFunc func(token string) ([]domain.SharedHost, error)
}

func (f *fakeCallbacks) Info() in.ShareInfo { return f.info }

func (f *fakeCallbacks) HandlePair(pin, clientName, remoteAddr string) (string, error) {
	return f.pairFunc(pin, clientName, remoteAddr)
}

func (f *fakeCallbacks) HostsForToken(token string) ([]domain.SharedHost, error) {
	return f.hostsFunc(token)
}

func startTestServer(t *testing.T, callbacks *fakeCallbacks) (baseURL string, client *http.Client) {
	t.Helper()

	cert, _, _, err := generateCert()
	if err != nil {
		t.Fatalf("generateCert() error = %v", err)
	}

	srv := New(callbacks, cert, WithListenAddrs([]string{"127.0.0.1:0"}))
	port, err := srv.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	return fmt.Sprintf("https://127.0.0.1:%d", port), client
}

func TestHandleInfo_ReturnsShareInfo(t *testing.T) {
	callbacks := &fakeCallbacks{info: in.ShareInfo{Ver: 1, ID: "inst-1", Name: "Starpia-PC", HostCount: 3}}
	baseURL, client := startTestServer(t, callbacks)

	resp, err := client.Get(baseURL + "/api/v1/info")
	if err != nil {
		t.Fatalf("GET /info error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got infoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if got.ID != "inst-1" || got.Name != "Starpia-PC" || got.HostCount != 3 {
		t.Fatalf("got %+v, want id=inst-1 name=Starpia-PC hostCount=3", got)
	}
}

func TestHandlePair_Success(t *testing.T) {
	callbacks := &fakeCallbacks{
		pairFunc: func(pin, clientName, remoteAddr string) (string, error) {
			if pin != "123456" || clientName != "kim-laptop" {
				t.Fatalf("unexpected pair args: pin=%q clientName=%q", pin, clientName)
			}
			return "the-token", nil
		},
	}
	baseURL, client := startTestServer(t, callbacks)

	body, _ := json.Marshal(pairRequestBody{PIN: "123456", ClientName: "kim-laptop"})
	resp, err := client.Post(baseURL+"/api/v1/pair", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /pair error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got pairResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if got.Token != "the-token" {
		t.Fatalf("Token = %q, want the-token", got.Token)
	}
}

func TestHandlePair_WrongPinReturns403(t *testing.T) {
	callbacks := &fakeCallbacks{
		pairFunc: func(pin, clientName, remoteAddr string) (string, error) {
			return "", in.ErrPinMismatch
		},
	}
	baseURL, client := startTestServer(t, callbacks)

	body, _ := json.Marshal(pairRequestBody{PIN: "000000", ClientName: "peer"})
	resp, err := client.Post(baseURL+"/api/v1/pair", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /pair error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestHandlePair_DeniedReturns403(t *testing.T) {
	callbacks := &fakeCallbacks{
		pairFunc: func(pin, clientName, remoteAddr string) (string, error) {
			return "", in.ErrPairDenied
		},
	}
	baseURL, client := startTestServer(t, callbacks)

	body, _ := json.Marshal(pairRequestBody{PIN: "123456", ClientName: "peer"})
	resp, err := client.Post(baseURL+"/api/v1/pair", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /pair error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestHandlePair_LockedOutReturns429WithRetryAfter(t *testing.T) {
	callbacks := &fakeCallbacks{
		pairFunc: func(pin, clientName, remoteAddr string) (string, error) {
			return "", in.ErrLockedOut
		},
	}
	baseURL, client := startTestServer(t, callbacks)

	body, _ := json.Marshal(pairRequestBody{PIN: "000000", ClientName: "peer"})
	resp, err := client.Post(baseURL+"/api/v1/pair", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /pair error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") != "60" {
		t.Fatalf("Retry-After = %q, want 60", resp.Header.Get("Retry-After"))
	}
}

func TestHandleHosts_NoTokenReturns401(t *testing.T) {
	callbacks := &fakeCallbacks{}
	baseURL, client := startTestServer(t, callbacks)

	resp, err := client.Get(baseURL + "/api/v1/hosts")
	if err != nil {
		t.Fatalf("GET /hosts error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestHandleHosts_ValidTokenReturnsSharedHosts(t *testing.T) {
	want := []domain.SharedHost{{Name: "web-prod-01", Address: "10.0.1.15", Port: 22, Username: "deploy"}}
	callbacks := &fakeCallbacks{
		hostsFunc: func(token string) ([]domain.SharedHost, error) {
			if token != "valid-token" {
				return nil, in.ErrShareUnauthorized
			}
			return want, nil
		},
	}
	baseURL, client := startTestServer(t, callbacks)

	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/hosts", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /hosts error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got []domain.SharedHost
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "web-prod-01" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHandleHosts_InvalidTokenReturns401(t *testing.T) {
	callbacks := &fakeCallbacks{
		hostsFunc: func(token string) ([]domain.SharedHost, error) {
			return nil, in.ErrShareUnauthorized
		},
	}
	baseURL, client := startTestServer(t, callbacks)

	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/hosts", nil)
	req.Header.Set("Authorization", "Bearer bogus")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /hosts error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
