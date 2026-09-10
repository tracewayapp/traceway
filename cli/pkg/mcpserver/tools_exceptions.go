package mcpserver

import (
	"cmp"
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

type listExceptionsIn struct {
	projectIn
	sourceIn
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
	sources, err := access.Exceptions(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, failed, err := access.ListExceptions(ctx, sources, access.ExceptionQuery{
		ProjectID:       projectID,
		Window:          tr,
		Page:            page,
		Search:          in.Search,
		SearchType:      cmp.Or(in.SearchType, "text"),
		OrderBy:         cmp.Or(in.OrderBy, "lastSeen"),
		IncludeArchived: in.IncludeArchived,
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return withSourceWarnings(resp, failed)
}

type getExceptionIn struct {
	projectIn
	sourceIn
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
	if !exceptionHashPattern.MatchString(in.Hash) {
		return nil, nil, usageErrf("invalid hash %q: must be 16 lowercase hex characters, from list_exceptions or an /issues/<hash> dashboard URL", in.Hash)
	}
	sources, err := access.Exceptions(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, err := access.GetException(ctx, sources, access.ExceptionLookup{ProjectID: projectID, Hash: in.Hash, Page: page})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getExceptionOccurrenceIn struct {
	projectIn
	sourceIn
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
	sources, err := access.Exceptions(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, err := access.GetOccurrence(ctx, sources, access.Lookup{ProjectID: projectID, ID: in.ID, At: recordedAt})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type archiveIn struct {
	projectIn
	sourceIn
	Hashes []string `json:"hashes" jsonschema:"Exception group hashes (16 hex characters each) to act on."`
}

type archiveResult struct {
	Status string   `json:"status"`
	Hashes []string `json:"hashes"`
}

func (s *server) archiveExceptions(ctx context.Context, req *mcp.CallToolRequest, in archiveIn) (*mcp.CallToolResult, any, error) {
	return s.changeArchived(ctx, req, in, "archived", access.ExceptionAccess.ArchiveExceptions)
}

func (s *server) unarchiveExceptions(ctx context.Context, req *mcp.CallToolRequest, in archiveIn) (*mcp.CallToolResult, any, error) {
	return s.changeArchived(ctx, req, in, "unarchived", access.ExceptionAccess.UnarchiveExceptions)
}

// changeArchived applies an archive change on the source that owns the
// hashes: with several sources bound, the caller names it.
func (s *server) changeArchived(ctx context.Context, req *mcp.CallToolRequest, in archiveIn, status string, apply func(access.ExceptionAccess, context.Context, string, []string) error) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if len(in.Hashes) == 0 {
		return nil, nil, usageErrf("hashes must contain at least one exception hash")
	}
	sources, err := access.Exceptions(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	if len(sources) > 1 {
		return nil, nil, usageErrf("several sources answer exceptions; pass source to say which one holds these hashes")
	}
	if err := apply(sources[0], ctx, projectID, in.Hashes); err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, archiveResult{Status: status, Hashes: in.Hashes}, nil
}
