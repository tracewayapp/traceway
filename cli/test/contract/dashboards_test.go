package contract

import (
	"encoding/json"
	"net/http"
	"strings"
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
	link := call("create_dashboard_embed", map[string]any{"project_id": projectID, "id": id, "timeframe": "7d", "expires_in_days": 3, "tag_filters": map[string]any{"account_id": "acme"}}, "/embed/dashboard?dashboard=")
	if link["token"] == "" || link["url"] == nil || link["expiresInSeconds"].(float64) < 3*86400-60 {
		t.Fatalf("embed link %v", link)
	}
	c := client.New(baseURL, client.WithJWT(token))
	raw, err := c.CreateDashboardEmbed(t.Context(), projectID, id, client.DashboardEmbedOptions{Theme: "light"}, baseURL)
	var issued struct {
		URL, Token, DashboardUUID string
		ExpiresInSeconds          int
	}
	if err != nil || json.Unmarshal(raw, &issued) != nil || issued.ExpiresInSeconds > 300 || !strings.HasPrefix(issued.URL, baseURL+"/embed/dashboard?dashboard="+issued.DashboardUUID+"&theme=light#token=") {
		t.Fatalf("client embed link: %s (%v)", raw, err)
	}
	// The link is all a viewer has: no session, no project parameter.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/api/embedded-dashboards/"+issued.DashboardUUID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+issued.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var shared struct {
		Name    string
		Widgets []struct{ Id, Title string }
	}
	if err := json.NewDecoder(resp.Body).Decode(&shared); err != nil || resp.StatusCode != http.StatusOK || shared.Name != "Dashboard MCP contract" || len(shared.Widgets) != 1 || shared.Widgets[0].Title != "Contract signal" {
		t.Fatalf("shared dashboard: %d %+v (%v)", resp.StatusCode, shared, err)
	}
	if _, err := c.CreateDashboardEmbed(t.Context(), projectID, id, client.DashboardEmbedOptions{ExpiresInDays: client.DashboardEmbedMaxDays}, ""); err != nil {
		t.Fatalf("longest link: %v", err)
	}
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
