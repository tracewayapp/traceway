package client

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// LogRecord matches the upstream models.LogRecord (subset — we drop fields
// we don't surface in v1, like resource/scope schema URLs).
type LogRecord struct {
	Id                 uuid.UUID         `json:"id"`
	Timestamp          time.Time         `json:"timestamp"`
	SeverityText       string            `json:"severityText"`
	SeverityNumber     uint8             `json:"severityNumber"`
	ServiceName        string            `json:"serviceName"`
	Body               string            `json:"body"`
	TraceId            string            `json:"traceId,omitempty"`
	SpanId             string            `json:"spanId,omitempty"`
	ResourceAttributes map[string]string `json:"resourceAttributes,omitempty"`
	ScopeName          string            `json:"scopeName,omitempty"`
	LogAttributes      map[string]string `json:"logAttributes,omitempty"`
}

// LogAttributeFilter matches one entry of the upstream attributeFilters —
// an exact key=value match against one of the three attribute maps. Scope is
// "resource", "scope", or "log".
type LogAttributeFilter struct {
	Scope string `json:"scope"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// QueryLogsRequest is the body for POST /api/logs. DistributedTraceId filters
// to every OTel trace under one distributed trace (the server resolves the
// member trace ids first); ExcludeTraceId drops one of those member traces and
// is only honored together with DistributedTraceId.
type QueryLogsRequest struct {
	TimeRange          TimeRange            `json:"-"`
	Pagination         PaginationParams     `json:"pagination"`
	OrderBy            string               `json:"orderBy,omitempty"`
	SortDirection      string               `json:"sortDirection,omitempty"`
	Search             string               `json:"search,omitempty"`
	SearchType         string               `json:"searchType,omitempty"`
	MinSeverity        uint8                `json:"minSeverity,omitempty"`
	ServiceName        string               `json:"serviceName,omitempty"`
	TraceId            string               `json:"traceId,omitempty"`
	DistributedTraceId string               `json:"distributedTraceId,omitempty"`
	ExcludeTraceId     string               `json:"excludeTraceId,omitempty"`
	AttributeFilters   []LogAttributeFilter `json:"attributeFilters,omitempty"`
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
type QueryLogsResponse struct {
	Data       []LogRecord `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// QueryLogs returns one page of log records for the given project and filters.
func (c *Client) QueryLogs(ctx context.Context, projectID string, req QueryLogsRequest) (*QueryLogsResponse, error) {
	path := "/api/logs?projectId=" + url.QueryEscape(projectID)
	var resp QueryLogsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
