package mcpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestCreateReadTokenToolValidatesAndForwards(t *testing.T) {
	calls := 0
	var body map[string]any
	cs := connectWrite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/personal-access-tokens" || r.URL.Query().Get("projectId") != "p-1" {
			t.Errorf("request %s %s", r.Method, r.URL)
		}
		body = nil
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"t-1","name":"Embed","prefix":"twp_abcdefgh","token":"twp_secret","scope":"traceway:read","expiresAt":null}`))
	}), "p-1")
	res := callTool(t, cs, "create_read_token", map[string]any{"name": " Embed "})
	out, _ := res.StructuredContent.(map[string]any)
	if res.IsError || calls != 1 || body["name"] != "Embed" || body["scope"] != "read" || body["expiresInDays"] != nil || out["token"] != "twp_secret" {
		t.Fatalf("create: %s, body %v", resultText(t, res), body)
	}
	if res := callTool(t, cs, "create_read_token", map[string]any{"name": "CI", "expires_in_days": 30}); res.IsError || body["expiresInDays"] != float64(30) {
		t.Fatalf("expiry: %s, body %v", resultText(t, res), body)
	}
	for _, args := range []map[string]any{{"name": ""}, {"name": strings.Repeat("x", 101)}, {"name": "x", "expires_in_days": 3651}, {"name": "x", "expires_in_days": -1}} {
		if res := callTool(t, cs, "create_read_token", args); !res.IsError || calls != 2 {
			t.Fatalf("invalid token %v made a request: %s", args, resultText(t, res))
		}
	}
	read := connect(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("read server created a token") }), "p-1")
	tools, err := read.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "create_read_token" || tool.Name == "list_personal_access_tokens" {
			t.Fatalf("the read server must not offer %s", tool.Name)
		}
	}
}
