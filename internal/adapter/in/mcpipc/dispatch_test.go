package mcpipc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/service/aicontrol"
)

// fakeAIControlUseCase implements in.AIControlUseCase for dispatch tests.
// The read-only slice (ListSessions/ListHosts/ReadScrollback) plus the
// minimal control loop dispatch now exposes (RequestControl/RunCommand/
// ReleaseControl) have usable injectable implementations -- the rest panic
// if ever invoked, since dispatch does not reach them yet.
type fakeAIControlUseCase struct {
	listSessionsFunc   func(clientID string) ([]domain.SessionView, error)
	listHostsFunc      func(clientID string) ([]domain.HostRef, error)
	readScrollbackFunc func(clientID, sessionID string, sinceSeq uint64) (domain.MaskedChunk, error)
	requestControlFunc func(clientID, sessionID string, scope domain.ControlScope) (domain.Delegation, error)
	runCommandFunc     func(clientID, sessionID, command string) (domain.CommandHandle, error)
	releaseControlFunc func(clientID, sessionID string) error
}

func (f *fakeAIControlUseCase) ListSessions(clientID string) ([]domain.SessionView, error) {
	return f.listSessionsFunc(clientID)
}

func (f *fakeAIControlUseCase) ListHosts(clientID string) ([]domain.HostRef, error) {
	return f.listHostsFunc(clientID)
}

func (f *fakeAIControlUseCase) ReadScrollback(clientID, sessionID string, sinceSeq uint64) (domain.MaskedChunk, error) {
	return f.readScrollbackFunc(clientID, sessionID, sinceSeq)
}

func (f *fakeAIControlUseCase) ConnectHost(clientID, hostName string) (domain.SessionView, error) {
	panic("not used by dispatch's read-only scope")
}

func (f *fakeAIControlUseCase) RequestControl(clientID, sessionID string, scope domain.ControlScope) (domain.Delegation, error) {
	return f.requestControlFunc(clientID, sessionID, scope)
}

func (f *fakeAIControlUseCase) ReleaseControl(clientID, sessionID string) error {
	return f.releaseControlFunc(clientID, sessionID)
}

func (f *fakeAIControlUseCase) RunCommand(clientID, sessionID, command string) (domain.CommandHandle, error) {
	return f.runCommandFunc(clientID, sessionID, command)
}

func (f *fakeAIControlUseCase) RequestConnectionScope(clientID string, hostNames []string) (domain.ConnectionScope, error) {
	panic("not used by dispatch's read-only scope")
}

func (f *fakeAIControlUseCase) GetShellState(clientID, sessionID string) (domain.ShellState, error) {
	panic("not used by dispatch's read-only scope")
}

func (f *fakeAIControlUseCase) ResetShell(clientID, sessionID string) error {
	panic("not used by dispatch's read-only scope")
}

func (f *fakeAIControlUseCase) CancelCommand(clientID, sessionID string) (domain.CommandHandle, error) {
	panic("not used by dispatch's read-only scope")
}

func (f *fakeAIControlUseCase) BackgroundCommand(clientID, sessionID string) (domain.CommandHandle, error) {
	panic("not used by dispatch's read-only scope")
}

var _ in.AIControlUseCase = (*fakeAIControlUseCase)(nil)

// startDispatchSession wires Dispatch.HandleSession to one end of a
// net.Pipe (mirroring the real SessionHandler seam) and connects an SDK
// client to the other end, returning the client session and a channel that
// closes once HandleSession returns.
func startDispatchSession(t *testing.T, control in.AIControlUseCase, clientID string) (*mcp.ClientSession, <-chan struct{}) {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		NewDispatch(control).HandleSession(clientID, serverConn)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	transport := &mcp.IOTransport{Reader: clientConn, Writer: nopWriteCloser{clientConn}}
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		session.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("HandleSession did not return after session.Close()")
		}
	})

	return session, done
}

func TestHandleSession_ListToolsAdvertisesToolSurface(t *testing.T) {
	control := &fakeAIControlUseCase{}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	names := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{
		"list_sessions", "list_hosts", "read_output",
		"request_control", "run_command", "release_control",
	} {
		if !names[want] {
			t.Errorf("tools/list missing %q, got %v", want, names)
		}
	}
}

func TestHandleSession_ListSessionsToolCallsUseCaseWithClientID(t *testing.T) {
	want := []domain.SessionView{{ID: "s1", Kind: domain.KindSSH, State: domain.StateRunning, Controlled: true}}
	var gotClientID string
	control := &fakeAIControlUseCase{
		listSessionsFunc: func(clientID string) ([]domain.SessionView, error) {
			gotClientID = clientID
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_sessions", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool(list_sessions) error = %v", err)
	}
	if gotClientID != "client-1" {
		t.Errorf("ListSessions clientID = %q, want client-1", gotClientID)
	}

	var out listSessionsOutput
	decodeToolResult(t, res, &out)
	if len(out.Sessions) != 1 || out.Sessions[0].ID != "s1" {
		t.Errorf("Sessions = %+v, want one session with ID s1", out.Sessions)
	}
}

func TestHandleSession_ListHostsToolExcludesCredentialsByType(t *testing.T) {
	want := []domain.HostRef{{Name: "prod-1", Labels: []string{"prod"}}}
	var gotClientID string
	control := &fakeAIControlUseCase{
		listHostsFunc: func(clientID string) ([]domain.HostRef, error) {
			gotClientID = clientID
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_hosts", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool(list_hosts) error = %v", err)
	}
	if gotClientID != "client-1" {
		t.Errorf("ListHosts clientID = %q, want client-1", gotClientID)
	}

	var out listHostsOutput
	decodeToolResult(t, res, &out)
	if len(out.Hosts) != 1 || out.Hosts[0].Name != "prod-1" {
		t.Errorf("Hosts = %+v, want one host named prod-1", out.Hosts)
	}
}

func TestHandleSession_ReadOutputToolForwardsSessionIDAndSinceSeq(t *testing.T) {
	want := domain.MaskedChunk{Data: "hello", NextSeq: 42}
	var gotSessionID string
	var gotSince uint64
	control := &fakeAIControlUseCase{
		readScrollbackFunc: func(clientID, sessionID string, sinceSeq uint64) (domain.MaskedChunk, error) {
			gotSessionID, gotSince = sessionID, sinceSeq
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "read_output",
		Arguments: map[string]any{"sessionId": "s1", "sinceSeq": 7},
	})
	if err != nil {
		t.Fatalf("CallTool(read_output) error = %v", err)
	}
	if gotSessionID != "s1" || gotSince != 7 {
		t.Errorf("ReadScrollback(sessionID, sinceSeq) = (%q, %d), want (s1, 7)", gotSessionID, gotSince)
	}

	var out domain.MaskedChunk
	decodeToolResult(t, res, &out)
	if out.Data != "hello" || out.NextSeq != 42 {
		t.Errorf("MaskedChunk = %+v, want %+v", out, want)
	}
}

func TestHandleSession_RequestControlToolForwardsScopeAndDecodesDelegation(t *testing.T) {
	want := domain.Delegation{SessionID: "s1", ClientID: "client-1", State: domain.DelegActive}
	var gotClientID, gotSessionID string
	var gotScope domain.ControlScope
	control := &fakeAIControlUseCase{
		requestControlFunc: func(clientID, sessionID string, scope domain.ControlScope) (domain.Delegation, error) {
			gotClientID, gotSessionID, gotScope = clientID, sessionID, scope
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "request_control",
		Arguments: map[string]any{"sessionId": "s1", "hostOnly": true, "pathPrefix": "/srv", "readOnly": true},
	})
	if err != nil {
		t.Fatalf("CallTool(request_control) error = %v", err)
	}
	if gotClientID != "client-1" || gotSessionID != "s1" {
		t.Errorf("RequestControl(clientID, sessionID) = (%q, %q), want (client-1, s1)", gotClientID, gotSessionID)
	}
	wantScope := domain.ControlScope{HostOnly: true, PathPrefix: "/srv", ReadOnly: true}
	if gotScope != wantScope {
		t.Errorf("scope = %+v, want %+v", gotScope, wantScope)
	}

	var out requestControlOutput
	decodeToolResult(t, res, &out)
	if out.Delegation.SessionID != "s1" || out.Delegation.State != domain.DelegActive {
		t.Errorf("Delegation = %+v, want %+v", out.Delegation, want)
	}
}

func TestHandleSession_RequestControlToolSurfacesDenialAsError(t *testing.T) {
	control := &fakeAIControlUseCase{
		requestControlFunc: func(clientID, sessionID string, scope domain.ControlScope) (domain.Delegation, error) {
			return domain.Delegation{}, errors.New("control denied by human approver")
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "request_control",
		Arguments: map[string]any{"sessionId": "s1"},
	})
	if err != nil {
		t.Fatalf("CallTool(request_control) error = %v", err)
	}
	if !res.IsError {
		t.Fatal("result.IsError = false, want true for a denied control request")
	}
}

func TestHandleSession_RunCommandToolForwardsCommandAndDecodesHandle(t *testing.T) {
	want := *domain.NewCommandHandle("s1", "ls -la")
	var gotClientID, gotSessionID, gotCommand string
	control := &fakeAIControlUseCase{
		runCommandFunc: func(clientID, sessionID, command string) (domain.CommandHandle, error) {
			gotClientID, gotSessionID, gotCommand = clientID, sessionID, command
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "run_command",
		Arguments: map[string]any{"sessionId": "s1", "command": "ls -la"},
	})
	if err != nil {
		t.Fatalf("CallTool(run_command) error = %v", err)
	}
	if gotClientID != "client-1" || gotSessionID != "s1" || gotCommand != "ls -la" {
		t.Errorf("RunCommand(clientID, sessionID, command) = (%q, %q, %q), want (client-1, s1, \"ls -la\")", gotClientID, gotSessionID, gotCommand)
	}

	var out runCommandOutput
	decodeToolResult(t, res, &out)
	if out.Handle.SessionID != "s1" || out.Handle.State != domain.CmdRunning {
		t.Errorf("Handle = %+v, want %+v", out.Handle, want)
	}
}

func TestHandleSession_RunCommandToolSurfacesInteractiveRejectionWithSuggestion(t *testing.T) {
	rejectErr := &aicontrol.InteractiveCommandError{Verb: "vim", Suggestion: "write the file non-interactively instead"}
	control := &fakeAIControlUseCase{
		runCommandFunc: func(clientID, sessionID, command string) (domain.CommandHandle, error) {
			return domain.CommandHandle{}, rejectErr
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "run_command",
		Arguments: map[string]any{"sessionId": "s1", "command": "vim notes.txt"},
	})
	if err != nil {
		t.Fatalf("CallTool(run_command) error = %v", err)
	}
	if !res.IsError {
		t.Fatal("result.IsError = false, want true for a rejected interactive command")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] = %T, want *mcp.TextContent", res.Content[0])
	}
	if !strings.Contains(text.Text, "vim") || !strings.Contains(text.Text, "write the file non-interactively instead") {
		t.Errorf("error text = %q, want it to mention the verb and the suggestion", text.Text)
	}
}

func TestHandleSession_RunCommandToolSurfacesDeniedAsError(t *testing.T) {
	control := &fakeAIControlUseCase{
		runCommandFunc: func(clientID, sessionID, command string) (domain.CommandHandle, error) {
			return domain.CommandHandle{}, errors.New("command denied by human approver")
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "run_command",
		Arguments: map[string]any{"sessionId": "s1", "command": "rm -rf /"},
	})
	if err != nil {
		t.Fatalf("CallTool(run_command) error = %v", err)
	}
	if !res.IsError {
		t.Fatal("result.IsError = false, want true for a denied command")
	}
}

func TestHandleSession_ReleaseControlToolForwardsSessionIDAndReturnsReleased(t *testing.T) {
	var gotClientID, gotSessionID string
	control := &fakeAIControlUseCase{
		releaseControlFunc: func(clientID, sessionID string) error {
			gotClientID, gotSessionID = clientID, sessionID
			return nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "release_control",
		Arguments: map[string]any{"sessionId": "s1"},
	})
	if err != nil {
		t.Fatalf("CallTool(release_control) error = %v", err)
	}
	if gotClientID != "client-1" || gotSessionID != "s1" {
		t.Errorf("ReleaseControl(clientID, sessionID) = (%q, %q), want (client-1, s1)", gotClientID, gotSessionID)
	}

	var out releaseControlOutput
	decodeToolResult(t, res, &out)
	if !out.Released {
		t.Error("Released = false, want true")
	}
}

func TestHandleSession_ListResourcesAdvertisesSessionsAndHosts(t *testing.T) {
	control := &fakeAIControlUseCase{}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	uris := make(map[string]bool, len(res.Resources))
	for _, r := range res.Resources {
		uris[r.URI] = true
	}
	for _, want := range []string{"momo-shell://sessions", "momo-shell://hosts"} {
		if !uris[want] {
			t.Errorf("resources/list missing %q, got %v", want, uris)
		}
	}
}

func TestHandleSession_ReadSessionsResource(t *testing.T) {
	want := []domain.SessionView{{ID: "s1", Kind: domain.KindLocal, State: domain.StateRunning}}
	control := &fakeAIControlUseCase{
		listSessionsFunc: func(clientID string) ([]domain.SessionView, error) { return want, nil },
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "momo-shell://sessions"})
	if err != nil {
		t.Fatalf("ReadResource(sessions) error = %v", err)
	}
	var out listSessionsOutput
	decodeResourceResult(t, res, &out)
	if len(out.Sessions) != 1 || out.Sessions[0].ID != "s1" {
		t.Errorf("Sessions = %+v, want one session with ID s1", out.Sessions)
	}
}

func TestHandleSession_ReadHostsResource(t *testing.T) {
	want := []domain.HostRef{{Name: "prod-1"}}
	control := &fakeAIControlUseCase{
		listHostsFunc: func(clientID string) ([]domain.HostRef, error) { return want, nil },
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "momo-shell://hosts"})
	if err != nil {
		t.Fatalf("ReadResource(hosts) error = %v", err)
	}
	var out listHostsOutput
	decodeResourceResult(t, res, &out)
	if len(out.Hosts) != 1 || out.Hosts[0].Name != "prod-1" {
		t.Errorf("Hosts = %+v, want one host named prod-1", out.Hosts)
	}
}

func TestHandleSession_ReadScrollbackResourceTemplateParsesSessionID(t *testing.T) {
	want := domain.MaskedChunk{Data: "buffered", NextSeq: 9}
	var gotSessionID string
	var gotSince uint64
	control := &fakeAIControlUseCase{
		readScrollbackFunc: func(clientID, sessionID string, sinceSeq uint64) (domain.MaskedChunk, error) {
			gotSessionID, gotSince = sessionID, sinceSeq
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "momo-shell://session/abc123/scrollback"})
	if err != nil {
		t.Fatalf("ReadResource(scrollback) error = %v", err)
	}
	if gotSessionID != "abc123" {
		t.Errorf("parsed sessionID = %q, want abc123", gotSessionID)
	}
	if gotSince != 0 {
		t.Errorf("resource read sinceSeq = %d, want 0 (full snapshot)", gotSince)
	}

	var out domain.MaskedChunk
	decodeResourceResult(t, res, &out)
	if out != want {
		t.Errorf("MaskedChunk = %+v, want %+v", out, want)
	}
}

func TestHandleSession_ScrollbackTemplateMalformedURIIsNotFound(t *testing.T) {
	control := &fakeAIControlUseCase{}
	session, _ := startDispatchSession(t, control, "client-1")

	// Matches the template's literal prefix but not its shape (no id
	// segment), so parseScrollbackSessionID must reject it.
	_, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "momo-shell://session//scrollback"})
	if err == nil {
		t.Fatal("ReadResource() error = nil, want a not-found error for a malformed scrollback URI")
	}
}

func TestHandleSession_ReturnsWhenConnCloses(t *testing.T) {
	control := &fakeAIControlUseCase{}
	_, done := startDispatchSession(t, control, "client-1")

	// t.Cleanup (registered inside startDispatchSession) already asserts
	// HandleSession returns after session.Close() -- this test just makes
	// that contract explicit as its own assertion for readability.
	select {
	case <-done:
		t.Fatal("HandleSession returned before the session was closed")
	default:
	}
}

// decodeToolResult unmarshals the single TextContent block a tool result
// carries (see textResult in dispatch.go) into v.
func decodeToolResult(t *testing.T, res *mcp.CallToolResult, v any) {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool result IsError = true, content = %+v", res.Content)
	}
	if len(res.Content) != 1 {
		t.Fatalf("tool result Content has %d blocks, want 1", len(res.Content))
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool result Content[0] = %T, want *mcp.TextContent", res.Content[0])
	}
	if err := json.Unmarshal([]byte(text.Text), v); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", text.Text, err)
	}
}

// decodeResourceResult unmarshals the single resource contents block (see
// jsonResource in dispatch.go) into v.
func decodeResourceResult(t *testing.T, res *mcp.ReadResourceResult, v any) {
	t.Helper()
	if len(res.Contents) != 1 {
		t.Fatalf("resource result Contents has %d blocks, want 1", len(res.Contents))
	}
	if err := json.Unmarshal([]byte(res.Contents[0].Text), v); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", res.Contents[0].Text, err)
	}
}
