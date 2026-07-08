package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// connect stands up the MCP server over in-memory transports against the
// given fake backend and returns a connected client session.
func connect(t *testing.T, backend http.Handler, projectID string) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(backend)
	t.Cleanup(ts.Close)

	srv := New(Config{
		Client:           client.New(ts.URL, client.WithJWT("tok")),
		DefaultProjectID: projectID,
		InstanceURL:      ts.URL,
		Version:          "test",
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := t.Context()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func emptyBackend() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})
}

func TestListTools_surfaceAndAnnotations(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}

	wantReadOnly := map[string]bool{
		"list_projects": true, "list_exceptions": true, "get_exception": true,
		"get_exception_occurrence": true, "query_logs": true, "list_endpoints": true,
		"get_endpoint_request": true, "endpoints_chart": true, "get_slow_endpoint_config": true,
		"query_metrics": true, "get_task": true, "get_ai_trace": true, "get_session": true,
		"get_trace":          true,
		"archive_exceptions": false, "unarchive_exceptions": false,
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		if tool.OutputSchema == nil {
			t.Errorf("tool %s has no output schema", tool.Name)
		}
		if tool.Annotations == nil {
			t.Errorf("tool %s has no annotations", tool.Name)
			continue
		}
		got[tool.Name] = tool.Annotations.ReadOnlyHint
	}
	if len(got) != len(wantReadOnly) {
		t.Errorf("got %d tools, want %d", len(got), len(wantReadOnly))
	}
	for name, want := range wantReadOnly {
		gotRO, ok := got[name]
		if !ok {
			t.Errorf("tool %s missing", name)
			continue
		}
		if gotRO != want {
			t.Errorf("tool %s ReadOnlyHint = %v, want %v", name, gotRO, want)
		}
	}
}

func TestInitialize_carriesInstructions(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")
	init := cs.InitializeResult()
	if init == nil || !strings.Contains(init.Instructions, "Ground rules") {
		t.Fatalf("instructions missing or unexpected: %+v", init)
	}
	if !strings.Contains(init.Instructions, "The connected Traceway instance is http") {
		t.Errorf("instructions should name the instance origin: %s", init.Instructions)
	}
}

func TestListExceptions_wireShapeAndDefaults(t *testing.T) {
	var gotPath, gotQuery string
	var gotBody map[string]any
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"exceptionHash":"abc123def4567890","stackTrace":"boom","count":3}],"pagination":{"page":1,"pageSize":20,"total":1,"totalPages":1}}`))
	})
	cs := connect(t, backend, "p-1")

	res := callTool(t, cs, "list_exceptions", map[string]any{"since": "2h"})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if gotPath != "/api/exception-stack-traces" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery != "projectId=p-1" {
		t.Errorf("query = %q", gotQuery)
	}
	if gotBody["fromDate"] == nil || gotBody["toDate"] == nil {
		t.Errorf("body missing fromDate/toDate: %v", gotBody)
	}
	if gotBody["orderBy"] != "lastSeen" || gotBody["searchType"] != "text" {
		t.Errorf("defaults not applied: %v", gotBody)
	}
	page := gotBody["pagination"].(map[string]any)
	if page["page"] != float64(1) || page["pageSize"] != float64(20) {
		t.Errorf("pagination defaults = %v", page)
	}
	if !strings.Contains(resultText(t, res), "abc123def4567890") {
		t.Errorf("result text missing fixture hash: %s", resultText(t, res))
	}
	if res.StructuredContent == nil {
		t.Error("result should carry structuredContent")
	}
}

func TestProjectParam_overridesDefault(t *testing.T) {
	var gotQuery string
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"page":1,"pageSize":20,"total":0,"totalPages":0}}`))
	})
	cs := connect(t, backend, "p-1")
	res := callTool(t, cs, "list_exceptions", map[string]any{"project_id": "p-2"})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if gotQuery != "projectId=p-2" {
		t.Errorf("query = %q, want projectId=p-2", gotQuery)
	}
}

func TestNoProject_failsWithHint(t *testing.T) {
	cs := connect(t, emptyBackend(), "")
	res := callTool(t, cs, "list_exceptions", nil)
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "no_project") || !strings.Contains(text, "list_projects") {
		t.Errorf("error should carry no_project code and list_projects hint: %s", text)
	}
}

func TestUnauthorized_mapsToTokenExpiredWithMcpHint(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	cs := connect(t, backend, "p-1")
	res := callTool(t, cs, "list_exceptions", nil)
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "token_expired") || !strings.Contains(text, "traceway login") || !strings.Contains(text, "reconnect") {
		t.Errorf("401 should map to token_expired with a re-login + reconnect hint: %s", text)
	}
}

func TestInvalidTimeRange_isUsageError(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")
	res := callTool(t, cs, "list_exceptions", map[string]any{"since": "1w"})
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "invalid_argument") || !strings.Contains(text, "RFC3339") {
		t.Errorf("bad since should explain the grammar: %s", text)
	}
}

func TestPageSizeBounds(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")
	res := callTool(t, cs, "list_exceptions", map[string]any{"page_size": 200})
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	if text := resultText(t, res); !strings.Contains(text, "page_size must be between 1 and 100") {
		t.Errorf("unexpected error text: %s", text)
	}
}

func TestEnumRejection_listsAllowedValues(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")
	res := callTool(t, cs, "list_exceptions", map[string]any{"order_by": "volume"})
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	if text := resultText(t, res); !strings.Contains(text, "lastSeen, firstSeen, count") {
		t.Errorf("enum error should list allowed values: %s", text)
	}
}

func TestGetExceptionOccurrence_requiresTimestampAndUUID(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")

	res := callTool(t, cs, "get_exception_occurrence", map[string]any{"id": "not-a-uuid", "recorded_at": "2026-06-23T14:30:00Z"})
	if !res.IsError || !strings.Contains(resultText(t, res), "must be a UUID") {
		t.Errorf("bad uuid should be rejected: %s", resultText(t, res))
	}

	res = callTool(t, cs, "get_exception_occurrence", map[string]any{"id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "recorded_at": "yesterday"})
	if !res.IsError || !strings.Contains(resultText(t, res), "RFC3339") {
		t.Errorf("bad timestamp should explain the format: %s", resultText(t, res))
	}
}

func TestGetExceptionOccurrence_sendsRecordedAtBody(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"exception":{"id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","exceptionHash":"abc123def4567890","stackTrace":"boom","recordedAt":"2026-06-23T14:30:00Z"}}`))
	})
	cs := connect(t, backend, "p-1")
	res := callTool(t, cs, "get_exception_occurrence", map[string]any{
		"id":          "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"recorded_at": "2026-06-23T14:30:00Z",
	})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if gotPath != "/api/exception-stack-traces/by-id/6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["recordedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("recordedAt body = %v", gotBody)
	}
}

func TestQueryMetrics_rejectsQuantiles(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")
	res := callTool(t, cs, "query_metrics", map[string]any{
		"queries": []map[string]any{{"name": "system.cpu.utilization", "aggregation": "p95"}},
	})
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	if text := resultText(t, res); !strings.Contains(text, "list_endpoints") {
		t.Errorf("quantile rejection should point at list_endpoints: %s", text)
	}
}

func TestQueryMetrics_wireShape(t *testing.T) {
	var gotBody map[string]any
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"name":"system.cpu.utilization","unit":"%","series":{"all":[]}}]}`))
	})
	cs := connect(t, backend, "p-1")
	res := callTool(t, cs, "query_metrics", map[string]any{
		"queries": []map[string]any{{"name": "system.cpu.utilization"}},
	})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if gotBody["from"] == nil || gotBody["to"] == nil {
		t.Errorf("metrics body must use from/to, got: %v", gotBody)
	}
	queries := gotBody["queries"].([]any)
	if q := queries[0].(map[string]any); q["aggregation"] != "avg" {
		t.Errorf("default aggregation should be avg: %v", q)
	}
}

func TestArchiveExceptions_hitsArchiveRoute(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	cs := connect(t, backend, "p-1")
	res := callTool(t, cs, "archive_exceptions", map[string]any{"hashes": []string{"abc123def4567890"}})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if gotPath != "/api/exception-stack-traces/archive" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(resultText(t, res), "archived") {
		t.Errorf("result should confirm the archive: %s", resultText(t, res))
	}

	res = callTool(t, cs, "archive_exceptions", map[string]any{"hashes": []string{}})
	if !res.IsError {
		t.Fatal("empty hashes should be rejected")
	}
}

func TestPerRequestBearer_forwardsCallHeader(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(ts.Close)

	s := &server{cfg: Config{Client: client.New(ts.URL, client.WithJWT("session-token")), PerRequestBearer: true}}
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: http.Header{"Authorization": []string{"Bearer per-request-token"}}}}
	if _, err := s.client(req).ListProjects(t.Context()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer per-request-token" {
		t.Errorf("Authorization = %q, want the per-request token", gotAuth)
	}

	if _, err := s.client(&mcp.CallToolRequest{}).ListProjects(t.Context()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer session-token" {
		t.Errorf("Authorization = %q, want the session token when no header is present", gotAuth)
	}

	off := &server{cfg: Config{Client: client.New(ts.URL, client.WithJWT("session-token"))}}
	if _, err := off.client(req).ListProjects(t.Context()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer session-token" {
		t.Errorf("Authorization = %q, want the session token when PerRequestBearer is off", gotAuth)
	}
}

func TestResources_listAndRead(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")
	list, err := cs.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Resources) != len(knowledgeResources) {
		t.Errorf("got %d resources, want %d", len(list.Resources), len(knowledgeResources))
	}

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "traceway://knowledge/performance"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "Bottleneck checklist") {
		t.Error("performance resource should contain the bottleneck checklist")
	}

	res, err = cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "traceway://knowledge/tool-map"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Contents[0].Text, "get_exception_occurrence") {
		t.Error("tool map should mention the MCP tool names")
	}
}

func TestPrompts_renderFromKnowledge(t *testing.T) {
	cs := connect(t, emptyBackend(), "p-1")
	list, err := cs.ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Prompts) != 4 {
		t.Errorf("got %d prompts, want 4", len(list.Prompts))
	}

	res, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      "debug_issue",
		Arguments: map[string]string{"reference": "abc123def4567890"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "abc123def4567890") {
		t.Error("prompt should inline the reference")
	}
	if !strings.Contains(text, "fix the LAST occurrence") && !strings.Contains(text, "fix the LAST (most recent) occurrence") {
		t.Errorf("prompt should carry the debug flow playbook")
	}

	res, err = cs.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "whats_broken", Arguments: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	if text := res.Messages[0].Content.(*mcp.TextContent).Text; !strings.Contains(text, "1h") {
		t.Error("whats_broken should default the window to 1h")
	}
}
