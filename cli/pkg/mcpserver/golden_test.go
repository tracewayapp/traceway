package mcpserver

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/internal/apifixture"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden tool outputs")

// TestToolOutputsMatchGolden pins the structured result of every tool for a
// single-source profile. The goldens were recorded before the tools moved
// onto the access layer; the refactor must not change what a client sees.
func TestToolOutputsMatchGolden(t *testing.T) {
	fx := apifixture.Known
	api := apifixture.New(t)
	srv := New(Config{
		Client:           client.New(api.URL, client.WithJWT("token")),
		DefaultProjectID: fx.ProjectID,
		InstanceURL:      api.URL,
		Version:          "golden",
	})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(t.Context(), serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "golden", Version: "0"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	from, to, at := fx.From.Format(time.RFC3339), fx.To.Format(time.RFC3339), fx.At.Format(time.RFC3339)
	calls := []struct {
		tool string
		args map[string]any
	}{
		{"list_projects", nil},
		{"list_exceptions", map[string]any{"from": from, "to": to}},
		{"get_exception", map[string]any{"hash": fx.ExceptionHash}},
		{"get_exception_occurrence", map[string]any{"id": fx.OccurrenceID.String(), "recorded_at": at}},
		{"query_logs", map[string]any{"from": from, "to": to, "min_severity": 17}},
		{"list_endpoints", map[string]any{"from": from, "to": to}},
		{"get_endpoint_request", map[string]any{"id": fx.RequestID.String(), "recorded_at": at}},
		{"endpoints_chart", map[string]any{"from": from, "to": to}},
		{"get_slow_endpoint_config", map[string]any{"endpoint": fx.EndpointName}},
		{"query_metrics", map[string]any{"from": from, "to": to, "queries": []map[string]any{{"name": fx.MetricName}}}},
		{"get_task", map[string]any{"id": fx.TaskID.String(), "recorded_at": at}},
		{"get_ai_trace", map[string]any{"id": fx.AITraceID.String(), "recorded_at": at}},
		{"get_session", map[string]any{"id": fx.SessionID.String(), "started_at": at}},
		{"get_trace", map[string]any{"id": fx.TraceID.String(), "recorded_at": at}},
		{"archive_exceptions", map[string]any{"hashes": []string{fx.ExceptionHash}}},
		{"unarchive_exceptions", map[string]any{"hashes": []string{fx.ExceptionHash}}},
	}
	for _, call := range calls {
		t.Run(call.tool, func(t *testing.T) {
			res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: call.tool, Arguments: call.args})
			if err != nil {
				t.Fatalf("protocol error: %v", err)
			}
			var text strings.Builder
			for _, c := range res.Content {
				if tc, ok := c.(*mcp.TextContent); ok {
					text.WriteString(tc.Text)
				}
			}
			if res.IsError {
				t.Fatalf("tool error: %s", text.String())
			}
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, []byte(text.String()), "", "  "); err != nil {
				t.Fatalf("result is not JSON: %v\n%s", err, text.String())
			}
			pretty.WriteByte('\n')
			path := filepath.Join("testdata", "golden", call.tool+".json")
			if *updateGolden {
				if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden (run with -update): %v", err)
			}
			if !bytes.Equal(want, pretty.Bytes()) {
				t.Errorf("output changed; run with -update if intended\n--- want\n%s\n--- got\n%s", want, pretty.Bytes())
			}
		})
	}
}
