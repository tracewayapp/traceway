package contract

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func TestContractReadTokenMCPAndClient(t *testing.T) {
	setAgentWrites(t, true)
	t.Cleanup(func() { setAgentWrites(t, false) })
	writer := oauthScopedAccessToken(t, "traceway:write")
	cs, err := connectRemote(t, "/mcp/write", writer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	call := toolCaller(t, cs)

	created := call("create_read_token", map[string]any{"project_id": projectID, "name": "Contract embed"}, "twp_")
	minted, _ := created["token"].(string)
	if !strings.HasPrefix(minted, "twp_") || created["scope"] != "traceway:read" || created["expiresAt"] != nil {
		t.Fatalf("create_read_token: %v", created)
	}

	c := client.New(baseURL, client.WithJWT(writer))
	raw, err := c.CreateReadToken(t.Context(), projectID, "Contract CI", 30)
	var expiring struct {
		Id, Token, Scope string
		ExpiresAt        *string
	}
	if err != nil || json.Unmarshal(raw, &expiring) != nil || !strings.HasPrefix(expiring.Token, "twp_") || expiring.Scope != "traceway:read" || expiring.ExpiresAt == nil {
		t.Fatalf("client read token: %s (%v)", raw, err)
	}
	listed, err := c.ListPersonalAccessTokens(t.Context())
	if err != nil || !strings.Contains(string(listed), expiring.Id) || !strings.Contains(string(listed), created["id"].(string)) || strings.Contains(string(listed), minted) || strings.Contains(string(listed), expiring.Token) {
		t.Fatalf("client token list: %s (%v)", listed, err)
	}
	call("list_personal_access_tokens", nil, "Contract embed")

	path := "/api/personal-access-tokens?projectId=" + projectID
	if status, _, body := doJSON(t, http.MethodPost, path, writer, map[string]any{"name": "w", "scope": "write", "expiresInDays": 30}); status != http.StatusForbidden {
		t.Fatalf("an agent created a write token: %d %v", status, body)
	}

	doc := map[string]any{"schemaVersion": 1, "name": "Read token contract", "widgets": []any{map[string]any{
		"id": "signal", "title": "Contract signal", "widgetType": "line_chart", "config": map[string]any{"sources": []any{map[string]any{"type": "metric", "name": seedMetricName, "aggregation": "max"}}},
	}}}
	imported := call("import_dashboards", map[string]any{"project_id": projectID, "data": doc}, "created")
	items, ok := imported["dashboards"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("import: %v", imported)
	}
	dashboard := int(items[0].(map[string]any)["id"].(float64))

	// The token exists so a host application can issue links, which is a read:
	// it must keep working once the organization turns agent writes off.
	setAgentWrites(t, false)
	reader := client.New(baseURL, client.WithJWT(minted))
	if link, err := reader.CreateDashboardEmbed(t.Context(), projectID, dashboard, client.DashboardEmbedOptions{}, baseURL); err != nil || !strings.Contains(string(link), "/embed/dashboard?dashboard=") {
		t.Fatalf("link issued with the created read token: %s (%v)", link, err)
	}
	if _, err := c.CreateReadToken(t.Context(), projectID, "Contract refused", 0); err == nil {
		t.Fatal("a token was created with agent writes off")
	}
	setAgentWrites(t, true)
	if _, err := reader.CreateReadToken(t.Context(), projectID, "Contract refused", 0); err == nil {
		t.Fatal("a read token created another token")
	}
}
