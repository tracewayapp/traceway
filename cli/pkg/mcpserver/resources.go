package mcpserver

import (
	"context"
	"slices"

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
		uri: "traceway://knowledge/learn", file: "learn/overview.md", name: "Learn Traceway: overview",
		description: "How Traceway is organized (organizations, projects, roles, time windows), a map of every dashboard page, and which learn/* resource covers each part. Read when the user asks how Traceway works, where something is, or what a term means.",
	},
	{
		uri: "traceway://knowledge/learn/issues", file: "learn/issues.md", name: "Learn: issues and exceptions",
		description: "How exceptions are grouped into issues, what the issue and occurrence pages show, search, archiving and regressions.",
	},
	{
		uri: "traceway://knowledge/learn/endpoints-and-tasks", file: "learn/endpoints-and-tasks.md", name: "Learn: endpoints and tasks",
		description: "Endpoint and task analytics: percentiles, impact score, Apdex, slow-endpoint marks, request and task detail pages.",
	},
	{
		uri: "traceway://knowledge/learn/traces-and-logs", file: "learn/traces-and-logs.md", name: "Learn: spans, traces and logs",
		description: "The span explorer, whole-trace waterfalls across projects, the distributed trace card, and log search with severities and trace correlation.",
	},
	{
		uri: "traceway://knowledge/learn/metrics-and-dashboards", file: "learn/metrics-and-dashboards.md", name: "Learn: metrics and dashboards",
		description: "Metric types and aggregations (including rate), dashboards, widgets, templates, import and export, starring onto the home page.",
	},
	{
		uri: "traceway://knowledge/learn/sessions-and-ai", file: "learn/sessions-and-ai.md", name: "Learn: sessions, AI traces and profiles",
		description: "Browser and mobile sessions with replay, AI traces, conversations and users with flagged content, and continuous profiles.",
	},
	{
		uri: "traceway://knowledge/learn/monitors", file: "learn/monitors.md", name: "Learn: monitors, status pages and post-mortems",
		description: "Uptime monitors (http, tcp, browser), runs, incidents, public status pages with timeline updates, and post-mortem documents.",
	},
	{
		uri: "traceway://knowledge/learn/alerts", file: "learn/alerts.md", name: "Learn: alerts",
		description: "Every alert rule type and its settings, cooldowns, snoozing, channels and notification history.",
	},
	{
		uri: "traceway://knowledge/learn/on-call", file: "learn/on-call.md", name: "Learn: on-call",
		description: "Teams, schedules (layers, rotations, restrictions, overrides), escalation policies, pages and their lifecycle, contact methods and personal notification rules.",
	},
	{
		uri: "traceway://knowledge/learn/organization", file: "learn/organization.md", name: "Learn: organization, access and settings",
		description: "Organization views, projects and connection, roles and per-project overrides, account, personal access tokens, settings and agent access.",
	},
	{
		uri: "traceway://knowledge/tool-map", file: "tool-map.md", name: "CLI to MCP tool map",
		description: "The 1:1 mapping between traceway CLI commands (used in the other resources' examples) and this server's MCP tools, plus the few deliberate differences.",
	},
}

var writeResources = []struct {
	uri, file, name, description string
}{
	{
		uri: "traceway://knowledge/operations", file: "operations.md", name: "Operating with write access",
		description: "The three opt-in levels for agent writes, what each write tool changes, the rules for every write, and playbooks for pages, flapping alerts, confirming fixes, post-mortems and slow endpoints. Read before your first write.",
	},
}

func addResources(srv *mcp.Server, allowWrite bool) {
	resources := knowledgeResources
	if allowWrite {
		resources = append(slices.Clone(knowledgeResources), writeResources...)
	}
	for _, r := range resources {
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
