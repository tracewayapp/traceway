package mcpserver

import (
	"context"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

type metricQueryIn struct {
	Name        string            `json:"name" jsonschema:"Metric name, e.g. system.cpu.utilization, mem.used_pcnt, go.gc_pause. Histogram metrics are stored as <name>.avg and <name>.count."`
	Aggregation string            `json:"aggregation,omitempty" jsonschema:"avg (default), sum, count, min, or max. p50/p95/p99 are rejected: the server has no quantile aggregation for metric points; latency percentiles come from list_endpoints."`
	TagFilters  map[string]string `json:"tag_filters,omitempty" jsonschema:"Exact-match tag filters, e.g. {host: web-1}."`
	GroupBy     string            `json:"group_by,omitempty" jsonschema:"Tag key to split the series by; without it the series key is all."`
}

type queryMetricsIn struct {
	projectIn
	timeRangeIn
	IntervalMinutes int             `json:"interval_minutes,omitempty" jsonschema:"Bucket size in minutes. Default 0 lets the server pick. Read down the buckets for the step where a value jumps."`
	Queries         []metricQueryIn `json:"queries" jsonschema:"One or more metric queries to run over the same window."`
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
		if q.Aggregation != "" && !slices.Contains(client.MetricAggregationsExact, q.Aggregation) {
			if slices.Contains(client.MetricAggregations, q.Aggregation) {
				return nil, nil, usageErrf("aggregation %s is not available: the server has no quantile aggregation for metric points and would silently return avg. Latency percentiles come from list_endpoints", q.Aggregation)
			}
			return nil, nil, usageErrf("aggregation must be one of: %s", strings.Join(client.MetricAggregationsExact, ", "))
		}
		items = append(items, client.MetricQueryItem{
			Name:        q.Name,
			Aggregation: pickStr(q.Aggregation, "avg"),
			TagFilters:  q.TagFilters,
			GroupBy:     q.GroupBy,
		})
	}
	resp, err := s.client(req).QueryMetrics(ctx, projectID, client.QueryMetricsRequest{
		TimeRange:       tr,
		IntervalMinutes: in.IntervalMinutes,
		Queries:         items,
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}
