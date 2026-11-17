package contract

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestContractAlertingConfigurationMCP(t *testing.T) {
	setAgentWrites(t, true)
	t.Cleanup(func() { setAgentWrites(t, false) })
	cs, err := connectRemote(t, "/mcp/write", oauthScopedAccessToken(t, "traceway:write"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	callTool := toolCaller(t, cs)
	org := organizationOfProject(t)
	call := func(name string, id int, data map[string]any, want string) map[string]any {
		t.Helper()
		args := map[string]any{"project_id": projectID, "organization_id": org}
		if id > 0 {
			args["id"] = id
		}
		if data != nil {
			args["data"] = data
		}
		return callTool(name, args, want)
	}
	number := func(m map[string]any) int {
		t.Helper()
		v, ok := m["id"].(float64)
		if !ok {
			t.Fatalf("missing id: %v", m)
		}
		return int(v)
	}
	call("list_organization_members", 0, nil, "contract@example.com")
	teamData := map[string]any{"name": "MCP config team", "memberUserIds": []int{jwtUserID(t)}, "projectIds": []string{projectID}}
	team := number(call("create_team", 0, teamData, "id"))
	teamData["description"] = "Operational coverage"
	call("update_team", team, teamData, "Operational coverage")
	call("set_team_members", team, map[string]any{"userIds": []int{jwtUserID(t)}}, "")
	call("set_team_projects", team, map[string]any{"projectIds": []string{projectID}}, "")
	call("list_teams", 0, nil, "MCP config team")
	scheduleData := map[string]any{"name": "MCP config rotation", "teamId": team, "timezone": "UTC", "definition": map[string]any{"schemaVersion": 1, "layers": []any{map[string]any{"name": "Daily", "rotationType": "daily", "handoffTime": "09:00", "rotationStart": "2026-01-01", "userIds": []int{jwtUserID(t)}}}}}
	schedule := number(call("create_oncall_schedule", 0, scheduleData, "id"))
	scheduleData["description"] = "Daily coverage"
	call("update_oncall_schedule", schedule, scheduleData, "Daily coverage")
	call("list_oncall_schedules", 0, nil, "MCP config rotation")
	call("get_oncall_schedule", schedule, nil, "Daily coverage")
	override := number(call("create_oncall_override", schedule, map[string]any{"userId": jwtUserID(t), "startAt": "2026-11-13T00:00:00Z", "endAt": "2026-11-14T00:00:00Z"}, "id"))
	callTool("get_oncall_timeline", map[string]any{"organization_id": org, "id": schedule, "from": "2026-11-12T00:00:00Z", "to": "2026-11-15T00:00:00Z"}, "")
	callTool("delete_oncall_override", map[string]any{"project_id": projectID, "organization_id": org, "id": schedule, "child_id": override}, "")
	policyData := map[string]any{"name": "MCP config policy", "definition": map[string]any{"schemaVersion": 1, "repeatCount": 0, "steps": []any{map[string]any{"delayMinutes": 5, "targets": []any{map[string]any{"type": "schedule", "id": schedule}}}}}}
	policy := number(call("create_escalation_policy", 0, policyData, "id"))
	policyData["name"] = "MCP config policy updated"
	call("update_escalation_policy", policy, policyData, "updated")
	call("list_escalation_policies", 0, nil, "MCP config policy updated")
	channelData := map[string]any{"name": "MCP config channel", "channelType": "escalation", "config": map[string]any{"policyId": policy}, "enabled": false}
	channel := number(call("create_notification_channel", 0, channelData, "id"))
	channelData["name"] = "MCP config channel updated"
	call("update_notification_channel", channel, channelData, "updated")
	call("list_notification_channels", 0, nil, "MCP config channel updated")
	ruleData := map[string]any{"name": "MCP config rule", "channelId": channel, "ruleType": "new_error", "config": map[string]any{}, "cooldownMinutes": 5, "severity": "critical"}
	rule := number(call("create_notification_rule", 0, ruleData, "id"))
	ruleData["name"] = "MCP config rule updated"
	call("update_notification_rule", rule, ruleData, "updated")
	callTool("list_notification_rules", map[string]any{"project_id": projectID}, "MCP config rule updated")
	contact := number(call("create_contact_method", 0, map[string]any{"methodType": "email", "config": map[string]any{"email": "mcp-config@example.com"}}, "id"))
	call("update_contact_method", contact, map[string]any{"enabled": true}, "enabled")
	call("list_contact_methods", 0, nil, "mcp-config@example.com")
	call("set_user_notification_rules", 0, map[string]any{"high": []any{map[string]any{"contactMethodId": contact, "delayMinutes": 0}}, "low": []any{}}, "contactMethodId")
	call("get_user_notification_rules", 0, nil, "contactMethodId")
	call("get_oncall_now", 0, nil, "")
	call("get_current_oncall", 0, nil, "")
	call("list_notification_history", 0, map[string]any{"pagination": map[string]any{"page": 1, "pageSize": 20}}, "pagination")
	call("set_user_notification_rules", 0, map[string]any{"high": []any{}, "low": []any{}}, "")
	call("delete_contact_method", contact, nil, "")
	call("delete_notification_rule", rule, nil, "")
	call("delete_notification_channel", channel, nil, "")
	call("delete_escalation_policy", policy, nil, "")
	call("delete_oncall_schedule", schedule, nil, "")
	call("delete_team", team, nil, "")
}

func TestContractAlertingMCPDoesNotExposeWritesOnReadMount(t *testing.T) {
	cs, err := connectRemote(t, "/mcp", oauthScopedAccessToken(t, "traceway:write"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	result, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "create_team", Arguments: map[string]any{"project_id": projectID, "organization_id": organizationOfProject(t), "data": map[string]any{"name": "Must not create"}}})
	if err == nil && !result.IsError {
		t.Fatal("read MCP mount accepted a write")
	}
}
