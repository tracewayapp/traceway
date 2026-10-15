package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func (s *server) addOperationReadTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:         "list_pages",
		OutputSchema: passThroughSchema,
		Description:  "List on-call pages (alerts that paged a person through an escalation policy). status: active (open or acknowledged, the default), open, acknowledged, resolved. A page carries its rule, subject, urgency, eventCount and, for issue pages, the exceptionHash it was opened for.",
		Annotations:  readOnly(),
	}, s.listPages)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "get_page",
		OutputSchema: passThroughSchema,
		Description:  "Get one on-call page by its numeric id plus its delivery log: who was notified, on which contact method, at which escalation level, and whether it was acknowledged by link.",
		Annotations:  readOnly(),
	}, s.getPage)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "list_notification_rules",
		OutputSchema: passThroughSchema,
		Description:  "List the project's alert rules: id, name, ruleType (new_error, error_regression, error_rate_threshold, error_count_threshold, endpoint_p95_threshold, endpoint_p99_threshold, endpoint_error_rate, apdex_drop, throughput_drop, metric_threshold, no_data, task_duration_threshold, task_failure_rate, impact_score_*, ai_trace_cost, ai_conversation_cost, ai_flagged_content, check_down), enabled, snoozedUntil, threshold config and the channels they notify.",
		Annotations:  readOnly(),
	}, s.listNotificationRules)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "list_checks",
		OutputSchema: passThroughSchema,
		Description:  "List the project's monitors (synthetic checks): id, name, checkType (http, tcp, browser), config (URL or host and assertions), intervalSeconds, enabled, currentStatus (up, down or pending) and consecutiveFailures.",
		Annotations:  readOnly(),
	}, s.listChecks)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "get_check",
		OutputSchema: passThroughSchema,
		Description:  "Get one monitor by numeric id with its recent incidents (when it went down, when it recovered, incident titles and linked post-mortems).",
		Annotations:  readOnly(),
	}, s.getCheck)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "list_post_mortems",
		OutputSchema: passThroughSchema,
		Description:  "List the project's post-mortems, newest first. search matches title, content and tags; tags are ANDed.",
		Annotations:  readOnly(),
	}, s.listPostMortems)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "get_post_mortem",
		OutputSchema: passThroughSchema,
		Description:  "Get one post-mortem by numeric id: title, markdown content, tags, the linked incident and author names.",
		Annotations:  readOnly(),
	}, s.getPostMortem)
}

func (s *server) addOperationWriteTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:         "acknowledge_page",
		OutputSchema: passThroughSchema,
		Description:  "Acknowledge an open on-call page: it stops escalating to the next level. MUTATING and visible to the whole team. Only call when the user asked to acknowledge this page.",
		Annotations:  mutating(),
	}, s.acknowledgePage)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "resolve_page",
		OutputSchema: passThroughSchema,
		Description:  "Resolve an open or acknowledged on-call page. archive_issue also archives the issue the page was opened for. MUTATING and not undoable from the API. Only call when the user said the problem is handled.",
		Annotations:  destructive(),
	}, s.resolvePage)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "set_notification_rule_enabled",
		OutputSchema: passThroughSchema,
		Description:  "Turn an alert rule on or off. Idempotent: it sets the requested state, so a rule already in it is left alone. MUTATING. A disabled rule notifies nobody until re-enabled, so say so to the user.",
		Annotations:  mutating(),
	}, s.setNotificationRuleEnabled)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "snooze_notification_rule",
		OutputSchema: passThroughSchema,
		Description:  "Snooze an alert rule for duration_minutes from now (it is skipped entirely and comes back on its own); calling it again restarts the snooze, and 0 clears it. MUTATING. Prefer this over disabling for a known incident.",
		Annotations:  mutatingRepeatable(),
	}, s.snoozeNotificationRule)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "run_check",
		OutputSchema: passThroughSchema,
		Description:  "Queue an immediate run of a monitor, for example to confirm a fix. Fails with 422 when a run is already queued or browser checks are disabled. The result shows up in get_check after the run.",
		Annotations:  additive(),
	}, s.runCheck)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "create_post_mortem",
		OutputSchema: passThroughSchema,
		Description:  "Create a post-mortem document in the project. content_md is markdown. incident_id optionally links the incident it covers (one post-mortem per incident). MUTATING: the document is visible to the team.",
		Annotations:  additive(),
	}, s.createPostMortem)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "update_post_mortem",
		OutputSchema: passThroughSchema,
		Description:  "Update a post-mortem. Only the fields you pass change; content_md replaces the whole body, so read it with get_post_mortem, send the full revised text, and pass that read's updatedAt as expected_updated_at. If someone edited the document since that read, the call fails with a 409 conflict instead of overwriting their change: read it again and reapply. Every edit is recorded in the post-mortem's activity log. MUTATING.",
		Annotations:  mutating(),
	}, s.updatePostMortem)

	mcp.AddTool(srv, &mcp.Tool{
		Name:         "set_slow_endpoint",
		OutputSchema: passThroughSchema,
		Description:  "Mark an endpoint as expected-slow: offset_ms is the accepted latency allowance and reason the note shown to the team. offset_ms 0 removes the mark. It shifts apdex and impact scoring for that endpoint, so only set it when the user decided the latency is acceptable. MUTATING.",
		Annotations:  mutating(),
	}, s.setSlowEndpoint)
}

type pageIdIn struct {
	projectIn
	ID int `json:"id" jsonschema:"The page's numeric id, from list_pages or a /pages/<id> URL."`
}

type listPagesIn struct {
	projectIn
	pageIn
	Status string `json:"status,omitempty" jsonschema:"active (default), open, acknowledged, or resolved."`
}

func (s *server) listPages(ctx context.Context, req *mcp.CallToolRequest, in listPagesIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	page, err := in.pageIn.resolve()
	if err != nil {
		return nil, nil, err
	}
	if err := validateEnum("status", in.Status, client.PageStatuses); err != nil {
		return nil, nil, err
	}
	status := in.Status
	if status == "" {
		status = "active"
	}
	return s.raw(s.client(req).ListPages(ctx, projectID, client.ListPagesRequest{Status: status, Pagination: page}))
}

func (s *server) getPage(ctx context.Context, req *mcp.CallToolRequest, in pageIdIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).GetPage(ctx, projectID, in.ID))
}

func (s *server) acknowledgePage(ctx context.Context, req *mcp.CallToolRequest, in pageIdIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	if err := s.client(req).AcknowledgePage(ctx, projectID, in.ID); err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, map[string]any{"status": "acknowledged", "id": in.ID}, nil
}

type resolvePageIn struct {
	pageIdIn
	ArchiveIssue bool `json:"archive_issue,omitempty" jsonschema:"Also archive the issue the page was opened for. Only when the user asked for it."`
}

func (s *server) resolvePage(ctx context.Context, req *mcp.CallToolRequest, in resolvePageIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	if err := s.client(req).ResolvePage(ctx, projectID, in.ID, in.ArchiveIssue); err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, map[string]any{"status": "resolved", "id": in.ID, "archivedIssue": in.ArchiveIssue}, nil
}

func (s *server) listNotificationRules(ctx context.Context, req *mcp.CallToolRequest, in projectIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).ListNotificationRules(ctx, projectID))
}

type ruleEnabledIn struct {
	projectIn
	ID      int  `json:"id" jsonschema:"The rule's numeric id, from list_notification_rules."`
	Enabled bool `json:"enabled" jsonschema:"true to turn the rule on, false to turn it off."`
}

func (s *server) setNotificationRuleEnabled(ctx context.Context, req *mcp.CallToolRequest, in ruleEnabledIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).SetNotificationRuleEnabled(ctx, projectID, in.ID, in.Enabled))
}

type ruleSnoozeIn struct {
	projectIn
	ID              int `json:"id" jsonschema:"The rule's numeric id, from list_notification_rules."`
	DurationMinutes int `json:"duration_minutes" jsonschema:"How long to snooze, in minutes. 0 clears the snooze."`
}

func (s *server) snoozeNotificationRule(ctx context.Context, req *mcp.CallToolRequest, in ruleSnoozeIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	if in.DurationMinutes < 0 {
		return nil, nil, usageErrf("duration_minutes must be 0 or greater")
	}
	return s.raw(s.client(req).SnoozeNotificationRule(ctx, projectID, in.ID, in.DurationMinutes))
}

func (s *server) listChecks(ctx context.Context, req *mcp.CallToolRequest, in projectIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).ListChecks(ctx, projectID))
}

type checkIdIn struct {
	projectIn
	ID int `json:"id" jsonschema:"The monitor's numeric id, from list_checks or a /monitors/<id> URL."`
}

func (s *server) getCheck(ctx context.Context, req *mcp.CallToolRequest, in checkIdIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).GetCheck(ctx, projectID, in.ID))
}

func (s *server) runCheck(ctx context.Context, req *mcp.CallToolRequest, in checkIdIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).RunCheck(ctx, projectID, in.ID))
}

type listPostMortemsIn struct {
	projectIn
	pageIn
	Search string   `json:"search,omitempty" jsonschema:"Matches title, content and tags."`
	Tags   []string `json:"tags,omitempty" jsonschema:"Only post-mortems carrying every one of these tags."`
}

func (s *server) listPostMortems(ctx context.Context, req *mcp.CallToolRequest, in listPostMortemsIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	page, err := in.pageIn.resolve()
	if err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).ListPostMortems(ctx, projectID, client.ListPostMortemsRequest{
		Search: in.Search, Tags: in.Tags, Page: page.Page, PageSize: page.PageSize,
	}))
}

type postMortemIdIn struct {
	projectIn
	ID int `json:"id" jsonschema:"The post-mortem's numeric id, from list_post_mortems or a /monitors/post-mortems/<id> URL."`
}

func (s *server) getPostMortem(ctx context.Context, req *mcp.CallToolRequest, in postMortemIdIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).GetPostMortem(ctx, projectID, in.ID))
}

type createPostMortemIn struct {
	projectIn
	Title      string   `json:"title" jsonschema:"Document title, required."`
	ContentMd  string   `json:"content_md,omitempty" jsonschema:"Markdown body. A good post-mortem covers summary, impact, timeline (UTC), root cause, resolution and follow-up actions."`
	Tags       []string `json:"tags,omitempty" jsonschema:"Short labels such as database or deploy."`
	IncidentID int      `json:"incident_id,omitempty" jsonschema:"Optional incident to link, from get_check incidents. One post-mortem per incident."`
}

func (s *server) createPostMortem(ctx context.Context, req *mcp.CallToolRequest, in createPostMortemIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if in.Title == "" {
		return nil, nil, usageErrf("title is required")
	}
	input := client.PostMortemInput{Title: in.Title, ContentMd: in.ContentMd, Tags: in.Tags}
	if in.IncidentID > 0 {
		input.IncidentId = &in.IncidentID
	}
	return s.raw(s.client(req).CreatePostMortem(ctx, projectID, input))
}

type updatePostMortemIn struct {
	projectIn
	ID                int       `json:"id" jsonschema:"The post-mortem's numeric id."`
	Title             *string   `json:"title,omitempty" jsonschema:"New title."`
	ContentMd         *string   `json:"content_md,omitempty" jsonschema:"New markdown body, replacing the whole document. Requires expected_updated_at."`
	Tags              *[]string `json:"tags,omitempty" jsonschema:"New tag list, replacing the old one."`
	IncidentID        int       `json:"incident_id,omitempty" jsonschema:"Link this incident."`
	UnlinkIncident    bool      `json:"unlink_incident,omitempty" jsonschema:"Remove the incident link."`
	ExpectedUpdatedAt string    `json:"expected_updated_at,omitempty" jsonschema:"The updatedAt of the version your edit is based on, exactly as get_post_mortem returned it. If the document changed since, the update fails with a 409 instead of overwriting that change."`
}

func (s *server) updatePostMortem(ctx context.Context, req *mcp.CallToolRequest, in updatePostMortemIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	if in.ContentMd != nil && in.ExpectedUpdatedAt == "" {
		return nil, nil, usageErrf("content_md replaces the whole body, so expected_updated_at is required: pass the updatedAt from the get_post_mortem your revision is based on")
	}
	if in.IncidentID > 0 && in.UnlinkIncident {
		return nil, nil, usageErrf("pass incident_id or unlink_incident, not both")
	}
	patch := client.PostMortemPatch{Title: in.Title, ContentMd: in.ContentMd, Tags: in.Tags, UnlinkIncident: in.UnlinkIncident, ExpectedUpdatedAt: in.ExpectedUpdatedAt}
	if in.IncidentID > 0 {
		patch.IncidentId = &in.IncidentID
	}
	return s.raw(s.client(req).UpdatePostMortem(ctx, projectID, in.ID, patch))
}

type setSlowEndpointIn struct {
	projectIn
	Endpoint string `json:"endpoint" jsonschema:"The endpoint name exactly as list_endpoints shows it, like GET /api/users/:id."`
	OffsetMs uint32 `json:"offset_ms" jsonschema:"Accepted extra latency in milliseconds. 0 removes the mark."`
	Reason   string `json:"reason,omitempty" jsonschema:"Why this latency is acceptable, shown to the team."`
}

func (s *server) setSlowEndpoint(ctx context.Context, req *mcp.CallToolRequest, in setSlowEndpointIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if in.Endpoint == "" {
		return nil, nil, usageErrf("endpoint is required")
	}
	if err := s.client(req).SetSlowEndpoint(ctx, projectID, client.SetSlowEndpointRequest{Endpoint: in.Endpoint, OffsetMs: in.OffsetMs, Reason: in.Reason}); err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, map[string]any{"status": "updated", "endpoint": in.Endpoint, "offsetMs": in.OffsetMs}, nil
}

// raw adapts a pass-through client call to a tool result: structured content
// must be a JSON object, so arrays are wrapped under "data".
func (s *server) raw(resp json.RawMessage, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	var v any
	if len(resp) == 0 {
		return nil, map[string]any{"status": "ok"}, nil
	}
	if err := json.Unmarshal(resp, &v); err != nil {
		return nil, nil, err
	}
	if _, ok := v.(map[string]any); !ok {
		v = map[string]any{"data": v}
	}
	return nil, v, nil
}

func positiveID(param string, id int) error {
	if id < 1 {
		return usageErrf("%s must be a positive integer", param)
	}
	return nil
}

func additive() *mcp.ToolAnnotations {
	nonDestructive := false
	return &mcp.ToolAnnotations{DestructiveHint: &nonDestructive}
}

func destructive() *mcp.ToolAnnotations {
	isDestructive := true
	return &mcp.ToolAnnotations{DestructiveHint: &isDestructive}
}
