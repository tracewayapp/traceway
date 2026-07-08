package mcpserver

import (
	"cmp"
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

type listEndpointsIn struct {
	projectIn
	timeRangeIn
	pageIn
	Search        string `json:"search,omitempty" jsonschema:"Free-text filter for endpoint names, e.g. GET /api/users/:id. URL-decode names taken from dashboard URLs first."`
	OrderBy       string `json:"order_by,omitempty" jsonschema:"Sort field: impact (default), count, p95, or lastSeen."`
	SortDirection string `json:"sort_direction,omitempty" jsonschema:"asc or desc (default)."`
}

func (s *server) listEndpoints(ctx context.Context, req *mcp.CallToolRequest, in listEndpointsIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	tr, err := in.timeRangeIn.resolve()
	if err != nil {
		return nil, nil, err
	}
	page, err := in.pageIn.resolve()
	if err != nil {
		return nil, nil, err
	}
	if err := validateEnum("order_by", in.OrderBy, client.EndpointsOrderByValues); err != nil {
		return nil, nil, err
	}
	if err := validateEnum("sort_direction", in.SortDirection, client.SortDirections); err != nil {
		return nil, nil, err
	}
	resp, err := s.client(req).ListEndpoints(ctx, projectID, client.ListEndpointsRequest{
		TimeRange:     tr,
		Pagination:    page,
		Search:        in.Search,
		OrderBy:       cmp.Or(in.OrderBy, "impact"),
		SortDirection: cmp.Or(in.SortDirection, "desc"),
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getEndpointRequestIn struct {
	projectIn
	ID         string `json:"id" jsonschema:"The request's UUID, from a dashboard /endpoints/<endpoint>/<endpointId> URL or a distributed trace node."`
	RecordedAt string `json:"recorded_at" jsonschema:"REQUIRED for a fast lookup: the record's timestamp, RFC3339. Approximate is fine (within 24h). Recover it from the dashboard URL's ?t= param or the trace node; see traceway://knowledge/timestamps. Never pass the current time for an old record."`
}

func (s *server) getEndpointRequest(ctx context.Context, req *mcp.CallToolRequest, in getEndpointRequestIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := validateUUID("id", in.ID); err != nil {
		return nil, nil, err
	}
	recordedAt, err := parseTimestamp("recorded_at", in.RecordedAt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := s.client(req).GetEndpoint(ctx, projectID, in.ID, recordedAt)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type endpointsChartIn struct {
	projectIn
	timeRangeIn
	MetricType      string `json:"metric_type,omitempty" jsonschema:"Metric to chart: total_time, p50, p95 (default), or p99."`
	IntervalMinutes int    `json:"interval_minutes,omitempty" jsonschema:"Bucket size in minutes. Default is the server's (5). Use a coarse interval over a wide window first, then re-run the suspect span with a finer one."`
}

func (s *server) endpointsChart(ctx context.Context, req *mcp.CallToolRequest, in endpointsChartIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	tr, err := in.timeRangeIn.resolve()
	if err != nil {
		return nil, nil, err
	}
	if err := validateEnum("metric_type", in.MetricType, client.EndpointChartMetricTypes); err != nil {
		return nil, nil, err
	}
	if in.IntervalMinutes < 0 {
		return nil, nil, usageErrf("interval_minutes must be 0 (server default) or positive")
	}
	resp, err := s.client(req).GetEndpointChart(ctx, projectID, client.EndpointChartRequest{
		TimeRange:       tr,
		MetricType:      cmp.Or(in.MetricType, "p95"),
		IntervalMinutes: in.IntervalMinutes,
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getSlowEndpointConfigIn struct {
	projectIn
	Endpoint string `json:"endpoint" jsonschema:"The endpoint name exactly as listed, e.g. GET /api/reports/export. URL-decode names taken from dashboard URLs first."`
}

func (s *server) getSlowEndpointConfig(ctx context.Context, req *mcp.CallToolRequest, in getSlowEndpointConfigIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if in.Endpoint == "" {
		return nil, nil, usageErrf("endpoint is required: the endpoint name exactly as returned by list_endpoints")
	}
	resp, err := s.client(req).GetSlowEndpoint(ctx, projectID, in.Endpoint)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}
