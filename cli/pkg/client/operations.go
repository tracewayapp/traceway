package client

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// The operations below cover on-call pages, alert rules, monitors and
// post-mortems. Their responses are passed through verbatim: they are rendered
// as JSON by the CLI and returned as structured content by the MCP server, so
// a typed copy of every dashboard shape would only add drift.

func projectQuery(projectID string) string {
	return "?projectId=" + url.QueryEscape(projectID)
}

var PageStatuses = []string{"active", "open", "acknowledged", "resolved"}

type ListPagesRequest struct {
	Status     string           `json:"status,omitempty"`
	Pagination PaginationParams `json:"pagination"`
}

func (c *Client) ListPages(ctx context.Context, projectID string, req ListPagesRequest) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodPost, "/api/pages"+projectQuery(projectID), req, &resp)
	return resp, err
}

func (c *Client) GetPage(ctx context.Context, projectID string, id int) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/pages/"+strconv.Itoa(id)+projectQuery(projectID), nil, &resp)
	return resp, err
}

func (c *Client) AcknowledgePage(ctx context.Context, projectID string, id int) error {
	return c.do(ctx, http.MethodPost, "/api/pages/"+strconv.Itoa(id)+"/acknowledge"+projectQuery(projectID), struct{}{}, nil)
}

func (c *Client) ResolvePage(ctx context.Context, projectID string, id int, archiveIssue bool) error {
	body := struct {
		ArchiveIssue bool `json:"archiveIssue"`
	}{archiveIssue}
	return c.do(ctx, http.MethodPost, "/api/pages/"+strconv.Itoa(id)+"/resolve"+projectQuery(projectID), body, nil)
}

func (c *Client) ListNotificationRules(ctx context.Context, projectID string) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/notification-rules"+projectQuery(projectID), nil, &resp)
	return resp, err
}

// SetNotificationRuleEnabled sets the rule's state through the toggle route's
// explicit enabled field, so repeating it, or racing another toggle, still
// leaves the rule in the requested state. A server older than that field
// ignores it and flips the rule, so a rule already in the requested state comes
// back in the other one. The returned state is checked and such a flip undone.
func (c *Client) SetNotificationRuleEnabled(ctx context.Context, projectID string, id int, enabled bool) (json.RawMessage, error) {
	path := "/api/notification-rules/" + strconv.Itoa(id) + "/toggle" + projectQuery(projectID)
	body := struct {
		Enabled bool `json:"enabled"`
	}{enabled}
	var resp json.RawMessage
	if err := c.do(ctx, http.MethodPost, path, body, &resp); err != nil {
		return nil, err
	}
	if ruleEnabled(resp, enabled) == enabled {
		return resp, nil
	}

	resp = nil
	if err := c.do(ctx, http.MethodPost, path, body, &resp); err != nil {
		return nil, fmt.Errorf("the server flipped alert rule %d instead of setting it, and undoing that failed, so it is now %s: %w", id, enabledWord(!enabled), err)
	}
	if ruleEnabled(resp, enabled) != enabled {
		return nil, fmt.Errorf("alert rule %d is still %s after a second toggle; it may have been changed at the same time, so check its state", id, enabledWord(!enabled))
	}
	return resp, nil
}

// ruleEnabled reads a toggle response's enabled field, or returns fallback
// when the response has none to check.
func ruleEnabled(resp json.RawMessage, fallback bool) bool {
	var rule struct {
		Enabled *bool `json:"enabled"`
	}
	if json.Unmarshal(resp, &rule) != nil || rule.Enabled == nil {
		return fallback
	}
	return *rule.Enabled
}

func enabledWord(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// SnoozeNotificationRule silences a rule for durationMinutes; 0 clears the
// snooze.
func (c *Client) SnoozeNotificationRule(ctx context.Context, projectID string, id, durationMinutes int) (json.RawMessage, error) {
	body := struct {
		DurationMinutes int `json:"durationMinutes"`
	}{durationMinutes}
	var resp json.RawMessage
	err := c.do(ctx, http.MethodPost, "/api/notification-rules/"+strconv.Itoa(id)+"/snooze"+projectQuery(projectID), body, &resp)
	return resp, err
}

func (c *Client) ListChecks(ctx context.Context, projectID string) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/synthetics/checks"+projectQuery(projectID), nil, &resp)
	return resp, err
}

func (c *Client) GetCheck(ctx context.Context, projectID string, id int) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/synthetics/checks/"+strconv.Itoa(id)+projectQuery(projectID), nil, &resp)
	return resp, err
}

func (c *Client) RunCheck(ctx context.Context, projectID string, id int) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodPost, "/api/synthetics/checks/"+strconv.Itoa(id)+"/run"+projectQuery(projectID), struct{}{}, &resp)
	return resp, err
}

type ListPostMortemsRequest struct {
	Search   string
	Tags     []string
	Page     int
	PageSize int
}

func (c *Client) ListPostMortems(ctx context.Context, projectID string, req ListPostMortemsRequest) (json.RawMessage, error) {
	q := url.Values{"projectId": {projectID}}
	if req.Search != "" {
		q.Set("search", req.Search)
	}
	for _, tag := range req.Tags {
		q.Add("tag", tag)
	}
	if req.Page > 0 {
		q.Set("page", strconv.Itoa(req.Page))
	}
	if req.PageSize > 0 {
		q.Set("pageSize", strconv.Itoa(req.PageSize))
	}
	var resp json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/post-mortems?"+q.Encode(), nil, &resp)
	return resp, err
}

type PostMortem struct {
	Id         int      `json:"id"`
	Title      string   `json:"title"`
	ContentMd  string   `json:"contentMd"`
	Tags       []string `json:"tags"`
	IncidentId *int     `json:"incidentId"`
	UpdatedAt  string   `json:"updatedAt"`
}

type PostMortemInput struct {
	Title      string   `json:"title"`
	ContentMd  string   `json:"contentMd"`
	Tags       []string `json:"tags"`
	IncidentId *int     `json:"incidentId"`
	// ExpectedUpdatedAt makes an update fail with 409 when the document
	// changed after it was read, instead of overwriting that change.
	ExpectedUpdatedAt string `json:"expectedUpdatedAt,omitempty"`
}

func (c *Client) GetPostMortem(ctx context.Context, projectID string, id int) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/post-mortems/"+strconv.Itoa(id)+projectQuery(projectID), nil, &resp)
	return resp, err
}

func (c *Client) CreatePostMortem(ctx context.Context, projectID string, in PostMortemInput) (json.RawMessage, error) {
	if in.Tags == nil {
		in.Tags = []string{}
	}
	var resp json.RawMessage
	err := c.do(ctx, http.MethodPost, "/api/post-mortems"+projectQuery(projectID), in, &resp)
	return resp, err
}

// PostMortemPatch changes only the fields that are set; the server's update
// replaces the whole document, so the rest is read back first. The update is
// conditional on the document still being at ExpectedUpdatedAt, the revision
// the caller based its edit on, or at the one read here when that is empty
// (409 otherwise).
type PostMortemPatch struct {
	Title             *string
	ContentMd         *string
	Tags              *[]string
	IncidentId        *int
	UnlinkIncident    bool
	ExpectedUpdatedAt string
}

func (c *Client) UpdatePostMortem(ctx context.Context, projectID string, id int, patch PostMortemPatch) (json.RawMessage, error) {
	raw, err := c.GetPostMortem(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	current, err := decodePostMortem(raw)
	if err != nil {
		return nil, err
	}
	in := PostMortemInput{Title: current.Title, ContentMd: current.ContentMd, Tags: current.Tags, IncidentId: current.IncidentId, ExpectedUpdatedAt: cmp.Or(patch.ExpectedUpdatedAt, current.UpdatedAt)}
	if patch.Title != nil {
		in.Title = *patch.Title
	}
	if patch.ContentMd != nil {
		in.ContentMd = *patch.ContentMd
	}
	if patch.Tags != nil {
		in.Tags = *patch.Tags
	}
	if patch.IncidentId != nil {
		in.IncidentId = patch.IncidentId
	}
	if patch.UnlinkIncident {
		in.IncidentId = nil
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	var resp json.RawMessage
	err = c.do(ctx, http.MethodPut, "/api/post-mortems/"+strconv.Itoa(id)+projectQuery(projectID), in, &resp)
	return resp, err
}

func decodePostMortem(raw json.RawMessage) (*PostMortem, error) {
	var wrapped struct {
		PostMortem *PostMortem `json:"postMortem"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.PostMortem != nil {
		return wrapped.PostMortem, nil
	}
	var pm PostMortem
	if err := json.Unmarshal(raw, &pm); err != nil {
		return nil, err
	}
	return &pm, nil
}

type SetSlowEndpointRequest struct {
	Endpoint string `json:"endpoint"`
	OffsetMs uint32 `json:"offsetMs"`
	Reason   string `json:"reason"`
}

func (c *Client) SetSlowEndpoint(ctx context.Context, projectID string, req SetSlowEndpointRequest) error {
	return c.do(ctx, http.MethodPost, "/api/endpoints/slow"+projectQuery(projectID), req, nil)
}
