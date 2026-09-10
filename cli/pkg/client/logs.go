package client

import (
	"github.com/tracewayapp/traceway/cli/pkg/access"

	"context"
	"net/http"
	"net/url"
	"time"
)

// LogRecord is access.LogRecord, the normalized log shape.
type LogRecord = access.LogRecord

// QueryLogsRequest is the body for POST /api/logs.
type QueryLogsRequest struct {
	TimeRange     TimeRange        `json:"-"`
	Pagination    PaginationParams `json:"pagination"`
	OrderBy       string           `json:"orderBy,omitempty"`
	SortDirection string           `json:"sortDirection,omitempty"`
	Search        string           `json:"search,omitempty"`
	SearchType    string           `json:"searchType,omitempty"`
	MinSeverity   uint8            `json:"minSeverity,omitempty"`
	ServiceName   string           `json:"serviceName,omitempty"`
	TraceId       string           `json:"traceId,omitempty"`
}

// MarshalJSON expands TimeRange into top-level fromDate/toDate.
func (r QueryLogsRequest) MarshalJSON() ([]byte, error) {
	type alias QueryLogsRequest
	wire := struct {
		FromDate time.Time `json:"fromDate"`
		ToDate   time.Time `json:"toDate"`
		alias
	}{r.TimeRange.From, r.TimeRange.To, alias(r)}
	return jsonMarshalNoHTMLEscape(wire)
}

// QueryLogsResponse mirrors the upstream PaginatedResponse[LogRecord].
type QueryLogsResponse = access.LogPage

// QueryLogs returns one page of log records for the given project and filters.
func (c *Client) QueryLogs(ctx context.Context, projectID string, req QueryLogsRequest) (*QueryLogsResponse, error) {
	path := "/api/logs?projectId=" + url.QueryEscape(projectID)
	var resp QueryLogsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
