package mcpipc

import (
	"context"
	"encoding/json"
	"io"
	"net"
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
// in.AIControlUseCase under the connection's clientID (doc 20 D3). This
// cycle's scope is the read-only slice (ListSessions/ListHosts/
// ReadScrollback) -- control/connect/run tools are added once A7/A8 make
// their human-approval flows end-to-end testable.
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
