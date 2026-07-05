package client

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// SessionAttributeFilter is one exact key=value match against a session's
// attributes map (no scope — sessions have a single attribute map).
type SessionAttributeFilter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ListSessionsRequest is the body for POST /api/sessions. OrderBy uses the
// server's snake_case field names (started_at, duration).
type ListSessionsRequest struct {
	TimeRange        TimeRange                `json:"-"`
	Pagination       PaginationParams         `json:"pagination"`
	OrderBy          string                   `json:"orderBy,omitempty"`
	SortDirection    string                   `json:"sortDirection,omitempty"`
	Search           string                   `json:"search,omitempty"`
	AttributeFilters []SessionAttributeFilter `json:"attributeFilters,omitempty"`
}

// MarshalJSON expands TimeRange into top-level fromDate/toDate.
func (r ListSessionsRequest) MarshalJSON() ([]byte, error) {
	type alias ListSessionsRequest
	wire := struct {
		FromDate time.Time `json:"fromDate"`
		ToDate   time.Time `json:"toDate"`
		alias
	}{r.TimeRange.From, r.TimeRange.To, alias(r)}
	return jsonMarshalNoHTMLEscape(wire)
}

// ListSessionsResponse mirrors PaginatedResponse[Session].
type ListSessionsResponse struct {
	Data       []Session  `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// ListSessions returns user sessions in the window. Each session's id +
// startedAt feed the by-id GetSession lookup.
func (c *Client) ListSessions(ctx context.Context, projectID string, req ListSessionsRequest) (*ListSessionsResponse, error) {
	path := "/api/sessions?projectId=" + url.QueryEscape(projectID)
	var resp ListSessionsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
