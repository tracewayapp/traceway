// Package mcpserver is the Traceway MCP server: a thin tool/prompt/resource
// layer over pkg/client plus the embedded operator playbooks in knowledge/.
// It is transport-agnostic: the CLI runs it over stdio (traceway mcp) and a
// future backend mount can serve the same server over streamable HTTP by
// constructing one per authenticated session.
package mcpserver

import (
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/client"
	"github.com/tracewayapp/traceway/cli/pkg/mcpserver/knowledge"
)

// Config carries everything a server session needs. It holds no mutable
// state: project selection is per-call (a project_id param falling back to
// DefaultProjectID), which keeps one server per authenticated session safe
// for a multi-tenant HTTP mount.
type Config struct {
	// Client is the authenticated API client. Refresh behavior is the
	// client's own concern (client.WithRefresher); the server never touches
	// credentials.
	Client *client.Client
	// DefaultProjectID is used when a tool call passes no project_id. May be
	// empty: list_projects still works and every tool accepts project_id.
	DefaultProjectID string
	// InstanceURL is the Traceway instance origin, appended to the server
	// instructions so clients can validate pasted dashboard URLs and produce
	// links. May be empty.
	InstanceURL string
	// Version is the traceway build version reported to MCP clients.
	Version string
	// AuthHint is the remediation shown on token_expired errors. The caller
	// knows how credentials were supplied (CLI session, env token, HTTP
	// bearer) and what actually fixes a dead one; empty falls back to the
	// CLI-session advice.
	AuthHint         string
	PerRequestBearer bool
}

var baseInstructions = sync.OnceValue(func() string {
	return knowledge.MustRead("instructions.md")
})

// sharedSchemaCache warms an mcp.SchemaCache once, by registering the full
// tool surface against a throwaway server, so concurrent per-session New
// calls only ever read fully-resolved schemas and never race on Resolve.
var sharedSchemaCache = sync.OnceValue(func() *mcp.SchemaCache {
	cache := mcp.NewSchemaCache()
	warm := mcp.NewServer(
		&mcp.Implementation{Name: "traceway", Title: "Traceway", Version: "warmup"},
		&mcp.ServerOptions{SchemaCache: cache},
	)
	(&server{}).addTools(warm)
	return cache
})

// New builds an MCP server with the full Traceway tool/prompt/resource
// surface registered. The returned server is ready to Run on any transport.
func New(cfg Config) *mcp.Server {
	version := cfg.Version
	if version == "" {
		version = "dev"
	}
	instructions := baseInstructions()
	if cfg.InstanceURL != "" {
		instructions += "\n\nThe connected Traceway instance is " + cfg.InstanceURL + ". Dashboard URLs the user pastes should match this origin, and when citing a record for the user, link it there (e.g. " + cfg.InstanceURL + "/issues/<hash>)."
	}
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "traceway", Title: "Traceway", Version: version},
		&mcp.ServerOptions{Instructions: instructions, SchemaCache: sharedSchemaCache()},
	)
	s := &server{cfg: cfg}
	s.addTools(srv)
	addPrompts(srv)
	addResources(srv)
	return srv
}

type server struct {
	cfg Config
}
