package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type AlertingOperation struct {
	Resource string
	Action   string
	Scope    string
	Method   string
	Path     string
	Body     bool
	ID       bool
	ChildID  bool
	ReadOnly bool
}

type AlertingRequest struct {
	ProjectID      string          `json:"projectId,omitempty"`
	OrganizationID string          `json:"organizationId,omitempty"`
	ID             int             `json:"id,omitempty"`
	ChildID        int             `json:"childId,omitempty"`
	Body           json.RawMessage `json:"body,omitempty"`
	From           string          `json:"from,omitempty"`
	To             string          `json:"to,omitempty"`
}

func AlertingOperations() []AlertingOperation {
	var ops []AlertingOperation
	add := func(resource, action, scope, method, path string, body, read bool) {
		ops = append(ops, AlertingOperation{Resource: resource, Action: action, Scope: scope, Method: method, Path: path, Body: body, ID: strings.Contains(path, "{id}"), ChildID: strings.Contains(path, "{childId}"), ReadOnly: read})
	}
	for _, r := range []struct{ resource, scope, path string }{
		{"channels", "project", "/api/notification-channels"}, {"alerts", "project", "/api/notification-rules"},
		{"teams", "organization", "/api/organizations/{organizationId}/teams"}, {"schedules", "organization", "/api/organizations/{organizationId}/schedules"},
		{"escalation-policies", "organization", "/api/organizations/{organizationId}/escalation-policies"}, {"contact-methods", "user", "/api/contact-methods"},
	} {
		add(r.resource, "list", r.scope, http.MethodGet, r.path, false, true)
		add(r.resource, "create", r.scope, http.MethodPost, r.path, true, false)
		add(r.resource, "update", r.scope, http.MethodPut, r.path+"/{id}", true, false)
		add(r.resource, "delete", r.scope, http.MethodDelete, r.path+"/{id}", false, false)
	}
	add("channels", "test", "project", http.MethodPost, "/api/notification-channels/{id}/test", false, false)
	add("organization-members", "list", "organization", http.MethodGet, "/api/organizations/{organizationId}/members", false, true)
	add("teams", "set-members", "organization", http.MethodPut, "/api/organizations/{organizationId}/teams/{id}/members", true, false)
	add("teams", "set-projects", "organization", http.MethodPut, "/api/organizations/{organizationId}/teams/{id}/projects", true, false)
	add("schedules", "show", "organization", http.MethodGet, "/api/organizations/{organizationId}/schedules/{id}", false, true)
	add("schedules", "timeline", "organization", http.MethodGet, "/api/organizations/{organizationId}/schedules/{id}/timeline", false, true)
	add("schedules", "create-override", "organization", http.MethodPost, "/api/organizations/{organizationId}/schedules/{id}/overrides", true, false)
	add("schedules", "delete-override", "organization", http.MethodDelete, "/api/organizations/{organizationId}/schedules/{id}/overrides/{childId}", false, false)
	for _, action := range []string{"test", "verify", "resend-code"} {
		add("contact-methods", action, "user", http.MethodPost, "/api/contact-methods/{id}/"+action, action == "verify", false)
	}
	add("notification-chains", "show", "user", http.MethodGet, "/api/user-notification-rules", false, true)
	add("notification-chains", "replace", "user", http.MethodPut, "/api/user-notification-rules", true, false)
	add("notification-history", "list", "project", http.MethodPost, "/api/notification-history", true, true)
	add("oncall", "now", "organization", http.MethodGet, "/api/organizations/{organizationId}/oncall/now", false, true)
	add("oncall", "current", "project", http.MethodGet, "/api/oncall/current", false, true)
	return ops
}

func alertingOperation(resource, action string) (AlertingOperation, error) {
	for _, op := range AlertingOperations() {
		if op.Resource == resource && op.Action == action {
			return op, nil
		}
	}
	return AlertingOperation{}, fmt.Errorf("unsupported alerting operation %q %q", resource, action)
}

func ValidateAlertingRequest(resource, action string, req AlertingRequest) error {
	op, err := alertingOperation(resource, action)
	if err != nil {
		return err
	}
	if (op.Scope == "project" || !op.ReadOnly) && strings.TrimSpace(req.ProjectID) == "" {
		return fmt.Errorf("projectId is required")
	}
	if op.Scope == "organization" {
		if _, err := strconv.Atoi(req.OrganizationID); err != nil {
			return fmt.Errorf("organizationId must be a positive integer")
		}
		id, _ := strconv.Atoi(req.OrganizationID)
		if id < 1 {
			return fmt.Errorf("organizationId must be a positive integer")
		}
	}
	if op.ID && req.ID < 1 {
		return fmt.Errorf("id must be a positive integer")
	}
	if op.ChildID && req.ChildID < 1 {
		return fmt.Errorf("childId must be a positive integer")
	}
	if action == "timeline" {
		from, e1 := time.Parse(time.RFC3339, req.From)
		to, e2 := time.Parse(time.RFC3339, req.To)
		if e1 != nil || e2 != nil || !from.Before(to) || to.Sub(from) > 62*24*time.Hour {
			return fmt.Errorf("timeline requires RFC3339 from/to, ordered and at most 62 days apart")
		}
	}
	if op.Body {
		return validateAlertingBody(resource, action, req.Body)
	}
	if len(req.Body) > 0 {
		return fmt.Errorf("this operation does not accept a body")
	}
	return nil
}

func validateAlertingBody(resource, action string, body json.RawMessage) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		return fmt.Errorf("body must be a JSON object")
	}
	var fields, required string
	switch resource {
	case "channels":
		fields = "name channelType config enabled"
		required = "name channelType config"
	case "alerts":
		fields = "channelId name ruleType config cooldownMinutes severity"
		required = "channelId name ruleType config"
	case "teams":
		fields = "name description memberUserIds projectIds"
		required = "name"
		if action == "set-members" {
			fields = "userIds"
			required = fields
		}
		if action == "set-projects" {
			fields = "projectIds"
			required = fields
		}
	case "schedules":
		fields = "teamId name description timezone definition"
		required = "teamId name timezone definition"
		if action == "create-override" {
			fields = "userId startAt endAt"
			required = fields
		}
	case "escalation-policies":
		fields = "name definition"
		required = fields
	case "contact-methods":
		fields = "methodType config enabled"
		required = "methodType config"
		if action == "update" {
			required = ""
			if len(obj) == 0 {
				return fmt.Errorf("at least one field is required")
			}
		}
		if action == "verify" {
			fields = "code"
			required = fields
		}
	case "notification-chains":
		fields = "high low"
		required = fields
	case "notification-history":
		fields = "pagination search fromDate toDate"
		required = "pagination"
	}
	for k := range obj {
		if !strings.Contains(" "+fields+" ", " "+k+" ") {
			return fmt.Errorf("unknown %s field %q", resource, k)
		}
	}
	for _, k := range strings.Fields(required) {
		if len(obj[k]) == 0 || string(obj[k]) == "null" {
			return fmt.Errorf("%s is required", k)
		}
	}

	for k, raw := range obj {
		switch k {
		case "name", "channelType", "ruleType", "methodType", "timezone", "code", "description", "severity", "startAt", "endAt", "search", "fromDate", "toDate":
			var value string
			if string(raw) == "null" || json.Unmarshal(raw, &value) != nil {
				return fmt.Errorf("%s must be a string", k)
			}
			if k == "code" {
				if len(value) != 6 || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
					return fmt.Errorf("code must contain exactly six ASCII digits")
				}
			}
			if k != "description" && k != "search" && k != "severity" && k != "fromDate" && k != "toDate" && strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s must not be empty", k)
			}
		case "config", "definition":
			var value map[string]json.RawMessage
			if json.Unmarshal(raw, &value) != nil || value == nil {
				return fmt.Errorf("%s must be a JSON object", k)
			}
		case "channelId", "teamId", "userId", "cooldownMinutes":
			var value int
			if string(raw) == "null" || json.Unmarshal(raw, &value) != nil || value < 0 || (k != "cooldownMinutes" && value == 0) {
				return fmt.Errorf("%s must be a %s integer", k, map[bool]string{true: "non-negative", false: "positive"}[k == "cooldownMinutes"])
			}
		case "enabled":
			var value bool
			if string(raw) == "null" || json.Unmarshal(raw, &value) != nil {
				return fmt.Errorf("enabled must be a boolean")
			}
		case "userIds", "memberUserIds":
			var value []int
			if json.Unmarshal(raw, &value) != nil || value == nil {
				return fmt.Errorf("%s must be an array of positive integers", k)
			}
			for _, id := range value {
				if id < 1 {
					return fmt.Errorf("%s must contain positive integers", k)
				}
			}
		case "projectIds":
			var value []string
			if json.Unmarshal(raw, &value) != nil || value == nil {
				return fmt.Errorf("projectIds must be an array of project IDs")
			}
			for _, id := range value {
				if strings.TrimSpace(id) == "" {
					return fmt.Errorf("projectIds must not contain empty IDs")
				}
			}
		}
	}
	if resource == "notification-chains" {
		for _, k := range []string{"high", "low"} {
			var steps []struct {
				ID              int `json:"id,omitempty"`
				ContactMethodID int `json:"contactMethodId"`
				DelayMinutes    int `json:"delayMinutes"`
			}
			var rawSteps []map[string]json.RawMessage
			if json.Unmarshal(obj[k], &rawSteps) != nil {
				return fmt.Errorf("%s must be an array of notification steps", k)
			}
			for _, step := range rawSteps {
				for field := range step {
					if field != "id" && field != "contactMethodId" && field != "delayMinutes" {
						return fmt.Errorf("unknown notification step field %q", field)
					}
				}
			}
			if json.Unmarshal(obj[k], &steps) != nil || steps == nil || len(steps) > 10 {
				return fmt.Errorf("%s must be an array of at most 10 notification steps", k)
			}
			for _, step := range steps {
				if step.ContactMethodID < 1 || step.DelayMinutes < 0 || step.DelayMinutes > 120 {
					return fmt.Errorf("%s steps require positive contactMethodId and delayMinutes between 0 and 120", k)
				}
			}
		}
	}
	if action == "create-override" {
		var start, end string
		_ = json.Unmarshal(obj["startAt"], &start)
		_ = json.Unmarshal(obj["endAt"], &end)
		a, e1 := time.Parse(time.RFC3339, start)
		b, e2 := time.Parse(time.RFC3339, end)
		if e1 != nil || e2 != nil || !a.Before(b) {
			return fmt.Errorf("startAt/endAt must be ordered RFC3339 timestamps")
		}
	}
	if resource == "notification-history" {
		var p PaginationParams
		if err := json.Unmarshal(obj["pagination"], &p); err != nil || p.Page < 1 || p.PageSize < 1 || p.PageSize > 100 {
			return fmt.Errorf("pagination requires page >= 1 and pageSize between 1 and 100")
		}
	}
	return nil
}

func (c *Client) ConfigureAlerting(ctx context.Context, resource, action string, req AlertingRequest) (json.RawMessage, error) {
	if err := ValidateAlertingRequest(resource, action, req); err != nil {
		return nil, err
	}
	op, _ := alertingOperation(resource, action)
	path := strings.NewReplacer("{organizationId}", url.PathEscape(req.OrganizationID), "{id}", strconv.Itoa(req.ID), "{childId}", strconv.Itoa(req.ChildID)).Replace(op.Path)
	query := url.Values{}
	query.Set("projectId", req.ProjectID)
	if action == "timeline" {
		query.Set("from", req.From)
		query.Set("to", req.To)
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	var payload any
	if op.Body {
		payload = req.Body
	} else if op.Method == http.MethodPost {
		payload = struct{}{}
	}
	var response json.RawMessage
	err := c.do(ctx, op.Method, path, payload, &response)
	return response, err
}
