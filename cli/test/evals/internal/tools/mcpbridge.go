package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// MCPBridge connects to the backend's streamable HTTP MCP mount (/mcp or
// /mcp/write) with a bearer PAT and exposes its tools to the model under the
// mcp__traceway__ prefix, as Claude Code names them, plus list/read tools for
// the server's knowledge resources.
type MCPBridge struct {
	Session      *mcp.ClientSession
	Instructions string
	// DefaultProject fills project_id when the model omits it, the way the
	// stdio server (traceway mcp) falls back to the CLI's current project.
	DefaultProject string
	tools          []*mcp.Tool
}

type bearerRT struct{ token string }

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// mcpCallTimeout bounds one MCP request. It is a per-call context, not an
// http.Client timeout: a client timeout would also cut the session's
// streams, and go-sdk gives up a session after a few reconnects that deliver
// nothing.
const mcpCallTimeout = 2 * time.Minute

// ConnectMCP opens a session to url with token. The bridge only makes
// requests and lists the tools once, so it opens no standalone SSE stream for
// server-initiated messages.
func ConnectMCP(ctx context.Context, url, token, defaultProject string) (*MCPBridge, error) {
	tr := &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: &http.Client{Transport: bearerRT{token}}, MaxRetries: 2, DisableStandaloneSSE: true}
	ctx, cancel := context.WithTimeout(ctx, mcpCallTimeout)
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "twevals", Version: "1"}, nil).Connect(ctx, tr, nil)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", url, err)
	}
	b := &MCPBridge{Session: cs, DefaultProject: defaultProject}
	if ir := cs.InitializeResult(); ir != nil {
		b.Instructions = ir.Instructions
	}
	for t, err := range cs.Tools(ctx, nil) {
		if err != nil {
			cs.Close()
			return nil, fmt.Errorf("list tools: %w", err)
		}
		b.tools = append(b.tools, t)
	}
	return b, nil
}

// Close ends the session.
func (b *MCPBridge) Close() { _ = b.Session.Close() }

// Tools returns one Tool per server tool plus the resource tools.
func (b *MCPBridge) Tools() []Tool {
	var out []Tool
	for _, t := range b.tools {
		out = append(out, &mcpTool{b: b, t: t})
	}
	return append(out, &mcpResources{b: b, list: true}, &mcpResources{b: b})
}

// ToolNames lists the server's tool names (unprefixed).
func (b *MCPBridge) ToolNames() []string {
	var out []string
	for _, t := range b.tools {
		out = append(out, t.Name)
	}
	return out
}

type mcpTool struct {
	b *MCPBridge
	t *mcp.Tool
}

func (m *mcpTool) Spec() Spec {
	params := map[string]any{"type": "object", "properties": map[string]any{}}
	if m.t.InputSchema != nil {
		raw, _ := json.Marshal(m.t.InputSchema)
		_ = json.Unmarshal(raw, &params)
	}
	return Spec{Name: ops.MCPPrefix + m.t.Name, Description: m.t.Description, Parameters: params}
}

func (m *mcpTool) Call(ctx context.Context, raw json.RawMessage) Result {
	args := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &args); err != nil {
			return errResult("invalid arguments: %v", err)
		}
	}
	if m.b.DefaultProject != "" && m.takesProject() {
		if _, ok := args["project_id"]; !ok {
			args["project_id"] = m.b.DefaultProject
		}
	}
	ctx, cancel := context.WithTimeout(ctx, mcpCallTimeout)
	defer cancel()
	res, err := m.b.Session.CallTool(ctx, &mcp.CallToolParams{Name: m.t.Name, Arguments: args})
	op := ops.ParseMCP(m.t.Name, args)
	if err != nil {
		out := "error: " + err.Error()
		return Result{Output: out, Ops: []record.OpRecord{{Op: op, OK: false, Output: out}}}
	}
	var b strings.Builder
	for _, c := range res.Content {
		switch tc := c.(type) {
		case *mcp.TextContent:
			b.WriteString(tc.Text)
		default:
			raw, _ := json.Marshal(c)
			b.Write(raw)
		}
	}
	text := b.String()
	if text == "" && res.StructuredContent != nil {
		raw, _ := json.Marshal(res.StructuredContent)
		text = string(raw)
	}
	out, trunc := CapAt(text, MaxMCPOutput)
	ok := !res.IsError
	return Result{Output: out, OK: ok, Truncated: trunc, Ops: []record.OpRecord{{Op: op, OK: ok, Output: text, Exit: map[bool]int{true: 0, false: 1}[ok]}}}
}

func (m *mcpTool) takesProject() bool {
	raw, _ := json.Marshal(m.t.InputSchema)
	return strings.Contains(string(raw), `"project_id"`)
}

type mcpResources struct {
	b    *MCPBridge
	list bool
}

func (r *mcpResources) Spec() Spec {
	if r.list {
		return Spec{Name: ops.MCPPrefix + "list_resources", Description: "List the Traceway MCP server's resources (its knowledge playbooks, traceway://knowledge/*).", Parameters: obj(map[string]any{})}
	}
	return Spec{Name: ops.MCPPrefix + "read_resource", Description: "Read one Traceway MCP resource by URI, e.g. traceway://knowledge/performance.",
		Parameters: obj(map[string]any{"uri": strProp("Resource URI")}, "uri")}
}

func (r *mcpResources) Call(ctx context.Context, raw json.RawMessage) Result {
	ctx, cancel := context.WithTimeout(ctx, mcpCallTimeout)
	defer cancel()
	if r.list {
		var b strings.Builder
		for res, err := range r.b.Session.Resources(ctx, nil) {
			if err != nil {
				return errResult("%v", err)
			}
			fmt.Fprintf(&b, "%s  %s: %s\n", res.URI, res.Name, res.Description)
		}
		return Result{Output: b.String(), OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpListRes}, OK: true}}}
	}
	var in struct {
		URI string `json:"uri"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	res, err := r.b.Session.ReadResource(ctx, &mcp.ReadResourceParams{URI: in.URI})
	op := ops.Op{Op: ops.OpReadResource, Args: map[string]any{"uri": in.URI}}
	if err != nil {
		return Result{Output: "error: " + err.Error(), Ops: []record.OpRecord{{Op: op}}}
	}
	var b strings.Builder
	for _, c := range res.Contents {
		b.WriteString(c.Text)
	}
	out, trunc := Cap(b.String())
	return Result{Output: out, OK: true, Truncated: trunc, Ops: []record.OpRecord{{Op: op, OK: true}}}
}
