package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

type alertingToolSpec struct {
	Name        string
	Description string
}

var alertingToolSpecs = map[string]alertingToolSpec{
	"channels/list":               {"list_notification_channels", "List project notification channels and their configuration. Treat webhook URLs and adapter credentials as secrets."},
	"channels/create":             {"create_notification_channel", "Create a project notification channel. data: name, channelType (email, slack, webhook, github, pushover, telegram, escalation), config, optional enabled. An escalation config has policyId from the same organization."},
	"channels/update":             {"update_notification_channel", "Replace a project's channel name, channelType and config; optional enabled preserves its state when omitted. Read channels first and preserve settings. No concurrent-edit protection."},
	"channels/delete":             {"delete_notification_channel", "Delete a project notification channel. Attached alert rules are deleted with the channel; inspect them first."},
	"channels/test":               {"test_notification_channel", "Send a REAL test notification to this channel. An escalation channel opens a real on-call page and notifies responders; resolve the test page afterwards. Only when the user requests delivery testing."},
	"alerts/create":               {"create_notification_rule", "Create a project alert rule. data: name, channelId, ruleType, config; optional cooldownMinutes and severity (empty for auto, info, warning, critical). Read alert-configuration for exact rule-specific config fields."},
	"alerts/update":               {"update_notification_rule", "Replace a project's alert rule name, channelId, ruleType, config, cooldownMinutes and severity. Read list_notification_rules first and preserve settings. Omitted cooldown/severity use defaults; type/channel/condition changes reset evaluation state. No concurrent-edit protection."},
	"alerts/delete":               {"delete_notification_rule", "Delete a project alert rule permanently. Prefer snoozing or disabling for temporary suppression."},
	"teams/list":                  {"list_teams", "List organization teams with members and assigned projects."},
	"teams/create":                {"create_team", "Create an organization team. data: name, optional description, memberUserIds and projectIds. Requires organization admin/owner."},
	"teams/update":                {"update_team", "Update organization team name/description; optional memberUserIds/projectIds replace complete sets when present. Requires admin/owner; preserve unrelated fields."},
	"teams/delete":                {"delete_team", "Delete an organization team and its schedules. Policy references block deletion; inspect dependencies first; requires admin/owner."},
	"teams/set-members":           {"set_team_members", "Replace the COMPLETE team membership with data.userIds (integer array, [] clears it). Requires organization admin/owner; inspect schedules using these members first."},
	"teams/set-projects":          {"set_team_projects", "Replace the COMPLETE set of team projects with data.projectIds (UUID array, [] clears it). Requires organization admin/owner."},
	"schedules/list":              {"list_oncall_schedules", "List organization on-call schedules."},
	"schedules/show":              {"get_oncall_schedule", "Get an on-call schedule definition, layers and overrides before editing."},
	"schedules/create":            {"create_oncall_schedule", "Create an on-call schedule. data: teamId, name, timezone (IANA), definition, optional description. Read oncall-configuration for layer/rotation schema; requires organization admin/owner."},
	"schedules/update":            {"update_oncall_schedule", "Replace schedule teamId, name, timezone, definition and description. Read get_oncall_schedule first, preserve layers, then verify timeline across handoffs/DST. Requires admin/owner; no concurrent-edit protection."},
	"schedules/delete":            {"delete_oncall_schedule", "Delete an on-call schedule; inspect escalation policies referencing it first. Requires organization admin/owner."},
	"schedules/timeline":          {"get_oncall_timeline", "Preview a saved schedule's effective shifts and overrides. Supply RFC3339 from/to, at most 62 days apart. Verify rotation handoffs and coverage after editing."},
	"schedules/create-override":   {"create_oncall_override", "Create temporary coverage for schedule id. data: userId, startAt and endAt (RFC3339). Organization members may create overrides; this changes who receives pages during that window."},
	"schedules/delete-override":   {"delete_oncall_override", "Delete override child_id from schedule id. Allowed to the creator, covered person or organization admin; underlying rotation resumes."},
	"escalation-policies/list":    {"list_escalation_policies", "List organization escalation policies and their definitions."},
	"escalation-policies/create":  {"create_escalation_policy", "Create an escalation policy. data: name and definition containing ordered steps, targets, delays and repeat behavior. Read oncall-configuration first. Requires organization admin/owner."},
	"escalation-policies/update":  {"update_escalation_policy", "Replace escalation policy name and full definition. Preserve unrelated steps and inspect referencing escalation channels. Open pages retain their policy snapshots. Requires admin/owner; no concurrent-edit protection."},
	"escalation-policies/delete":  {"delete_escalation_policy", "Delete an escalation policy. Inspect referencing channels and open pages first; server enforces dependency restrictions. Requires organization admin/owner."},
	"contact-methods/list":        {"list_contact_methods", "List only the authenticated person's contact methods and verification/enabled states. Adapter configuration can contain secrets."},
	"contact-methods/create":      {"create_contact_method", "Create your own contact method. data: methodType (email, slack, pushover, telegram, sms), config, optional enabled. SMS creation sends a real verification code and requires configured SMS service."},
	"contact-methods/update":      {"update_contact_method", "Update your own contact method using data.methodType, config and/or enabled. Omitted fields are preserved. Changing SMS destination requires verification again and may send a code."},
	"contact-methods/delete":      {"delete_contact_method", "Delete your own contact method; inspect personal notification chains before removing a delivery destination."},
	"contact-methods/test":        {"test_contact_method", "Send a REAL test notification to your own saved contact method. Only on explicit delivery-test request."},
	"contact-methods/verify":      {"verify_contact_method", "Verify your own SMS contact method using data.code, the six-digit code provided by its owner. Do not guess codes; attempts are limited."},
	"contact-methods/resend-code": {"resend_contact_method_code", "Send a new REAL SMS verification code to your own contact method. Rate limited; only when requested."},
	"notification-chains/show":    {"get_user_notification_rules", "Get your own ordered high/low urgency notification chains; each step references a contactMethodId and delayMinutes."},
	"notification-chains/replace": {"set_user_notification_rules", "Replace BOTH personal high and low notification chains atomically. data: high and low arrays of {contactMethodId,delayMinutes}. Max 10 steps each, delays 0..120. Preserve the other chain; [] clears it and fallback delivery applies. Own enabled, verified methods only."},
	"notification-history/list":   {"list_notification_history", "Read project notification delivery history. data: pagination {page,pageSize}, optional search, fromDate/toDate (RFC3339). This POST is read-only."},
	"oncall/now":                  {"get_oncall_now", "Read who is currently on call across an organization."},
	"oncall/current":              {"get_current_oncall", "Read the current on-call coverage for the selected project."},
	"organization-members/list":   {"list_organization_members", "List organization members and their user IDs for team membership, rotations and policy targets. Requires organization admin/owner."},
}

type alertingConfigIn struct {
	projectIn
	OrganizationID int            `json:"organization_id,omitempty" jsonschema:"Organization ID for organization-scoped tools. For writes it must own project_id; org settings affect all its projects."`
	ID             int            `json:"id,omitempty" jsonschema:"Entity ID from its list/show tool; schedule ID for override operations."`
	ChildID        int            `json:"child_id,omitempty" jsonschema:"Override ID when deleting a schedule override."`
	Data           map[string]any `json:"data,omitempty" jsonschema:"JSON request object in the API's camelCase format, as described by this tool and its knowledge guide. Full replacement tools must preserve unrelated settings. Never paste secrets into chat; use an approved credential channel."`
	From           string         `json:"from,omitempty" jsonschema:"Timeline start, RFC3339."`
	To             string         `json:"to,omitempty" jsonschema:"Timeline end, RFC3339, at most 62 days after from."`
}

func (s *server) addAlertingConfigurationTools(srv *mcp.Server, write bool) {
	for _, op := range client.AlertingOperations() {
		if op.ReadOnly == write {
			continue
		}
		// list_notification_rules already belongs to the established operations surface.
		if op.Resource == "alerts" && op.Action == "list" {
			continue
		}
		spec, ok := alertingToolSpecs[op.Resource+"/"+op.Action]
		if !ok {
			panic(fmt.Sprintf("missing MCP alerting tool for %s/%s", op.Resource, op.Action))
		}
		annotation := readOnly()
		if write {
			annotation = destructive()
			if op.Action == "create" || op.Action == "test" || op.Action == "resend-code" || op.Action == "create-override" {
				annotation = additive()
			}
		}
		description := spec.Description
		if write {
			description += " MUTATING: only on the user's request. project_id is required for organization opt-in and audit attribution, including personal settings."
		}
		mcp.AddTool(srv, &mcp.Tool{Name: spec.Name, Description: description, OutputSchema: passThroughSchema, Annotations: annotation},
			func(ctx context.Context, req *mcp.CallToolRequest, in alertingConfigIn) (*mcp.CallToolResult, any, error) {
				input := client.AlertingRequest{ID: in.ID, ChildID: in.ChildID, From: in.From, To: in.To}
				if in.OrganizationID != 0 {
					input.OrganizationID = strconv.Itoa(in.OrganizationID)
				}
				if !op.ReadOnly || op.Scope == "project" {
					id, err := s.project(in.ProjectID)
					if err != nil {
						return nil, nil, err
					}
					input.ProjectID = id
				} else {
					input.ProjectID = in.ProjectID
				}
				if in.Data != nil {
					body, err := json.Marshal(in.Data)
					if err != nil {
						return nil, nil, usageErrf("invalid data: %v", err)
					}
					input.Body = body
				}
				if err := client.ValidateAlertingRequest(op.Resource, op.Action, input); err != nil {
					return nil, nil, usageErrf("%v", err)
				}
				return s.raw(s.client(req).ConfigureAlerting(ctx, op.Resource, op.Action, input))
			})
	}
}
