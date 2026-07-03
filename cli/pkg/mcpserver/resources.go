package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/mcpserver/knowledge"
)

// knowledgeResources maps each embedded chunk to its resource URI and a
// description that tells the model when to read it.
var knowledgeResources = []struct {
	uri, file, name, description string
}{
	{
		uri: "traceway://knowledge/debug-flow", file: "debug-flow.md", name: "Debug flow",
		description: "The issue-debugging playbook: resolve a reference to a hash, why to fix the LAST occurrence rather than the group, triage, correlating by trace id, and reporting. Read before debugging an issue to root cause.",
	},
	{
		uri: "traceway://knowledge/performance", file: "performance.md", name: "Performance reference",
		description: "The authoritative performance investigation reference: the loop, reading the latency shape, marked-slow endpoint accounting, onset pinpointing, and the bottleneck checklist mapping waterfall shapes to causes. Read before any latency investigation.",
	},
	{
		uri: "traceway://knowledge/performance-flow", file: "performance-flow.md", name: "Performance flow summary",
		description: "The condensed 8-step performance loop. The full methodology and checklist live in traceway://knowledge/performance.",
	},
	{
		uri: "traceway://knowledge/url-resolution", file: "url-resolution.md", name: "Dashboard URL resolution",
		description: "How to resolve a pasted Traceway dashboard URL to the right lookup, including the ?t=, preset, from, and to query params. Read when the user pastes an instance URL.",
	},
	{
		uri: "traceway://knowledge/timestamps", file: "timestamps.md", name: "By-id lookup timestamps",
		description: "Why the by-id detail lookups require a timestamp (partition pruning), the lookup windows, and how to recover or estimate the timestamp when it is not handed to you.",
	},
	{
		uri: "traceway://knowledge/notifications", file: "notifications.md", name: "Notification resolution",
		description: "How to resolve a Traceway alert notification: exception notifications (Hash, Exception ID, Occurred at) versus performance notifications by ruleType. Read when the user pastes an alert.",
	},
	{
		uri: "traceway://knowledge/ground-rules", file: "ground-rules.md", name: "Ground rules",
		description: "Read/write posture, time-window discipline, and output conventions for operating a Traceway instance.",
	},
	{
		uri: "traceway://knowledge/query-recipes", file: "query-recipes.md", name: "Query recipes",
		description: "Ready-made triage queries: what is broken right now, what broke since a deploy, worst endpoint by latency, errors for one service.",
	},
	{
		uri: "traceway://knowledge/tool-map", file: "tool-map.md", name: "CLI to MCP tool map",
		description: "The 1:1 mapping between traceway CLI commands (used in the other resources' examples) and this server's MCP tools, plus the few deliberate differences.",
	},
}

func addResources(srv *mcp.Server) {
	for _, r := range knowledgeResources {
		text := knowledge.MustRead(r.file)
		uri := r.uri
		srv.AddResource(&mcp.Resource{
			URI:         uri,
			Name:        r.name,
			Description: r.description,
			MIMEType:    "text/markdown",
			Size:        int64(len(text)),
		}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: text}},
			}, nil
		})
	}
}
