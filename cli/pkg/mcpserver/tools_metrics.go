package mcpserver

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

type metricQueryIn struct {
	Name        string            `json:"name" jsonschema:"Metric name, e.g. system.cpu.utilization, system.memory.utilization, jvm.memory.used, go.memory.used, browser.web_vital.inp. An OTLP histogram is stored under its own name as a distribution; an explicit-bound one also keeps <name>.avg and <name>.count."`
	Aggregation string            `json:"aggregation,omitempty" jsonschema:"avg (default), sum, count, min, max, rate or last. A distribution metric (browser web vitals, app.* metrics, OTLP histograms) also takes p50, p75, p90, p95, p99 and good_ratio, and not last; the server answers 422 when the metric does not fit the aggregation. Request latency percentiles of endpoints still come from list_endpoints."`
	TagFilters  map[string]string `json:"tag_filters,omitempty" jsonschema:"Exact-match tag filters, e.g. {host: web-1}."`
	GroupBy     string            `json:"group_by,omitempty" jsonschema:"Tag key to split the series by; without it the series key is __all__."`
	Threshold   *float64          `json:"threshold,omitempty" jsonschema:"good_ratio only: the result is the fraction (0..1) of observations at or under this value. Rated web vitals have a default."`
}

type queryMetricsIn struct {
	projectIn
	timeRangeIn
	IntervalMinutes int             `json:"interval_minutes,omitempty" jsonschema:"Bucket size in minutes. Default 0 lets the server pick. The server may widen it so no series exceeds 2000 points; the effective value is returned as intervalMinutes. Read down the buckets for the step where a value jumps."`
	Queries         []metricQueryIn `json:"queries" jsonschema:"One or more metric queries to run over the same window."`
	WholeRange      bool            `json:"whole_range,omitempty" jsonschema:"Return one value per series for the whole window instead of a time series. Use it for a percentile over the window (what an alert over that window computes): a window's percentile cannot be put together from its buckets' percentiles."`
}

func (s *server) queryMetrics(ctx context.Context, req *mcp.CallToolRequest, in queryMetricsIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	tr, err := in.timeRangeIn.resolve()
	if err != nil {
		return nil, nil, err
	}
	if len(in.Queries) == 0 {
		return nil, nil, usageErrf("queries must contain at least one metric query")
	}
	if in.IntervalMinutes < 0 {
		return nil, nil, usageErrf("interval_minutes must be 0 (server default) or positive")
	}
	items := make([]client.MetricQueryItem, 0, len(in.Queries))
	for _, q := range in.Queries {
		if q.Name == "" {
			return nil, nil, usageErrf("queries[].name is required")
		}
		if q.Aggregation != "" && !slices.Contains(client.MetricAggregations, q.Aggregation) {
			return nil, nil, usageErrf("aggregation must be one of: %s", strings.Join(client.MetricAggregations, ", "))
		}
		if q.Threshold != nil && q.Aggregation != "good_ratio" {
			return nil, nil, usageErrf("threshold only applies to aggregation good_ratio")
		}
		items = append(items, client.MetricQueryItem{
			Name:        q.Name,
			Aggregation: cmp.Or(q.Aggregation, "avg"),
			TagFilters:  q.TagFilters,
			GroupBy:     q.GroupBy,
			Threshold:   q.Threshold,
		})
	}
	resp, err := s.client(req).QueryMetrics(ctx, projectID, client.QueryMetricsRequest{
		TimeRange:       tr,
		IntervalMinutes: in.IntervalMinutes,
		Queries:         items,
		WholeRange:      in.WholeRange,
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}
