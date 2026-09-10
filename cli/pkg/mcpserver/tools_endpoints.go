package mcpserver

import (
	"cmp"
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

type listEndpointsIn struct {
	projectIn
	sourceIn
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
	sources, err := access.Endpoints(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, failed, err := access.ListEndpoints(ctx, sources, access.EndpointQuery{
		ProjectID:     projectID,
		Window:        tr,
		Page:          page,
		Search:        in.Search,
		OrderBy:       cmp.Or(in.OrderBy, "impact"),
		SortDirection: cmp.Or(in.SortDirection, "desc"),
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return withSourceWarnings(resp, failed)
}

type getEndpointRequestIn struct {
	projectIn
	sourceIn
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
	sources, err := access.Endpoints(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, err := access.GetRequest(ctx, sources, access.Lookup{ProjectID: projectID, ID: in.ID, At: recordedAt})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type endpointsChartIn struct {
	projectIn
	sourceIn
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
	source, err := s.singleEndpointsSource(req, in.Source, "endpoints_chart")
	if err != nil {
		return nil, nil, err
	}
	resp, err := source.EndpointChart(ctx, access.EndpointChartQuery{
		ProjectID:       projectID,
		Window:          tr,
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
	sourceIn
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
	source, err := s.singleEndpointsSource(req, in.Source, "get_slow_endpoint_config")
	if err != nil {
		return nil, nil, err
	}
	resp, err := source.SlowEndpoint(ctx, projectID, in.Endpoint)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

// singleEndpointsSource picks the one source for the endpoint views that
// have no merge (a ranked chart, an operator setting): the first bound
// source by default, or the named one.
func (s *server) singleEndpointsSource(req *mcp.CallToolRequest, name, tool string) (access.EndpointsAccess, error) {
	sources, err := access.Endpoints(s.sources(req), name)
	if err != nil {
		return nil, usageErrf("%v", err)
	}
	if len(sources) > 1 && name == "" {
		return nil, usageErrf("several sources answer endpoints; %s reads one of them, pass source to choose (default: %s)", tool, sources[0].Name())
	}
	return sources[0], nil
}
