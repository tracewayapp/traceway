package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/mcpserver/knowledge"
)

// Prompts render the knowledge chunks with the user's argument inlined, so
// the playbooks the prompts teach cannot drift from the skill or resources.

func addPrompts(srv *mcp.Server) {
	srv.AddPrompt(&mcp.Prompt{
		Name:        "debug_issue",
		Title:       "Debug a Traceway issue to root cause",
		Description: "Resolve an issue reference (hash, dashboard URL, title, or free-form bug description), investigate it with the Traceway tools, and drive it to root cause.",
		Arguments: []*mcp.PromptArgument{{
			Name: "reference", Required: true,
			Description: "The issue to debug: a 16-hex exception hash, a dashboard URL, an error title or message to search for, or a free-form bug description.",
		}},
	}, promptHandler(func(args map[string]string) string {
		return fmt.Sprintf(
			"Debug this Traceway issue to root cause: %s\n\nFollow this playbook, using the traceway MCP tools (translate CLI commands via traceway://knowledge/tool-map; URLs resolve per traceway://knowledge/url-resolution):\n\n%s",
			args["reference"], knowledge.MustRead("debug-flow.md"))
	}))

	srv.AddPrompt(&mcp.Prompt{
		Name:        "investigate_performance",
		Title:       "Investigate latency or slowness",
		Description: "Quantify a slow endpoint or symptom, pinpoint when it started, localize the cost to a span, and map it to a known bottleneck.",
		Arguments: []*mcp.PromptArgument{{
			Name: "target", Required: true,
			Description: "The endpoint name or the slowness symptom to investigate.",
		}},
	}, promptHandler(func(args map[string]string) string {
		return fmt.Sprintf(
			"Investigate this performance problem: %s\n\nFirst read traceway://knowledge/performance (the authoritative methodology and bottleneck checklist), then run this loop with the traceway MCP tools (translate CLI commands via traceway://knowledge/tool-map):\n\n%s",
			args["target"], knowledge.MustRead("performance-flow.md"))
	}))

	srv.AddPrompt(&mcp.Prompt{
		Name:        "whats_broken",
		Title:       "What is broken right now",
		Description: "Triage the current state of a project: recent exceptions, error logs, and slow endpoints.",
		Arguments: []*mcp.PromptArgument{{
			Name: "window", Required: false,
			Description: "Time window to inspect, like 1h (default) or 24h.",
		}},
	}, promptHandler(func(args map[string]string) string {
		window := args["window"]
		if window == "" {
			window = "1h"
		}
		return fmt.Sprintf(
			"Triage what is broken in the last %s: list recent exceptions ordered by lastSeen, error-severity logs (min_severity 17), and the worst endpoints by impact, then summarize what deserves attention and why. Use the traceway MCP tools with since=%q. These recipes show the shape of each query (CLI syntax; translate via traceway://knowledge/tool-map):\n\n%s",
			window, window, knowledge.MustRead("query-recipes.md"))
	}))

	srv.AddPrompt(&mcp.Prompt{
		Name:        "resolve_notification",
		Title:       "Resolve a Traceway alert notification",
		Description: "Turn a pasted Traceway notification (email, Slack, or webhook) into the right investigation: an exception drill-down or a performance analysis.",
		Arguments: []*mcp.PromptArgument{{
			Name: "notification_text", Required: true,
			Description: "The notification body as pasted by the user.",
		}},
	}, promptHandler(func(args map[string]string) string {
		return fmt.Sprintf(
			"Resolve this Traceway notification and investigate what it reports:\n\n---\n%s\n---\n\nClassify and resolve it per this guide, using the traceway MCP tools (translate CLI commands via traceway://knowledge/tool-map):\n\n%s",
			args["notification_text"], knowledge.MustRead("notifications.md"))
	}))
}

func promptHandler(render func(args map[string]string) string) mcp.PromptHandler {
	return func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: render(req.Params.Arguments)},
			}},
		}, nil
	}
}
