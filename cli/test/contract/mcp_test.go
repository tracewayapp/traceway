package contract

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	client "github.com/tracewayapp/traceway/cli/pkg/client"
	"github.com/tracewayapp/traceway/cli/pkg/mcpserver"
)

// TestContract_mcpToolSurface drives every MCP tool once against the real
// booted backend through an in-memory MCP session. The per-endpoint goldens
// above pin the wire shapes; this asserts the MCP layer's parameter plumbing
// (project fallback, time ranges, mandatory timestamps) works end to end.
func TestContract_mcpToolSurface(t *testing.T) {
	cs := connectInMemory(t, false)

	from := seedAt.Add(-time.Hour).Format(time.RFC3339)
	to := seedAt.Add(time.Hour).Format(time.RFC3339)
	at := seedAt.Format(time.RFC3339)

	call := toolCaller(t, cs)
	call("list_projects", nil, projectID)
	call("list_exceptions", map[string]any{"from": from, "to": to}, seedHash)
	call("get_exception", map[string]any{"hash": seedHash}, seedExceptionID.String())
	call("get_exception_occurrence", map[string]any{"id": seedExceptionID.String(), "recorded_at": at}, seedHash)
	call("query_logs", map[string]any{"from": from, "to": to, "service_name": "contract-svc"}, "contract-svc")
	call("list_endpoints", map[string]any{"from": from, "to": to}, "GET /api/contract")
	call("get_endpoint_request", map[string]any{"id": seedEndpointID.String(), "recorded_at": at}, seedEndpointID.String())
	call("endpoints_chart", map[string]any{"from": from, "to": to}, "endpoints")
	call("get_slow_endpoint_config", map[string]any{"endpoint": "GET /api/contract"}, "offsetMs")
	call("query_metrics", map[string]any{
		"from": from, "to": to, "interval_minutes": 60,
		"queries": []map[string]any{{"name": seedMetricName}},
	}, seedMetricName)
	call("get_task", map[string]any{"id": seedTaskID.String(), "recorded_at": at}, seedTaskID.String())
	call("get_ai_trace", map[string]any{"id": seedAiTraceID.String(), "recorded_at": at}, seedAiTraceID.String())
	call("get_session", map[string]any{"id": seedSessionID.String(), "started_at": at}, seedSessionID.String())
	call("get_trace", map[string]any{"id": seedTraceID, "recorded_at": at}, seedTraceID)

	call("list_pages", map[string]any{"status": "resolved"}, "pagination")
	call("list_notification_rules", nil, "rules")
	call("list_checks", nil, "checks")
	call("list_post_mortems", nil, "pagination")
}

// TestContract_mcpWriteTools calls every write tool through /mcp/write with a
// write token after the organization opted in, so a write route missing from
// the backend's agent allowlist fails here, not in production.
func TestContract_mcpWriteTools(t *testing.T) {
	setAgentWrites(t, true)
	t.Cleanup(func() { setAgentWrites(t, false) })
	cs, err := connectRemote(t, "/mcp/write", oauthScopedAccessToken(t, "traceway:write"))
	if err != nil {
		t.Fatalf("connect /mcp/write: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	callTool := toolCaller(t, cs)
	call := func(name string, args map[string]any, wantSubstr string) map[string]any {
		t.Helper()
		if args == nil {
			args = map[string]any{}
		}
		args["project_id"] = projectID
		return callTool(name, args, wantSubstr)
	}
	ruleID, checkID, pageID := seedOperations(t)

	call("archive_exceptions", map[string]any{"hashes": []string{seedHash}}, "archived")
	call("unarchive_exceptions", map[string]any{"hashes": []string{seedHash}}, "unarchived")
	call("set_slow_endpoint", map[string]any{"endpoint": "GET /api/contract", "offset_ms": 250, "reason": "contract"}, "updated")
	call("get_slow_endpoint_config", map[string]any{"endpoint": "GET /api/contract"}, "250")
	call("set_slow_endpoint", map[string]any{"endpoint": "GET /api/contract", "offset_ms": 0}, "updated")

	created := call("create_post_mortem", map[string]any{"title": "Contract outage", "content_md": "first draft", "tags": []string{"contract"}}, "Contract outage")
	id, ok := created["id"].(float64)
	if !ok {
		t.Fatalf("create_post_mortem returned no id: %v", created)
	}
	call("update_post_mortem", map[string]any{"id": int(id), "content_md": "revised", "expected_updated_at": created["updatedAt"]}, "revised")
	got := call("get_post_mortem", map[string]any{"id": int(id)}, "revised")
	if got["title"] != "Contract outage" {
		t.Errorf("update_post_mortem must keep fields it was not given, title = %v", got["title"])
	}
	stale, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_post_mortem", Arguments: map[string]any{
		"project_id": projectID, "id": int(id), "content_md": "based on the first draft", "expected_updated_at": created["updatedAt"],
	}})
	if err != nil || !stale.IsError || !strings.Contains(stale.Content[0].(*mcp.TextContent).Text, "changed since it was read") {
		t.Errorf("an update based on a stale read must be refused with a conflict, got err=%v result=%+v", err, stale)
	}
	call("list_post_mortems", map[string]any{"search": "Contract"}, "Contract outage")

	call("set_notification_rule_enabled", map[string]any{"id": ruleID, "enabled": false}, "")
	call("set_notification_rule_enabled", map[string]any{"id": ruleID, "enabled": true}, "")
	call("snooze_notification_rule", map[string]any{"id": ruleID, "duration_minutes": 30}, "snoozedUntil")
	call("snooze_notification_rule", map[string]any{"id": ruleID, "duration_minutes": 0}, "")
	call("list_notification_rules", nil, "Contract rule")

	call("run_check", map[string]any{"id": checkID}, "")
	call("get_check", map[string]any{"id": checkID}, "Contract check")

	call("acknowledge_page", map[string]any{"id": pageID}, "")
	call("get_page", map[string]any{"id": pageID}, "acknowledged")
	call("resolve_page", map[string]any{"id": pageID}, "")
	call("get_page", map[string]any{"id": pageID}, "resolved")
}

// seedOperations creates, with the dashboard session, the alert rule, monitor
// and open page the write tools act on. The page comes from an escalation
// channel's test button, the one way to open a real page without firing a rule.
func seedOperations(t *testing.T) (ruleID, checkID, pageID int) {
	t.Helper()
	q := "?projectId=" + projectID
	post := func(path string, body any) map[string]any {
		status, _, res := doJSON(t, http.MethodPost, path, jwtToken, body)
		if status != http.StatusOK && status != http.StatusCreated {
			t.Fatalf("POST %s: %d %v", path, status, res)
		}
		return res
	}
	number := func(res map[string]any, key string) int {
		v, ok := res[key].(float64)
		if !ok {
			t.Fatalf("response has no numeric %q: %v", key, res)
		}
		return int(v)
	}

	email := post("/api/notification-channels"+q, map[string]any{"name": "Contract email", "channelType": "email", "config": map[string]any{"recipients": []string{"ops@example.com"}}})
	ruleID = number(post("/api/notification-rules"+q, map[string]any{
		"channelId": number(email, "id"), "name": "Contract rule", "ruleType": "error_rate_threshold",
		"config": map[string]any{"thresholdPercent": 50, "lookbackMinutes": 5, "minRequests": 10}, "cooldownMinutes": 15,
	}), "id")

	checkID = number(post("/api/synthetics/checks"+q, map[string]any{
		"name": "Contract check", "checkType": "http", "intervalSeconds": 3600, "timeoutSeconds": 10, "failureThreshold": 2,
		"config": map[string]any{"url": baseURL + "/api/has-organizations", "method": "GET"},
	}), "id")

	policy := post("/api/organizations/"+strconv.Itoa(organizationOfProject(t))+"/escalation-policies", map[string]any{
		"name": "Contract policy",
		"definition": map[string]any{"schemaVersion": 1, "repeatCount": 0, "steps": []map[string]any{
			{"delayMinutes": 5, "targets": []map[string]any{{"type": "user", "id": jwtUserID(t)}}},
		}},
	})
	escalation := post("/api/notification-channels"+q, map[string]any{"name": "Contract paging", "channelType": "escalation", "config": map[string]any{"policyId": number(policy, "id")}})
	post("/api/notification-channels/"+strconv.Itoa(number(escalation, "id"))+"/test"+q, nil)

	pages := post("/api/pages"+q, map[string]any{"status": "open", "pagination": map[string]any{"page": 1, "pageSize": 10}})
	data, _ := pages["data"].([]any)
	if len(data) == 0 {
		t.Fatalf("the escalation channel test opened no page: %v", pages)
	}
	first, _ := data[0].(map[string]any)
	return ruleID, checkID, number(first, "id")
}

func jwtUserID(t *testing.T) int {
	t.Helper()
	parts := strings.Split(jwtToken, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		UserID int `json:"userId"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.UserID == 0 {
		t.Fatalf("session JWT carries no userId: %v %s", err, payload)
	}
	return claims.UserID
}

func connectInMemory(t *testing.T, allowWrite bool) *mcp.ClientSession {
	t.Helper()
	srv := mcpserver.New(mcpserver.Config{
		Client:           client.New(baseURL, client.WithJWT(jwtToken)),
		DefaultProjectID: projectID,
		InstanceURL:      baseURL,
		Version:          "contract",
		AllowWrite:       allowWrite,
	})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(t.Context(), serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "contract", Version: "0"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func toolCaller(t *testing.T, cs *mcp.ClientSession) func(name string, args map[string]any, wantSubstr string) map[string]any {
	return func(name string, args map[string]any, wantSubstr string) map[string]any {
		t.Helper()
		res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: protocol error: %v", name, err)
		}
		var text strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text.WriteString(tc.Text)
			}
		}
		if res.IsError {
			t.Fatalf("%s: tool error: %s", name, text.String())
		}
		if wantSubstr != "" && !strings.Contains(text.String(), wantSubstr) {
			t.Errorf("%s: result missing %q: %s", name, wantSubstr, clip([]byte(text.String())))
		}
		structured, _ := res.StructuredContent.(map[string]any)
		return structured
	}
}
