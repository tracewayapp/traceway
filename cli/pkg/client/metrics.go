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
// The map key is the group label ("all" if no GroupBy was specified).
type MetricQueryResult struct {
	Name   string                       `json:"name"`
	Unit   string                       `json:"unit"`
	Series map[string][]TimeSeriesPoint `json:"series"`
}

// QueryMetricsResponse is the upstream MetricQueryResponse.
type QueryMetricsResponse struct {
	Results []MetricQueryResult `json:"results"`
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

// DiscoveredMetric matches the upstream models.DiscoveredMetric — one metric
// name seen in the window, with the tag keys observed on it. MetricType and
// Unit come from the project's metric registry and may be empty.
type DiscoveredMetric struct {
	Name       string   `json:"name"`
	TagKeys    []string `json:"tagKeys"`
	MetricType string   `json:"metricType,omitempty"`
	Unit       string   `json:"unit,omitempty"`
}

// DiscoverMetricsResponse is the body of GET /api/metrics/discover.
type DiscoverMetricsResponse struct {
	Metrics []DiscoveredMetric `json:"metrics"`
}

// DiscoverMetrics lists the metric names (with tag keys and registry metadata)
// that received points in the window. A zero TimeRange omits from/to and uses
// the server default of the last 7 days.
func (c *Client) DiscoverMetrics(ctx context.Context, projectID string, tr TimeRange) (*DiscoverMetricsResponse, error) {
	path := "/api/metrics/discover?projectId=" + url.QueryEscape(projectID)
	if !tr.From.IsZero() {
		path += "&from=" + url.QueryEscape(tr.From.Format(time.RFC3339)) +
			"&to=" + url.QueryEscape(tr.To.Format(time.RFC3339))
	}
	var resp DiscoverMetricsResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// MetricTagValuesResponse is the body of GET /api/metrics/discover/tags.
type MetricTagValuesResponse struct {
	Values []string `json:"values"`
}

// DiscoverMetricTagValues lists the values observed for one tag key of one
// metric. The server always scans the last 7 days for this endpoint.
func (c *Client) DiscoverMetricTagValues(ctx context.Context, projectID, name, key string) (*MetricTagValuesResponse, error) {
	path := "/api/metrics/discover/tags?projectId=" + url.QueryEscape(projectID) +
		"&name=" + url.QueryEscape(name) + "&key=" + url.QueryEscape(key)
	var resp MetricTagValuesResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
