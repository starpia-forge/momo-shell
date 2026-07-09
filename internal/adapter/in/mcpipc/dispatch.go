package mcpipc

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

const (
	serverName    = "momo-shell"
	serverVersion = "0.1.0"

	scrollbackURIPrefix = "momo-shell://session/"
	scrollbackURISuffix = "/scrollback"
)

// Dispatch is the real SessionHandler (A6): it hosts one mcp.Server per
// authenticated connection, mapping MCP tools/resources to
// in.AIControlUseCase under the connection's clientID (doc 20 D3). The full
// AIControlUseCase surface is now exposed as MCP tools: the read-only slice
// (ListSessions/ListHosts/ReadScrollback), the control loop
// (RequestControl/RunCommand/ReleaseControl), and the remaining session
// operations -- connect_host, request_connection_scope (fan-out), get_shell_
// state/reset_shell, and cancel_command/background_command.
type Dispatch struct {
	control in.AIControlUseCase
}

// NewDispatch builds a Dispatch calling control for every tool/resource.
func NewDispatch(control in.AIControlUseCase) *Dispatch {
	return &Dispatch{control: control}
}

var _ SessionHandler = (*Dispatch)(nil)

// HandleSession implements SessionHandler: it builds a fresh mcp.Server
// bound to clientID (captured by the handlers below) and serves MCP over
// conn until the connection closes -- Server.Run returns on conn EOF/close,
// which is also how mcpipc.Server.Stop's force-close unblocks this call.
func (d *Dispatch) HandleSession(clientID string, conn net.Conn) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	d.registerTools(server, clientID)
	d.registerResources(server, clientID)

	transport := &mcp.IOTransport{Reader: conn, Writer: nopWriteCloser{conn}}
	_ = server.Run(context.Background(), transport)
}

// --- tools -------------------------------------------------------------

type listSessionsInput struct{}

type listSessionsOutput struct {
	Sessions []domain.SessionView `json:"sessions"`
}

type listHostsInput struct{}

type listHostsOutput struct {
	Hosts []domain.HostRef `json:"hosts"`
}

type readOutputInput struct {
	SessionID string `json:"sessionId"`
	SinceSeq  uint64 `json:"sinceSeq,omitempty"`
}

type requestControlInput struct {
	SessionID  string `json:"sessionId"`
	HostOnly   bool   `json:"hostOnly,omitempty"`
	PathPrefix string `json:"pathPrefix,omitempty"`
	ReadOnly   bool   `json:"readOnly,omitempty"`
}

type requestControlOutput struct {
	Delegation domain.Delegation `json:"delegation"`
}

type runCommandInput struct {
	SessionID string `json:"sessionId"`
	Command   string `json:"command"`
}

type runCommandOutput struct {
	Handle domain.CommandHandle `json:"handle"`
}

type releaseControlInput struct {
	SessionID string `json:"sessionId"`
}

type releaseControlOutput struct {
	Released bool `json:"released"`
}

type connectHostInput struct {
	HostName string `json:"hostName"`
}

type connectHostOutput struct {
	Session domain.SessionView `json:"session"`
}

type requestConnectionScopeInput struct {
	HostNames []string `json:"hostNames"`
}

// requestConnectionScopeOutput re-projects domain.ConnectionScope: its
// HostNames is a map[string]bool (all true) that would serialize as a JSON
// object, so it is flattened to a sorted array an AI consumer can read.
type requestConnectionScopeOutput struct {
	ScopeID       string   `json:"scopeId"`
	HostNames     []string `json:"hostNames"`
	MaxConcurrent int      `json:"maxConcurrent"`
}

type getShellStateInput struct {
	SessionID string `json:"sessionId"`
}

type getShellStateOutput struct {
	ShellState domain.ShellState `json:"shellState"`
}

type resetShellInput struct {
	SessionID string `json:"sessionId"`
}

type resetShellOutput struct {
	Reset bool `json:"reset"`
}

type cancelCommandInput struct {
	SessionID string `json:"sessionId"`
}

type cancelCommandOutput struct {
	Handle domain.CommandHandle `json:"handle"`
}

type backgroundCommandInput struct {
	SessionID string `json:"sessionId"`
}

type backgroundCommandOutput struct {
	Handle domain.CommandHandle `json:"handle"`
}

func (d *Dispatch) registerTools(server *mcp.Server, clientID string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sessions",
		Description: "List every open shell session's AI-facing view (state and control-delegation flag).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listSessionsInput) (*mcp.CallToolResult, listSessionsOutput, error) {
		sessions, err := d.control.ListSessions(clientID)
		if err != nil {
			return nil, listSessionsOutput{}, err
		}
		out := listSessionsOutput{Sessions: sessions}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_hosts",
		Description: "List every saved host's name and labels. Credentials are never included.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listHostsInput) (*mcp.CallToolResult, listHostsOutput, error) {
		hosts, err := d.control.ListHosts(clientID)
		if err != nil {
			return nil, listHostsOutput{}, err
		}
		out := listHostsOutput{Hosts: hosts}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_output",
		Description: "Read a masked slice of a session's scrollback since a given cursor (sinceSeq).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in readOutputInput) (*mcp.CallToolResult, domain.MaskedChunk, error) {
		chunk, err := d.control.ReadScrollback(clientID, in.SessionID, in.SinceSeq)
		if err != nil {
			return nil, domain.MaskedChunk{}, err
		}
		return textResult(chunk), chunk, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "request_control",
		Description: "Request control delegation over an existing session, optionally scoped (hostOnly/pathPrefix/readOnly). Blocks for human approval (up to 60s) unless already delegated.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in requestControlInput) (*mcp.CallToolResult, requestControlOutput, error) {
		scope := domain.ControlScope{HostOnly: in.HostOnly, PathPrefix: in.PathPrefix, ReadOnly: in.ReadOnly}
		deleg, err := d.control.RequestControl(clientID, in.SessionID, scope)
		if err != nil {
			return nil, requestControlOutput{}, err
		}
		out := requestControlOutput{Delegation: deleg}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_command",
		Description: "Inject a command into a delegated session. Low-risk commands auto-run; higher-risk or uncertain commands block for human approval (up to 60s); interactive commands (editors/pagers/monitors/REPLs/prompting package managers) are rejected with a non-interactive alternative suggestion instead of running.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in runCommandInput) (*mcp.CallToolResult, runCommandOutput, error) {
		handle, err := d.control.RunCommand(clientID, in.SessionID, in.Command)
		if err != nil {
			return nil, runCommandOutput{}, err
		}
		out := runCommandOutput{Handle: handle}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "release_control",
		Description: "Voluntarily release this client's control delegation over a session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in releaseControlInput) (*mcp.CallToolResult, releaseControlOutput, error) {
		if err := d.control.ReleaseControl(clientID, in.SessionID); err != nil {
			return nil, releaseControlOutput{}, err
		}
		out := releaseControlOutput{Released: true}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "connect_host",
		Description: "Open a fresh SSH session to a saved host by name (from list_hosts). Credentials never cross this boundary -- the custodian fills them. Blocks for human approval (up to 60s) unless a request_connection_scope grant already covers the host.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in connectHostInput) (*mcp.CallToolResult, connectHostOutput, error) {
		session, err := d.control.ConnectHost(clientID, in.HostName)
		if err != nil {
			return nil, connectHostOutput{}, err
		}
		out := connectHostOutput{Session: session}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "request_connection_scope",
		Description: "Batch pre-authorize connect_host over a named host set (fan-out). Blocks for one human approval (up to 60s); afterwards connect_host to those hosts auto-grants up to the returned concurrent-session cap.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in requestConnectionScopeInput) (*mcp.CallToolResult, requestConnectionScopeOutput, error) {
		scope, err := d.control.RequestConnectionScope(clientID, in.HostNames)
		if err != nil {
			return nil, requestConnectionScopeOutput{}, err
		}
		hostNames := make([]string, 0, len(scope.HostNames))
		for name := range scope.HostNames {
			hostNames = append(hostNames, name)
		}
		sort.Strings(hostNames)
		out := requestConnectionScopeOutput{ScopeID: scope.ID, HostNames: hostNames, MaxConcurrent: scope.MaxConcurrent}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_shell_state",
		Description: "Snapshot a delegated session's shell state (cwd and key env vars) via an on-demand probe.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in getShellStateInput) (*mcp.CallToolResult, getShellStateOutput, error) {
		state, err := d.control.GetShellState(clientID, in.SessionID)
		if err != nil {
			return nil, getShellStateOutput{}, err
		}
		out := getShellStateOutput{ShellState: state}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reset_shell",
		Description: "Reset a delegated session's shell to a clean state (interrupt the current line, return to the home directory).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in resetShellInput) (*mcp.CallToolResult, resetShellOutput, error) {
		if err := d.control.ResetShell(clientID, in.SessionID); err != nil {
			return nil, resetShellOutput{}, err
		}
		out := resetShellOutput{Reset: true}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cancel_command",
		Description: "Interrupt the in-flight command in a delegated session (sends Ctrl-C). Returns the running command's handle; its terminal state follows asynchronously.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in cancelCommandInput) (*mcp.CallToolResult, cancelCommandOutput, error) {
		handle, err := d.control.CancelCommand(clientID, in.SessionID)
		if err != nil {
			return nil, cancelCommandOutput{}, err
		}
		out := cancelCommandOutput{Handle: handle}
		return textResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "background_command",
		Description: "Send the in-flight command in a delegated session to the background (Ctrl-Z then bg). Returns the handle in its new background terminal state.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in backgroundCommandInput) (*mcp.CallToolResult, backgroundCommandOutput, error) {
		handle, err := d.control.BackgroundCommand(clientID, in.SessionID)
		if err != nil {
			return nil, backgroundCommandOutput{}, err
		}
		out := backgroundCommandOutput{Handle: handle}
		return textResult(out), out, nil
	})
}

// textResult renders v as a single JSON text content block, mirroring what
// AddTool would otherwise leave to StructuredContent alone -- callers that
// only read Content (rather than the typed Out) still see the data.
func textResult(v any) *mcp.CallToolResult {
	b, err := json.Marshal(v)
	if err != nil {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

// --- resources -----------------------------------------------------------

func (d *Dispatch) registerResources(server *mcp.Server, clientID string) {
	server.AddResource(&mcp.Resource{
		URI:      "momo-shell://sessions",
		Name:     "sessions",
		MIMEType: "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		sessions, err := d.control.ListSessions(clientID)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, listSessionsOutput{Sessions: sessions})
	})

	server.AddResource(&mcp.Resource{
		URI:      "momo-shell://hosts",
		Name:     "hosts",
		MIMEType: "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		hosts, err := d.control.ListHosts(clientID)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, listHostsOutput{Hosts: hosts})
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: scrollbackURIPrefix + "{id}" + scrollbackURISuffix,
		Name:        "session-scrollback",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		sessionID, ok := parseScrollbackSessionID(req.Params.URI)
		if !ok {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		// Resources are side-effect-free snapshot reads (doc 16/17): always
		// the full scrollback so far. Incremental polling via a cursor is
		// the read_output tool's job, not this resource's.
		chunk, err := d.control.ReadScrollback(clientID, sessionID, 0)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, chunk)
	})
}

// parseScrollbackSessionID extracts {id} from a concrete
// momo-shell://session/{id}/scrollback URI (the SDK matches the template to
// route here but does not bind template variables itself).
func parseScrollbackSessionID(uri string) (string, bool) {
	if !strings.HasPrefix(uri, scrollbackURIPrefix) || !strings.HasSuffix(uri, scrollbackURISuffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(uri, scrollbackURIPrefix), scrollbackURISuffix)
	if id == "" {
		return "", false
	}
	return id, true
}

func jsonResource(uri string, v any) (*mcp.ReadResourceResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(b),
		}},
	}, nil
}

// --- shared IOTransport helper -------------------------------------------

// nopWriteCloser adapts an io.Writer that must not be closed (because
// something else -- typically the same net.Conn used as the Reader -- owns
// the single Close) into an io.WriteCloser for mcp.IOTransport.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
