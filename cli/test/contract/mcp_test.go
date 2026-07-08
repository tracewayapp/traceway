package contract

import (
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	client "github.com/tracewayapp/traceway/cli/pkg/client"
	"github.com/tracewayapp/traceway/cli/pkg/mcpserver"
)

// TestContract_mcpToolSurface drives every MCP tool once against the real
// booted backend through an in-memory MCP session. The per-endpoint goldens
// above pin the wire shapes; this asserts the MCP layer's parameter plumbing
// (project fallback, time ranges, mandatory timestamps) works end to end.
func TestContract_mcpToolSurface(t *testing.T) {
	srv := mcpserver.New(mcpserver.Config{
		Client:           client.New(baseURL, client.WithJWT(jwtToken)),
		DefaultProjectID: projectID,
		InstanceURL:      baseURL,
		Version:          "contract",
	})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(t.Context(), serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "contract", Version: "0"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	from := seedAt.Add(-time.Hour).Format(time.RFC3339)
	to := seedAt.Add(time.Hour).Format(time.RFC3339)
	at := seedAt.Format(time.RFC3339)

	call := func(name string, args map[string]any, wantSubstr string) {
		t.Helper()
		res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: protocol error: %v", name, err)
		}
		var text strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text.WriteString(tc.Text)
			}
		}
		if res.IsError {
			t.Fatalf("%s: tool error: %s", name, text.String())
		}
		if wantSubstr != "" && !strings.Contains(text.String(), wantSubstr) {
			t.Errorf("%s: result missing %q: %s", name, wantSubstr, clip([]byte(text.String())))
		}
	}

	call("list_projects", nil, projectID)
	call("list_exceptions", map[string]any{"from": from, "to": to}, seedHash)
	call("get_exception", map[string]any{"hash": seedHash}, seedExceptionID.String())
	call("get_exception_occurrence", map[string]any{"id": seedExceptionID.String(), "recorded_at": at}, seedHash)
	call("query_logs", map[string]any{"from": from, "to": to, "service_name": "contract-svc"}, "contract-svc")
	call("list_endpoints", map[string]any{"from": from, "to": to}, "GET /api/contract")
	call("get_endpoint_request", map[string]any{"id": seedEndpointID.String(), "recorded_at": at}, seedEndpointID.String())
	call("endpoints_chart", map[string]any{"from": from, "to": to}, "endpoints")
	call("get_slow_endpoint_config", map[string]any{"endpoint": "GET /api/contract"}, "offsetMs")
	call("query_metrics", map[string]any{
		"from": from, "to": to, "interval_minutes": 60,
		"queries": []map[string]any{{"name": seedMetricName}},
	}, seedMetricName)
	call("get_task", map[string]any{"id": seedTaskID.String(), "recorded_at": at}, seedTaskID.String())
	call("get_ai_trace", map[string]any{"id": seedAiTraceID.String(), "recorded_at": at}, seedAiTraceID.String())
	call("get_session", map[string]any{"id": seedSessionID.String(), "started_at": at}, seedSessionID.String())
	call("get_trace", map[string]any{"id": seedTraceID.String(), "recorded_at": at}, seedTraceID.String())

	// Archive round trip last, restoring state for any test that follows.
	call("archive_exceptions", map[string]any{"hashes": []string{seedHash}}, "archived")
	call("unarchive_exceptions", map[string]any{"hashes": []string{seedHash}}, "unarchived")
}
