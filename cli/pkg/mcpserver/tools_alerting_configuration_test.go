package mcpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAlertingMCPValidationAndWire(t *testing.T) {
	calls := 0
	var path string
	var body map[string]any
	cs := connectWrite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		path = r.Method + " " + r.URL.Path
		if r.URL.Query().Get("projectId") != "p-1" {
			t.Error("missing audit project")
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"id":9}`))
	}), "p-1")
	for name, args := range map[string]map[string]any{
		"create_team":                 {"organization_id": 7, "data": map[string]any{"name": "Ops"}},
		"update_contact_method":       {"id": 9, "data": map[string]any{"enabled": false}},
		"set_user_notification_rules": {"data": map[string]any{"high": []any{}, "low": []any{}}},
	} {
		res := callTool(t, cs, name, args)
		if res.IsError {
			t.Fatalf("%s: %s", name, resultText(t, res))
		}
	}
	if calls != 3 {
		t.Fatalf("calls=%d", calls)
	}
	res := callTool(t, cs, "create_team", map[string]any{"organization_id": 7, "data": map[string]any{"name": "Ops"}})
	if res.IsError || path != "POST /api/organizations/7/teams" || body["name"] != "Ops" {
		t.Fatalf("wire %s %v: %s", path, body, resultText(t, res))
	}
	for name, args := range map[string]map[string]any{
		"create_team":                 {"data": map[string]any{"name": "Ops"}},
		"create_notification_channel": {"data": map[string]any{"name": 123}},
		"set_user_notification_rules": {"data": map[string]any{"high": []any{}}},
		"delete_oncall_override":      {"organization_id": 7, "id": 1},
		"verify_contact_method":       {"id": 1, "data": map[string]any{"code": "not-a-code"}},
	} {
		res := callTool(t, cs, name, args)
		if !res.IsError || calls != 4 {
			t.Fatalf("invalid %s: %s calls=%d", name, resultText(t, res), calls)
		}
	}
}

func TestAlertingMCPReadsAndPolicyErrors(t *testing.T) {
	cs := connect(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("read tool mutated")
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}), "")
	for name, args := range map[string]map[string]any{"list_teams": {"organization_id": 7}, "list_contact_methods": nil, "get_user_notification_rules": nil} {
		res := callTool(t, cs, name, args)
		if res.IsError {
			t.Fatalf("%s: %s", name, resultText(t, res))
		}
	}
	calls := 0
	denied := connectWrite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Admin or owner access required"}`))
	}), "p-1")
	res := callTool(t, denied, "create_team", map[string]any{"organization_id": 7, "data": map[string]any{"name": "Ops"}})
	if !res.IsError || calls != 1 || !strings.Contains(resultText(t, res), "Admin or owner") {
		t.Fatalf("denial %s calls=%d", resultText(t, res), calls)
	}
}
