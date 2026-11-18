package client

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// MetricQueryItem is one query within a QueryMetricsRequest.
type MetricQueryItem struct {
	Name        string            `json:"name"`
	Aggregation string            `json:"aggregation,omitempty"`
	TagFilters  map[string]string `json:"tagFilters,omitempty"`
	GroupBy     string            `json:"groupBy,omitempty"`
}

// QueryMetricsRequest is the body for POST /api/metrics/query.
//
// Note: metrics uses `from`/`to` (NOT fromDate/toDate like the other endpoints)
// and has no pagination — results are time-bucketed via IntervalMinutes.
type QueryMetricsRequest struct {
	TimeRange       TimeRange         `json:"-"`
	IntervalMinutes int               `json:"intervalMinutes,omitempty"`
	Queries         []MetricQueryItem `json:"queries"`
}

// MarshalJSON expands TimeRange into top-level from/to (NOT fromDate/toDate).
func (r QueryMetricsRequest) MarshalJSON() ([]byte, error) {
	type alias QueryMetricsRequest
	wire := struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
		alias
	}{r.TimeRange.From, r.TimeRange.To, alias(r)}
	return jsonMarshalNoHTMLEscape(wire)
}

// TimeSeriesPoint is one data point in a metric query result.
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricQueryResult is one query's results, optionally grouped by tag.
// The map key is the group label ("__all__" if no GroupBy was specified).
type MetricQueryResult struct {
	Name   string                       `json:"name"`
	Unit   string                       `json:"unit"`
	Series map[string][]TimeSeriesPoint `json:"series"`
	// TruncatedGroups is true when a grouped query was cut to the first 200 groups.
	TruncatedGroups bool `json:"truncatedGroups,omitempty"`
}

// QueryMetricsResponse is the upstream MetricQueryResponse.
type QueryMetricsResponse struct {
	Results []MetricQueryResult `json:"results"`
	// IntervalMinutes is the effective bucket size: the server widens the
	// requested interval so no series exceeds 2000 points.
	IntervalMinutes int `json:"intervalMinutes,omitempty"`
}

// QueryMetrics runs one or more metric queries against the project.
func (c *Client) QueryMetrics(ctx context.Context, projectID string, req QueryMetricsRequest) (*QueryMetricsResponse, error) {
	path := "/api/metrics/query?projectId=" + url.QueryEscape(projectID)
	var resp QueryMetricsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
