package mcpserver

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestOpampToolsWireContract(t *testing.T) {
	for _, tc := range []struct {
		tool, method, path string
		args               map[string]any
	}{
		{"list_sdk_config_rules", "GET", "/api/sdk-config/rules", nil},
		{"preview_sdk_config", "POST", "/api/sdk-config/preview", map[string]any{"attributes": map[string]string{"service.version": "2.0.0"}, "instance_uid": "0195f2a0-0000-7000-8000-0000000000aa"}},
		{"create_sdk_config_rule", "POST", "/api/sdk-config/rules", map[string]any{"name": "Full sessions", "enabled": false, "conditions": map[string]any{}, "settings": map[string]any{"replay": map[string]any{"mode": "stream", "maskAllInputs": true}}}},
		{"update_sdk_config_rule", "PUT", "/api/sdk-config/rules/7", map[string]any{"id": 7, "name": "Full sessions", "conditions": map[string]any{}, "settings": map[string]any{}, "expected_updated_at": "2026-11-12T10:00:00Z"}},
		{"delete_sdk_config_rule", "DELETE", "/api/sdk-config/rules/7", map[string]any{"id": 7}},
		{"reorder_sdk_config_rules", "PUT", "/api/sdk-config/rules/reorder", map[string]any{"ids": []int{7, 2}}},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			calls := 0
			backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tc.method || r.URL.Path != tc.path || r.URL.Query().Get("projectId") != "p-1" {
					t.Errorf("request: %s %s", r.Method, r.URL)
				}
				var body map[string]any
				if r.Method != "GET" && r.Method != "DELETE" {
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
				}
				if tc.tool == "update_sdk_config_rule" && body["expectedUpdatedAt"] != tc.args["expected_updated_at"] {
					t.Errorf("missing update version: %v", body)
				}
				if tc.tool == "create_sdk_config_rule" && body["enabled"] != false {
					t.Errorf("lost explicit false: %v", body)
				}
				if tc.tool == "preview_sdk_config" && body["instanceUid"] != tc.args["instance_uid"] {
					t.Errorf("lost preview instance: %v", body)
				}
				_, _ = w.Write([]byte(`{"data":[]}`))
			})
			cs := connectWrite(t, backend, "p-1")
			res := callTool(t, cs, tc.tool, tc.args)
			if res.IsError || calls != 1 {
				t.Fatalf("tool: %s calls=%d", resultText(t, res), calls)
			}
		})
	}
}

func TestOpampUpdateRequiresVersion(t *testing.T) {
	called := false
	cs := connectWrite(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), "p-1")
	res := callTool(t, cs, "update_sdk_config_rule", map[string]any{"id": 7, "name": "x", "conditions": map[string]any{}, "settings": map[string]any{}, "expected_updated_at": "invalid"})
	if !res.IsError || called {
		t.Fatal("invalid update should fail before any request")
	}
}
