package client

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// AiTraceStats matches the upstream models.AiTraceStats — aggregates for one
// AI trace name. Durations are time.Duration values (nanoseconds on the wire).
type AiTraceStats struct {
	TraceName       string        `json:"traceName"`
	Count           uint64        `json:"count"`
	P50Duration     time.Duration `json:"p50Duration"`
	P95Duration     time.Duration `json:"p95Duration"`
	AvgDuration     time.Duration `json:"avgDuration"`
	TotalTokens     int64         `json:"totalTokens"`
	TotalCost       float64       `json:"totalCost"`
	AvgInputTokens  float64       `json:"avgInputTokens"`
	AvgOutputTokens float64       `json:"avgOutputTokens"`
	LastSeen        time.Time     `json:"lastSeen"`
	HasRoot         bool          `json:"hasRoot"`
	HasNonRoot      bool          `json:"hasNonRoot"`
}

// ListAiTracesRequest is the body for POST /api/ai-traces/grouped. OrderBy
// uses the server's snake_case field names (count, p50_duration, p95_duration,
// avg_duration, total_tokens, total_cost, last_seen); RootFilter is "root",
// "non_root", or "" for all.
type ListAiTracesRequest struct {
	TimeRange     TimeRange        `json:"-"`
	Pagination    PaginationParams `json:"pagination"`
	OrderBy       string           `json:"orderBy,omitempty"`
	SortDirection string           `json:"sortDirection,omitempty"`
	Search        string           `json:"search,omitempty"`
	RootFilter    string           `json:"rootFilter,omitempty"`
}

// MarshalJSON expands TimeRange into top-level fromDate/toDate.
func (r ListAiTracesRequest) MarshalJSON() ([]byte, error) {
	type alias ListAiTracesRequest
	wire := struct {
		FromDate time.Time `json:"fromDate"`
		ToDate   time.Time `json:"toDate"`
		alias
	}{r.TimeRange.From, r.TimeRange.To, alias(r)}
	return jsonMarshalNoHTMLEscape(wire)
}

// ListAiTracesResponse mirrors PaginatedResponse[AiTraceStats].
type ListAiTracesResponse struct {
	Data       []AiTraceStats `json:"data"`
	Pagination Pagination     `json:"pagination"`
}

// ListAiTraces returns token/cost/duration stats grouped by AI trace name.
func (c *Client) ListAiTraces(ctx context.Context, projectID string, req ListAiTracesRequest) (*ListAiTracesResponse, error) {
	path := "/api/ai-traces/grouped?projectId=" + url.QueryEscape(projectID)
	var resp ListAiTracesResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
