package client

import (
	"github.com/tracewayapp/traceway/cli/pkg/access"

	"context"
	"net/http"
	"net/url"
	"time"
)

// MetricQueryItem is one query within a QueryMetricsRequest.
type MetricQueryItem = access.MetricQueryItem

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
type TimeSeriesPoint = access.TimeSeriesPoint

// MetricQueryResult is one query's results, optionally grouped by tag.
// The map key is the group label ("all" if no GroupBy was specified).
type MetricQueryResult = access.MetricSeries

// QueryMetricsResponse is the upstream MetricQueryResponse.
type QueryMetricsResponse = access.MetricResult

// QueryMetrics runs one or more metric queries against the project.
func (c *Client) QueryMetrics(ctx context.Context, projectID string, req QueryMetricsRequest) (*QueryMetricsResponse, error) {
	path := "/api/metrics/query?projectId=" + url.QueryEscape(projectID)
	var resp QueryMetricsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DiscoverMetricsResponse is the body of GET /api/metrics/discover.
type DiscoverMetricsResponse struct {
	Metrics []access.MetricInfo `json:"metrics"`
}

// DiscoverMetrics lists the metric names seen in the window with their tag
// keys and, when registered, type and unit.
func (c *Client) DiscoverMetrics(ctx context.Context, projectID string, tr TimeRange) (*DiscoverMetricsResponse, error) {
	path := "/api/metrics/discover?projectId=" + url.QueryEscape(projectID) +
		"&from=" + url.QueryEscape(tr.From.UTC().Format(time.RFC3339)) +
		"&to=" + url.QueryEscape(tr.To.UTC().Format(time.RFC3339))
	var resp DiscoverMetricsResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
