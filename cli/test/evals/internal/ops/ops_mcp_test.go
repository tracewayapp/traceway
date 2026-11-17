package ops

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	client "github.com/tracewayapp/traceway/cli/v2/pkg/client"
	"github.com/tracewayapp/traceway/cli/v2/pkg/mcpserver"
)

// TestEveryMCPToolMaps lists the write server's tools in memory: each must
// map to an op.
func TestEveryMCPToolMaps(t *testing.T) {
	srv := mcpserver.New(mcpserver.Config{Client: client.New("http://127.0.0.1:1"), AllowWrite: true})
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	n := 0
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		n++
		op := ParseMCP(tool.Name, map[string]any{"enabled": true})
		if strings.HasPrefix(op.Op, "mcp.") {
			t.Errorf("MCP tool %s maps to no op", tool.Name)
		}
		readOnly := tool.Annotations != nil && tool.Annotations.ReadOnlyHint
		if op.Write == readOnly {
			t.Errorf("MCP tool %s is readOnly=%v but op %s has write=%v", tool.Name, readOnly, op.Op, op.Write)
		}
	}
	if n < 30 {
		t.Fatalf("expected the full write tool surface, got %d tools", n)
	}
}

func TestRenderMCP(t *testing.T) {
	name, args, err := RenderMCP("metrics.query", map[string]any{"name": "system.cpu.utilization", "aggregation": "max", "group_by": "state", "since": "24h"})
	if err != nil || name != "query_metrics" {
		t.Fatal(name, err)
	}
	qs := args["queries"].([]any)
	if q := qs[0].(map[string]any); q["name"] != "system.cpu.utilization" || q["group_by"] != "state" || args["since"] != "24h" {
		t.Fatalf("RenderMCP args %+v", args)
	}
	// The fake model's templates hand numbers over as json.Number.
	if _, args, err := RenderMCP("alerts.disable", map[string]any{"id": json.Number("12")}); err != nil || args["id"] != 12 || args["enabled"] != false {
		t.Fatalf("RenderMCP alerts.disable = %+v, %v", args, err)
	}
}
