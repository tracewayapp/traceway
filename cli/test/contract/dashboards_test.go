package contract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func TestContractDashboardMCPAndClient(t *testing.T) {
	setAgentWrites(t, true)
	t.Cleanup(func() { setAgentWrites(t, false) })
	token := oauthScopedAccessToken(t, "traceway:write")
	cs, err := connectRemote(t, "/mcp/write", token)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	call := toolCaller(t, cs)
	doc := map[string]any{"schemaVersion": 1, "name": "Dashboard MCP contract", "widgets": []any{map[string]any{
		"id": "signal", "title": "Contract signal", "widgetType": "line_chart", "config": map[string]any{"sources": []any{map[string]any{"type": "metric", "name": seedMetricName, "aggregation": "max"}}},
	}}}
	created := call("import_dashboards", map[string]any{"project_id": projectID, "data": doc}, "created")
	items, ok := created["dashboards"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("response %v", created)
	}
	id := int(items[0].(map[string]any)["id"].(float64))
	call("export_dashboard", map[string]any{"project_id": projectID, "id": id}, seedMetricName)
	call("list_dashboards", map[string]any{"project_id": projectID}, "Dashboard MCP contract")
	call("dashboard_library", nil, "appliedProjectIds")
	call("discover_metrics", map[string]any{"project_id": projectID, "from": seedAt.Add(-time.Hour).Format(time.RFC3339), "to": seedAt.Add(time.Hour).Format(time.RFC3339)}, seedMetricName)
	c := client.New(baseURL, client.WithJWT(token))
	doc["description"] = "Updated through the CLI client"
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.ValidateDashboardImport(data, "upsert"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ImportDashboards(t.Context(), projectID, data, "upsert"); err != nil {
		t.Fatal(err)
	}
	call("export_dashboard", map[string]any{"project_id": projectID, "id": id}, "Updated through the CLI client")
}
