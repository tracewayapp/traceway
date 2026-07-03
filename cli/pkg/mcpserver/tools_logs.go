package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

type queryLogsIn struct {
	projectIn
	timeRangeIn
	pageIn
	Search        string `json:"search,omitempty" jsonschema:"Free-text search."`
	SearchType    string `json:"search_type,omitempty" jsonschema:"What search matches: body (default) searches log bodies, attribute searches attribute values."`
	MinSeverity   uint8  `json:"min_severity,omitempty" jsonschema:"Minimum OTel severity NUMBER, never a name: 1 TRACE, 5 DEBUG, 9 INFO, 13 WARN, 17 ERROR, 21 FATAL. Use 17 for errors and worse."`
	ServiceName   string `json:"service_name,omitempty" jsonschema:"Filter to one service."`
	TraceID       string `json:"trace_id,omitempty" jsonschema:"Filter to one OpenTelemetry trace: returns the logs of a single request."`
	SortDirection string `json:"sort_direction,omitempty" jsonschema:"asc or desc (default) by timestamp."`
}

func (s *server) queryLogs(ctx context.Context, req *mcp.CallToolRequest, in queryLogsIn) (*mcp.CallToolResult, any, error) {
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
	if err := validateEnum("search_type", in.SearchType, client.LogsSearchTypes); err != nil {
		return nil, nil, err
	}
	if err := validateEnum("sort_direction", in.SortDirection, client.SortDirections); err != nil {
		return nil, nil, err
	}
	resp, err := s.client(req).QueryLogs(ctx, projectID, client.QueryLogsRequest{
		TimeRange:     tr,
		Pagination:    page,
		Search:        in.Search,
		SearchType:    pickStr(in.SearchType, "body"),
		MinSeverity:   in.MinSeverity,
		ServiceName:   in.ServiceName,
		TraceId:       in.TraceID,
		OrderBy:       "timestamp",
		SortDirection: pickStr(in.SortDirection, "desc"),
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}
