package ops

import (
	"strings"

	client "github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

// alertingMCP names the MCP tool of each alerting-configuration operation
// (client.AlertingOperations: channels, alerts, teams, schedules,
// escalation policies, contact methods, notification chains and history,
// on-call, SLOs), as cli/pkg/mcpserver/tools_alerting_configuration.go
// registers it. alerts/list is the established alerts.list op above.
var alertingMCP = map[string]string{
	"channels/list":               "list_notification_channels",
	"channels/create":             "create_notification_channel",
	"channels/update":             "update_notification_channel",
	"channels/delete":             "delete_notification_channel",
	"channels/test":               "test_notification_channel",
	"alerts/create":               "create_notification_rule",
	"alerts/update":               "update_notification_rule",
	"alerts/delete":               "delete_notification_rule",
	"teams/list":                  "list_teams",
	"teams/create":                "create_team",
	"teams/update":                "update_team",
	"teams/delete":                "delete_team",
	"teams/set-members":           "set_team_members",
	"teams/set-projects":          "set_team_projects",
	"schedules/list":              "list_oncall_schedules",
	"schedules/show":              "get_oncall_schedule",
	"schedules/create":            "create_oncall_schedule",
	"schedules/update":            "update_oncall_schedule",
	"schedules/delete":            "delete_oncall_schedule",
	"schedules/timeline":          "get_oncall_timeline",
	"schedules/create-override":   "create_oncall_override",
	"schedules/delete-override":   "delete_oncall_override",
	"escalation-policies/list":    "list_escalation_policies",
	"escalation-policies/create":  "create_escalation_policy",
	"escalation-policies/update":  "update_escalation_policy",
	"escalation-policies/delete":  "delete_escalation_policy",
	"contact-methods/list":        "list_contact_methods",
	"contact-methods/create":      "create_contact_method",
	"contact-methods/update":      "update_contact_method",
	"contact-methods/delete":      "delete_contact_method",
	"contact-methods/test":        "test_contact_method",
	"contact-methods/verify":      "verify_contact_method",
	"contact-methods/resend-code": "resend_contact_method_code",
	"notification-chains/show":    "get_user_notification_rules",
	"notification-chains/replace": "set_user_notification_rules",
	"notification-history/list":   "list_notification_history",
	"oncall/now":                  "get_oncall_now",
	"oncall/current":              "get_current_oncall",
	"slos/list":                   "list_slos",
	"slos/show":                   "get_slo",
	"slos/series":                 "get_slo_series",
	"slos/preview":                "preview_slo",
	"slos/create":                 "create_slo",
	"slos/update":                 "update_slo",
	"slos/delete":                 "delete_slo",
	"organization-members/list":   "list_organization_members",
}

// configSpecs are the dashboard, metric discovery and token commands. The
// write-edition-only ones are writes, except tokens list, which reads the
// user's tokens and exists only where they can be created.
var configSpecs = []Spec{
	{Op: "metrics.discover", CLI: []string{"metrics", "discover"}, MCP: "discover_metrics"},
	{Op: "dashboards.list", CLI: []string{"dashboards", "list"}, MCP: "list_dashboards"},
	{Op: "dashboards.library", CLI: []string{"dashboards", "library"}, MCP: "dashboard_library"},
	{Op: "dashboards.export", CLI: []string{"dashboards", "export"}, MCP: "export_dashboard", Positional: []string{"id"}},
	{Op: "dashboards.validate", CLI: []string{"dashboards", "validate"}, Rename: map[string]string{"file": "data"}},
	{Op: "dashboards.import", CLI: []string{"dashboards", "import"}, MCP: "import_dashboards", Write: true, WriteEdition: true, Rename: map[string]string{"file": "data"}},
	{Op: "dashboards.embed", CLI: []string{"dashboards", "embed"}, MCP: "create_dashboard_embed", Write: true, WriteEdition: true, Positional: []string{"id"}, Rename: map[string]string{"tag": "tag_filters"}},
	{Op: "tokens.create", CLI: []string{"tokens", "create"}, MCP: "create_read_token", Write: true, WriteEdition: true},
	{Op: "tokens.list", CLI: []string{"tokens", "list"}, MCP: "list_personal_access_tokens", WriteEdition: true},
}

func init() {
	specs = append(specs, configSpecs...)
	validated := map[string]bool{}
	for _, a := range client.AlertingOperations() {
		key := a.Resource + "/" + a.Action
		if key == "alerts/list" {
			continue
		}
		s := Spec{
			Op: opName(a.Resource, a.Action), CLI: []string{a.Resource, a.Action}, MCP: alertingMCP[key],
			Write: !a.ReadOnly, WriteEdition: !a.ReadOnly,
			Rename: map[string]string{"organization": "organization_id", "file": "data"},
		}
		if a.ID {
			s.Positional = append(s.Positional, "id")
		}
		if a.ChildID {
			s.Positional = append(s.Positional, "child_id")
		}
		specs = append(specs, s)
		// `<resource> validate` checks a request file offline, in both
		// editions, for every resource with an operation that takes one.
		if a.Body && !validated[a.Resource] {
			validated[a.Resource] = true
			specs = append(specs, Spec{Op: opName(a.Resource, "validate"), CLI: []string{a.Resource, "validate"},
				Positional: []string{"action", "id", "child_id"}, Rename: map[string]string{"file": "data"}})
		}
	}
}

func opName(resource, action string) string {
	return strings.ReplaceAll(resource, "-", "_") + "." + strings.ReplaceAll(action, "-", "_")
}
