package mcpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDashboardToolsValidateAndForward(t *testing.T) {
	calls := 0
	var body map[string]any
	cs := connectWrite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/dashboards/import" || r.URL.Query().Get("projectId") != "p-1" {
			t.Errorf("request %s %s", r.Method, r.URL)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"created":1,"updated":0,"dashboards":[{"id":42}]}`))
	}), "p-1")
	doc := map[string]any{"schemaVersion": 1, "name": "Worker", "widgets": []any{}}
	res := callTool(t, cs, "import_dashboards", map[string]any{"data": doc})
	if res.IsError || res.StructuredContent == nil || calls != 1 || body["mode"] != "create" {
		t.Fatalf("import: %s, body %v", resultText(t, res), body)
	}
	for _, args := range []map[string]any{{"data": doc, "mode": "replace"}, {"data": map[string]any{"name": "invalid"}}} {
		res = callTool(t, cs, "import_dashboards", args)
		if !res.IsError || calls != 1 {
			t.Fatalf("invalid import made request: %s", resultText(t, res))
		}
	}
}

func TestDashboardToolsReadAndErrors(t *testing.T) {
	calls := 0
	cs := connect(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("read tool mutated")
		}
		if r.URL.Path == "/api/metrics/discover/tags" && (r.URL.Query().Get("name") != "queue.depth" || r.URL.Query().Get("key") != "queue" || r.URL.Query().Get("from") == "") {
			t.Errorf("query %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}), "p-1")
	for name, args := range map[string]map[string]any{"list_dashboards": nil, "dashboard_library": nil, "export_dashboard": {"id": 42}, "discover_metrics": {"name": "queue.depth", "key": "queue", "since": "1h"}} {
		if res := callTool(t, cs, name, args); res.IsError {
			t.Fatalf("%s: %s", name, resultText(t, res))
		}
	}
	for name, args := range map[string]map[string]any{"export_dashboard": {"id": 0}, "discover_metrics": {"name": "queue.depth"}} {
		if res := callTool(t, cs, name, args); !res.IsError || calls != 4 {
			t.Fatalf("invalid %s: %s", name, resultText(t, res))
		}
	}
	denied := connectWrite(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Agent write access is turned off"}`))
	}), "p-1")
	res := callTool(t, denied, "import_dashboards", map[string]any{"data": map[string]any{"name": "Worker", "widgets": []any{}}})
	if !res.IsError || !strings.Contains(resultText(t, res), "turned off") {
		t.Fatalf("denial: %s", resultText(t, res))
	}
}

func TestDashboardEmbedToolValidatesAndForwards(t *testing.T) {
	calls := 0
	var body map[string]any
	var query string
	cs := connectWrite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/dashboards/42/embed-token" || r.URL.Query().Get("projectId") != "p-1" {
			t.Errorf("request %s %s", r.Method, r.URL)
		}
		query = r.URL.RawQuery
		body = nil
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"dashboardUuid":"d-1","token":"jwt","expiresInSeconds":300,"embedPath":"/embed/dashboard?dashboard=d-1#token=jwt"}`))
	}), "p-1")
	res := callTool(t, cs, "create_dashboard_embed", map[string]any{"id": 42, "timeframe": "7d", "expires_in_days": 3, "theme": "light", "tag_filters": map[string]any{"account_id": "acme"}})
	out, _ := res.StructuredContent.(map[string]any)
	filters, _ := body["tagFilters"].(map[string]any)
	if res.IsError || calls != 1 || !strings.Contains(query, "timeframe=7d") || !strings.Contains(query, "theme=light") || body["expiresInDays"] != float64(3) || filters["account_id"] != "acme" {
		t.Fatalf("embed: %s, query %s, body %v", resultText(t, res), query, body)
	}
	if url, _ := out["url"].(string); !strings.HasSuffix(url, "/embed/dashboard?dashboard=d-1#token=jwt") || !strings.HasPrefix(url, "http") {
		t.Fatalf("url: %v", out["url"])
	}
	for _, args := range []map[string]any{
		{"id": 0}, {"id": 42, "timeframe": "week"}, {"id": 42, "from": "2026-11-01T00:00:00Z"}, {"id": 42, "expires_at": "tomorrow"},
		{"id": 42, "expires_in_days": 91}, {"id": 42, "theme": "sepia"}, {"id": 42, "tag_filters": map[string]any{"account_id": ""}},
	} {
		if res := callTool(t, cs, "create_dashboard_embed", args); !res.IsError || calls != 1 {
			t.Fatalf("invalid embed %v made a request: %s", args, resultText(t, res))
		}
	}
	read := connect(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("read server issued a link") }), "p-1")
	tools, err := read.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "create_dashboard_embed" {
			t.Fatal("the read server must not offer create_dashboard_embed")
		}
	}
}
