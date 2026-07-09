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
// Every method dispatch exposes has an injectable func field; a test only
// sets the fields for the tools it exercises (an unset field nil-panics,
// surfacing an unexpected call).
type fakeAIControlUseCase struct {
	listSessionsFunc           func(clientID string) ([]domain.SessionView, error)
	listHostsFunc              func(clientID string) ([]domain.HostRef, error)
	readScrollbackFunc         func(clientID, sessionID string, sinceSeq uint64) (domain.MaskedChunk, error)
	requestControlFunc         func(clientID, sessionID string, scope domain.ControlScope) (domain.Delegation, error)
	runCommandFunc             func(clientID, sessionID, command string) (domain.CommandHandle, error)
	releaseControlFunc         func(clientID, sessionID string) error
	connectHostFunc            func(clientID, hostName string) (domain.SessionView, error)
	requestConnectionScopeFunc func(clientID string, hostNames []string) (domain.ConnectionScope, error)
	getShellStateFunc          func(clientID, sessionID string) (domain.ShellState, error)
	resetShellFunc             func(clientID, sessionID string) error
	cancelCommandFunc          func(clientID, sessionID string) (domain.CommandHandle, error)
	backgroundCommandFunc      func(clientID, sessionID string) (domain.CommandHandle, error)
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
	return f.connectHostFunc(clientID, hostName)
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
	return f.requestConnectionScopeFunc(clientID, hostNames)
}

func (f *fakeAIControlUseCase) GetShellState(clientID, sessionID string) (domain.ShellState, error) {
	return f.getShellStateFunc(clientID, sessionID)
}

func (f *fakeAIControlUseCase) ResetShell(clientID, sessionID string) error {
	return f.resetShellFunc(clientID, sessionID)
}

func (f *fakeAIControlUseCase) CancelCommand(clientID, sessionID string) (domain.CommandHandle, error) {
	return f.cancelCommandFunc(clientID, sessionID)
}

func (f *fakeAIControlUseCase) BackgroundCommand(clientID, sessionID string) (domain.CommandHandle, error) {
	return f.backgroundCommandFunc(clientID, sessionID)
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
		"connect_host", "request_connection_scope", "get_shell_state",
		"reset_shell", "cancel_command", "background_command",
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

func TestHandleSession_ConnectHostToolForwardsHostNameAndDecodesSession(t *testing.T) {
	want := domain.SessionView{ID: "s9", Kind: domain.KindSSH, HostID: "h1", State: domain.StateConnecting, Controlled: true}
	var gotClientID, gotHostName string
	control := &fakeAIControlUseCase{
		connectHostFunc: func(clientID, hostName string) (domain.SessionView, error) {
			gotClientID, gotHostName = clientID, hostName
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "connect_host",
		Arguments: map[string]any{"hostName": "prod-1"},
	})
	if err != nil {
		t.Fatalf("CallTool(connect_host) error = %v", err)
	}
	if gotClientID != "client-1" || gotHostName != "prod-1" {
		t.Errorf("ConnectHost(clientID, hostName) = (%q, %q), want (client-1, prod-1)", gotClientID, gotHostName)
	}

	var out connectHostOutput
	decodeToolResult(t, res, &out)
	if out.Session.ID != "s9" || !out.Session.Controlled {
		t.Errorf("Session = %+v, want %+v", out.Session, want)
	}
}

func TestHandleSession_ConnectHostToolSurfacesDenialAsError(t *testing.T) {
	control := &fakeAIControlUseCase{
		connectHostFunc: func(clientID, hostName string) (domain.SessionView, error) {
			return domain.SessionView{}, errors.New("connect denied by human approver")
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "connect_host",
		Arguments: map[string]any{"hostName": "prod-1"},
	})
	if err != nil {
		t.Fatalf("CallTool(connect_host) error = %v", err)
	}
	if !res.IsError {
		t.Fatal("result.IsError = false, want true for a denied connect request")
	}
}

func TestHandleSession_RequestConnectionScopeToolForwardsHostsAndFlattensNames(t *testing.T) {
	want := domain.ConnectionScope{
		ID:            "scope-1",
		ClientID:      "client-1",
		HostNames:     map[string]bool{"web-2": true, "db-1": true, "web-1": true},
		MaxConcurrent: 3,
	}
	var gotClientID string
	var gotHostNames []string
	control := &fakeAIControlUseCase{
		requestConnectionScopeFunc: func(clientID string, hostNames []string) (domain.ConnectionScope, error) {
			gotClientID, gotHostNames = clientID, hostNames
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "request_connection_scope",
		Arguments: map[string]any{"hostNames": []string{"web-1", "db-1", "web-2"}},
	})
	if err != nil {
		t.Fatalf("CallTool(request_connection_scope) error = %v", err)
	}
	if gotClientID != "client-1" {
		t.Errorf("RequestConnectionScope clientID = %q, want client-1", gotClientID)
	}
	if strings.Join(gotHostNames, ",") != "web-1,db-1,web-2" {
		t.Errorf("forwarded hostNames = %v, want [web-1 db-1 web-2] in order", gotHostNames)
	}

	var out requestConnectionScopeOutput
	decodeToolResult(t, res, &out)
	if out.ScopeID != "scope-1" || out.MaxConcurrent != 3 {
		t.Errorf("scope = %+v, want ScopeID scope-1 / MaxConcurrent 3", out)
	}
	if strings.Join(out.HostNames, ",") != "db-1,web-1,web-2" {
		t.Errorf("output hostNames = %v, want sorted [db-1 web-1 web-2]", out.HostNames)
	}
}

func TestHandleSession_RequestConnectionScopeToolSurfacesDenialAsError(t *testing.T) {
	control := &fakeAIControlUseCase{
		requestConnectionScopeFunc: func(clientID string, hostNames []string) (domain.ConnectionScope, error) {
			return domain.ConnectionScope{}, errors.New("connection scope denied by human approver")
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "request_connection_scope",
		Arguments: map[string]any{"hostNames": []string{"prod-1"}},
	})
	if err != nil {
		t.Fatalf("CallTool(request_connection_scope) error = %v", err)
	}
	if !res.IsError {
		t.Fatal("result.IsError = false, want true for a denied scope request")
	}
}

func TestHandleSession_GetShellStateToolForwardsSessionIDAndDecodesState(t *testing.T) {
	want := domain.ShellState{SessionID: "s1", Cwd: "/home/me", Env: map[string]string{"HOME": "/home/me"}}
	var gotClientID, gotSessionID string
	control := &fakeAIControlUseCase{
		getShellStateFunc: func(clientID, sessionID string) (domain.ShellState, error) {
			gotClientID, gotSessionID = clientID, sessionID
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_shell_state",
		Arguments: map[string]any{"sessionId": "s1"},
	})
	if err != nil {
		t.Fatalf("CallTool(get_shell_state) error = %v", err)
	}
	if gotClientID != "client-1" || gotSessionID != "s1" {
		t.Errorf("GetShellState(clientID, sessionID) = (%q, %q), want (client-1, s1)", gotClientID, gotSessionID)
	}

	var out getShellStateOutput
	decodeToolResult(t, res, &out)
	if out.ShellState.Cwd != "/home/me" || out.ShellState.Env["HOME"] != "/home/me" {
		t.Errorf("ShellState = %+v, want %+v", out.ShellState, want)
	}
}

func TestHandleSession_GetShellStateToolSurfacesNotDelegatedAsError(t *testing.T) {
	control := &fakeAIControlUseCase{
		getShellStateFunc: func(clientID, sessionID string) (domain.ShellState, error) {
			return domain.ShellState{}, errors.New("aicontrol: session not delegated to this client")
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_shell_state",
		Arguments: map[string]any{"sessionId": "s1"},
	})
	if err != nil {
		t.Fatalf("CallTool(get_shell_state) error = %v", err)
	}
	if !res.IsError {
		t.Fatal("result.IsError = false, want true for a non-delegated session")
	}
}

func TestHandleSession_ResetShellToolForwardsSessionIDAndReturnsReset(t *testing.T) {
	var gotClientID, gotSessionID string
	control := &fakeAIControlUseCase{
		resetShellFunc: func(clientID, sessionID string) error {
			gotClientID, gotSessionID = clientID, sessionID
			return nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "reset_shell",
		Arguments: map[string]any{"sessionId": "s1"},
	})
	if err != nil {
		t.Fatalf("CallTool(reset_shell) error = %v", err)
	}
	if gotClientID != "client-1" || gotSessionID != "s1" {
		t.Errorf("ResetShell(clientID, sessionID) = (%q, %q), want (client-1, s1)", gotClientID, gotSessionID)
	}

	var out resetShellOutput
	decodeToolResult(t, res, &out)
	if !out.Reset {
		t.Error("Reset = false, want true")
	}
}

func TestHandleSession_CancelCommandToolForwardsSessionIDAndDecodesHandle(t *testing.T) {
	want := *domain.NewCommandHandle("s1", "sleep 100")
	var gotClientID, gotSessionID string
	control := &fakeAIControlUseCase{
		cancelCommandFunc: func(clientID, sessionID string) (domain.CommandHandle, error) {
			gotClientID, gotSessionID = clientID, sessionID
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "cancel_command",
		Arguments: map[string]any{"sessionId": "s1"},
	})
	if err != nil {
		t.Fatalf("CallTool(cancel_command) error = %v", err)
	}
	if gotClientID != "client-1" || gotSessionID != "s1" {
		t.Errorf("CancelCommand(clientID, sessionID) = (%q, %q), want (client-1, s1)", gotClientID, gotSessionID)
	}

	var out cancelCommandOutput
	decodeToolResult(t, res, &out)
	if out.Handle.SessionID != "s1" || out.Handle.State != domain.CmdRunning {
		t.Errorf("Handle = %+v, want a running handle for s1", out.Handle)
	}
}

func TestHandleSession_CancelCommandToolSurfacesNoActiveCommandAsError(t *testing.T) {
	control := &fakeAIControlUseCase{
		cancelCommandFunc: func(clientID, sessionID string) (domain.CommandHandle, error) {
			return domain.CommandHandle{}, errors.New("aicontrol: no active command")
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "cancel_command",
		Arguments: map[string]any{"sessionId": "s1"},
	})
	if err != nil {
		t.Fatalf("CallTool(cancel_command) error = %v", err)
	}
	if !res.IsError {
		t.Fatal("result.IsError = false, want true when there is no active command")
	}
}

func TestHandleSession_BackgroundCommandToolForwardsSessionIDAndDecodesHandle(t *testing.T) {
	want := domain.CommandHandle{SessionID: "s1", Command: "sleep 100", State: domain.CmdBackground, Seq: 2}
	var gotClientID, gotSessionID string
	control := &fakeAIControlUseCase{
		backgroundCommandFunc: func(clientID, sessionID string) (domain.CommandHandle, error) {
			gotClientID, gotSessionID = clientID, sessionID
			return want, nil
		},
	}
	session, _ := startDispatchSession(t, control, "client-1")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "background_command",
		Arguments: map[string]any{"sessionId": "s1"},
	})
	if err != nil {
		t.Fatalf("CallTool(background_command) error = %v", err)
	}
	if gotClientID != "client-1" || gotSessionID != "s1" {
		t.Errorf("BackgroundCommand(clientID, sessionID) = (%q, %q), want (client-1, s1)", gotClientID, gotSessionID)
	}

	var out backgroundCommandOutput
	decodeToolResult(t, res, &out)
	if out.Handle.State != domain.CmdBackground {
		t.Errorf("Handle.State = %q, want %q", out.Handle.State, domain.CmdBackground)
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
