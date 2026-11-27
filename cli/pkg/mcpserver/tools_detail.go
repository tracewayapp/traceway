package mcpserver

import (
	"cmp"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

type getTaskIn struct {
	projectIn
	ID         string `json:"id" jsonschema:"The task run's UUID, from a dashboard /tasks/<task>/<taskId> URL or a distributed trace node."`
	RecordedAt string `json:"recorded_at" jsonschema:"REQUIRED for a fast lookup: the record's timestamp, RFC3339. Approximate is fine (within 24h). Recover it from the dashboard URL's ?t= param or the trace node; see traceway://knowledge/timestamps. Never pass the current time for an old record."`
}

func (s *server) getTask(ctx context.Context, req *mcp.CallToolRequest, in getTaskIn) (*mcp.CallToolResult, any, error) {
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
	resp, err := s.client(req).GetTask(ctx, projectID, in.ID, recordedAt)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getAiTraceIn struct {
	projectIn
	ID         string `json:"id" jsonschema:"The AI trace's UUID, from a dashboard /ai-traces/<traceName>/<traceId> URL or a distributed trace node."`
	RecordedAt string `json:"recorded_at" jsonschema:"REQUIRED for a fast lookup: the record's timestamp, RFC3339. Approximate is fine (within 24h). Recover it from the dashboard URL's ?t= param or the trace node; see traceway://knowledge/timestamps. Never pass the current time for an old record."`
}

func (s *server) getAiTrace(ctx context.Context, req *mcp.CallToolRequest, in getAiTraceIn) (*mcp.CallToolResult, any, error) {
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
	resp, err := s.client(req).GetAiTrace(ctx, projectID, in.ID, recordedAt)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getSessionIn struct {
	projectIn
	ID        string `json:"id" jsonschema:"The session's UUID, from list_sessions, a dashboard /sessions/<sessionId> URL or an occurrence's sessionId."`
	StartedAt string `json:"started_at,omitempty" jsonschema:"The session's start time, RFC3339 (sessions are partitioned by start, not recording time): a list_sessions row's startedAt, or a linked occurrence's recordedAt (within 24h is fine). When omitted, the start is looked up among the sessions of the last 30 days first. Never pass the current time for an old session."`
}

func (s *server) getSession(ctx context.Context, req *mcp.CallToolRequest, in getSessionIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := validateUUID("id", in.ID); err != nil {
		return nil, nil, err
	}
	c := s.client(req)
	var startedAt time.Time
	if strings.TrimSpace(in.StartedAt) == "" {
		startedAt, err = c.FindSessionStart(ctx, projectID, in.ID)
		if errors.Is(err, client.ErrSessionNotFound) {
			return nil, nil, usageErrf("no session %s started in the last 30 days: pass started_at for an older session, or check project_id", in.ID)
		}
		if err != nil {
			return nil, nil, s.apiErr(err)
		}
	} else if startedAt, err = parseTimestamp("started_at", in.StartedAt); err != nil {
		return nil, nil, err
	}
	resp, err := c.GetSession(ctx, projectID, in.ID, startedAt)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type listSessionsIn struct {
	projectIn
	timeRangeIn
	pageIn
	Search        string `json:"search,omitempty" jsonschema:"Match the session id, the client IP or an attribute value such as a user id or email."`
	OrderBy       string `json:"order_by,omitempty" jsonschema:"Sort field: startedAt (default) or duration."`
	SortDirection string `json:"sort_direction,omitempty" jsonschema:"asc or desc (default)."`
}

func (s *server) listSessions(ctx context.Context, req *mcp.CallToolRequest, in listSessionsIn) (*mcp.CallToolResult, any, error) {
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
	if err := validateEnum("order_by", in.OrderBy, client.SessionsOrderByValues); err != nil {
		return nil, nil, err
	}
	if err := validateEnum("sort_direction", in.SortDirection, client.SortDirections); err != nil {
		return nil, nil, err
	}
	resp, err := s.client(req).ListSessions(ctx, projectID, client.ListSessionsRequest{
		TimeRange:     tr,
		Pagination:    page,
		Search:        strings.TrimSpace(in.Search),
		OrderBy:       cmp.Or(in.OrderBy, "startedAt"),
		SortDirection: cmp.Or(in.SortDirection, "desc"),
	})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getTraceIn struct {
	ID         string `json:"id" jsonschema:"The trace id: 32 hex characters, from an occurrence's or request's traceId. A dashed UUID is accepted too."`
	RecordedAt string `json:"recorded_at" jsonschema:"REQUIRED for a fast lookup: any participating record's timestamp, RFC3339. Approximate is fine (within 48h for traces). Reuse the occurrence's recordedAt that gave you the trace id. Never pass the current time for an old trace."`
}

func (s *server) getTrace(ctx context.Context, req *mcp.CallToolRequest, in getTraceIn) (*mcp.CallToolResult, any, error) {
	if err := validateTraceID("id", in.ID); err != nil {
		return nil, nil, err
	}
	recordedAt, err := parseTimestamp("recorded_at", in.RecordedAt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := s.client(req).GetDistributedTrace(ctx, in.ID, recordedAt)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}
