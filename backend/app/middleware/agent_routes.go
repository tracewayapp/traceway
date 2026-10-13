package middleware

type agentAccess int

const (
	agentRead agentAccess = iota + 1
	agentWrite
)

// agentRoutes is the complete surface reachable with an agent credential
// (personal access tokens and CLI/MCP OAuth tokens). Anything missing is
// denied, so a new dashboard route is never exposed to agents by accident.
// Every write here is project-scoped, which is what lets the gate resolve the
// owning organization's opt-in from the projectId query parameter.
var agentRoutes = map[string]agentAccess{
	"GET /api/projects": agentRead,

	"GET /api/dashboard":                  agentRead,
	"GET /api/dashboard/overview":         agentRead,
	"GET /api/metrics/application":        agentRead,
	"GET /api/metrics/stats":              agentRead,
	"GET /api/metrics/server":             agentRead,
	"POST /api/metrics/query":             agentRead,
	"GET /api/metrics/discover":           agentRead,
	"GET /api/metrics/discover/tags":      agentRead,
	"GET /api/metrics/discover/instances": agentRead,
	"GET /api/metrics/discover/org":       agentRead,

	"GET /api/dashboards":            agentRead,
	"GET /api/dashboards/library":    agentRead,
	"GET /api/dashboards/starred":    agentRead,
	"GET /api/dashboards/:id":        agentRead,
	"GET /api/dashboards/:id/export": agentRead,
	"GET /api/dashboard-templates":   agentRead,

	"POST /api/endpoints":             agentRead,
	"POST /api/endpoints/grouped":     agentRead,
	"POST /api/endpoints/endpoint":    agentRead,
	"POST /api/endpoints/chart":       agentRead,
	"GET /api/endpoints/slow":         agentRead,
	"POST /api/endpoints/:endpointId": agentRead,
	"POST /api/endpoints/slow":        agentWrite,

	"POST /api/tasks":         agentRead,
	"POST /api/tasks/grouped": agentRead,
	"POST /api/tasks/task":    agentRead,
	"POST /api/tasks/:taskId": agentRead,

	"POST /api/sessions":            agentRead,
	"POST /api/sessions/:sessionId": agentRead,

	"POST /api/profiles/grouped":          agentRead,
	"POST /api/profiles/series":           agentRead,
	"POST /api/profiles/series/breakdown": agentRead,
	"POST /api/profiles/dimensions":       agentRead,
	"POST /api/profiles/types":            agentRead,
	"POST /api/profiles/flamegraph":       agentRead,
	"POST /api/profiles/top":              agentRead,
	"POST /api/profiles/labels":           agentRead,

	"POST /api/ai-traces/grouped":             agentRead,
	"POST /api/ai-traces/trace":               agentRead,
	"POST /api/ai-traces/:traceId":            agentRead,
	"POST /api/ai-conversations/grouped":      agentRead,
	"POST /api/ai-conversations/conversation": agentRead,
	"POST /api/ai-users/grouped":              agentRead,

	"POST /api/distributed-traces/:traceId":                   agentRead,
	"GET /api/otel/spans/:traceId/:spanId":                    agentRead,
	"POST /api/spans/search":                                  agentRead,
	"GET /api/spans/traces/:traceId":                          agentRead,
	"GET /api/spans/traces/:traceId/spans/:spanId/attributes": agentRead,

	"POST /api/logs": agentRead,

	"POST /api/exception-stack-traces":                    agentRead,
	"POST /api/exception-stack-traces/by-id/:exceptionId": agentRead,
	"POST /api/exception-stack-traces/:hash":              agentRead,
	"POST /api/exception-stack-traces/archive":            agentWrite,
	"POST /api/exception-stack-traces/unarchive":          agentWrite,

	"GET /api/notification-rules":             agentRead,
	"POST /api/notification-history":          agentRead,
	"POST /api/notification-rules/:id/toggle": agentWrite,
	"POST /api/notification-rules/:id/snooze": agentWrite,

	"GET /api/synthetics/checks":              agentRead,
	"POST /api/synthetics/overview":           agentRead,
	"GET /api/synthetics/checks/:id":          agentRead,
	"POST /api/synthetics/checks/:id/results": agentRead,
	"POST /api/synthetics/checks/:id/series":  agentRead,
	"GET /api/synthetics/open-count":          agentRead,
	"POST /api/synthetics/checks/:id/run":     agentWrite,

	"GET /api/organizations/:organizationId/overview/servers":    agentRead,
	"POST /api/organizations/:organizationId/overview/issues":    agentRead,
	"POST /api/organizations/:organizationId/overview/pages":     agentRead,
	"GET /api/organizations/:organizationId/overview/counts":     agentRead,
	"POST /api/organizations/:organizationId/overview/incidents": agentRead,
	"GET /api/organizations/:organizationId/overview/monitors":   agentRead,

	"GET /api/organizations/:organizationId/status-pages":                  agentRead,
	"GET /api/organizations/:organizationId/incidents":                     agentRead,
	"GET /api/organizations/:organizationId/incidents/:incidentId/updates": agentRead,
	"GET /api/organizations/:organizationId/status-pages/:id/incidents":    agentRead,

	"GET /api/post-mortems":              agentRead,
	"GET /api/post-mortems/:id":          agentRead,
	"GET /api/post-mortems/:id/activity": agentRead,
	"POST /api/post-mortems":             agentWrite,
	"PUT /api/post-mortems/:id":          agentWrite,

	"GET /api/organizations/:organizationId/teams":                          agentRead,
	"GET /api/organizations/:organizationId/schedules":                      agentRead,
	"GET /api/organizations/:organizationId/schedules/:scheduleId":          agentRead,
	"GET /api/organizations/:organizationId/schedules/:scheduleId/timeline": agentRead,
	"GET /api/organizations/:organizationId/oncall/now":                     agentRead,
	"GET /api/oncall/current":                                               agentRead,
	"GET /api/escalation-policies":                                          agentRead,
	"GET /api/organizations/:organizationId/escalation-policies":            agentRead,

	"POST /api/pages":                 agentRead,
	"POST /api/pages/for-issues":      agentRead,
	"GET /api/pages/open-count":       agentRead,
	"GET /api/pages/:id":              agentRead,
	"POST /api/pages/:id/acknowledge": agentWrite,
	"POST /api/pages/:id/resolve":     agentWrite,
}

func agentRouteAccess(method, fullPath string) (agentAccess, bool) {
	access, ok := agentRoutes[method+" "+fullPath]
	return access, ok
}

// AgentRouteKeys lists the allowlist as "METHOD /path" keys, so the route
// tree test can prove every entry names a registered route.
func AgentRouteKeys() []string {
	keys := make([]string, 0, len(agentRoutes))
	for k := range agentRoutes {
		keys = append(keys, k)
	}
	return keys
}
