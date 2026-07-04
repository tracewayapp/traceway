package mcpserver

import (
	"cmp"
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

type listExceptionsIn struct {
	projectIn
	timeRangeIn
	pageIn
	Search          string `json:"search,omitempty" jsonschema:"Free-text filter over stack traces."`
	SearchType      string `json:"search_type,omitempty" jsonschema:"How to interpret search: text (default) or regex."`
	OrderBy         string `json:"order_by,omitempty" jsonschema:"Sort field: lastSeen (default), firstSeen, or count."`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"Include archived exception groups."`
}

func (s *server) listExceptions(ctx context.Context, req *mcp.CallToolRequest, in listExceptionsIn) (*mcp.CallToolResult, any, error) {
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
	if err := validateEnum("search_type", in.SearchType, client.ExceptionsSearchTypes); err != nil {
		return nil, nil, err
	}
	if err := validateEnum("order_by", in.OrderBy, client.ExceptionsOrderByValues); err != nil {
		return nil, nil, err
	}
	resp, err := s.client(req).ListExceptions(ctx, projectID, client.ListExceptionsRequest{
		TimeRange:       tr,
		Pagination:      page,
		Search:          in.Search,
		SearchType:      cmp.Or(in.SearchType, "text"),
		OrderBy:         cmp.Or(in.OrderBy, "lastSeen"),
		IncludeArchived: in.IncludeArchived,
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getExceptionIn struct {
	projectIn
	pageIn
	Hash string `json:"hash" jsonschema:"The exception group's hash: 16 hex characters, from list_exceptions or the /issues/<hash> dashboard URL path."`
}

func (s *server) getException(ctx context.Context, req *mcp.CallToolRequest, in getExceptionIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	page, err := in.pageIn.resolve()
	if err != nil {
		return nil, nil, err
	}
	if err := validateHash("hash", in.Hash); err != nil {
		return nil, nil, err
	}
	resp, err := s.client(req).GetException(ctx, projectID, in.Hash, page)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getExceptionOccurrenceIn struct {
	projectIn
	ID         string `json:"id" jsonschema:"The occurrence's UUID, from a dashboard URL path, a notification's Exception ID, or a get_exception occurrence."`
	RecordedAt string `json:"recorded_at" jsonschema:"REQUIRED for a fast lookup: the record's timestamp, RFC3339. Approximate is fine (within 24h). Recover it from the dashboard URL's ?t= param, an occurrence's recordedAt, or a notification's Occurred at; see traceway://knowledge/timestamps. Never pass the current time for an old record."`
}

func (s *server) getExceptionOccurrence(ctx context.Context, req *mcp.CallToolRequest, in getExceptionOccurrenceIn) (*mcp.CallToolResult, any, error) {
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
	resp, err := s.client(req).GetExceptionById(ctx, projectID, in.ID, recordedAt)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type archiveIn struct {
	projectIn
	Hashes []string `json:"hashes" jsonschema:"Exception group hashes (16 hex characters each) to act on."`
}

type archiveResult struct {
	Status string   `json:"status"`
	Hashes []string `json:"hashes"`
}

func (s *server) archiveExceptions(ctx context.Context, req *mcp.CallToolRequest, in archiveIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if len(in.Hashes) == 0 {
		return nil, nil, usageErrf("hashes must contain at least one exception hash")
	}
	if err := s.client(req).ArchiveExceptions(ctx, projectID, in.Hashes); err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, archiveResult{Status: "archived", Hashes: in.Hashes}, nil
}

func (s *server) unarchiveExceptions(ctx context.Context, req *mcp.CallToolRequest, in archiveIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if len(in.Hashes) == 0 {
		return nil, nil, usageErrf("hashes must contain at least one exception hash")
	}
	if err := s.client(req).UnarchiveExceptions(ctx, projectID, in.Hashes); err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, archiveResult{Status: "unarchived", Hashes: in.Hashes}, nil
}
