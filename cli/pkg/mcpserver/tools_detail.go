package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/pkg/access"
)

type getTaskIn struct {
	projectIn
	sourceIn
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
	sources, err := access.Tasks(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, err := access.GetTask(ctx, sources, access.Lookup{ProjectID: projectID, ID: in.ID, At: recordedAt})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getAiTraceIn struct {
	projectIn
	sourceIn
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
	sources, err := access.AITraces(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, err := access.GetAITrace(ctx, sources, access.Lookup{ProjectID: projectID, ID: in.ID, At: recordedAt})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getSessionIn struct {
	projectIn
	sourceIn
	ID        string `json:"id" jsonschema:"The session's UUID, from a dashboard /sessions/<sessionId> URL or an occurrence's sessionId."`
	StartedAt string `json:"started_at" jsonschema:"REQUIRED for a fast lookup: the session's start time, RFC3339 (sessions are partitioned by start, not recording time). Approximate is fine (within 24h); a linked occurrence's recordedAt falls inside the window. Never pass the current time for an old session."`
}

func (s *server) getSession(ctx context.Context, req *mcp.CallToolRequest, in getSessionIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := validateUUID("id", in.ID); err != nil {
		return nil, nil, err
	}
	startedAt, err := parseTimestamp("started_at", in.StartedAt)
	if err != nil {
		return nil, nil, err
	}
	sources, err := access.Sessions(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, err := access.GetSession(ctx, sources, access.Lookup{ProjectID: projectID, ID: in.ID, At: startedAt})
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}

type getTraceIn struct {
	sourceIn
	ID         string `json:"id" jsonschema:"The distributed trace UUID, from an occurrence's or request's distributedTraceId."`
	RecordedAt string `json:"recorded_at" jsonschema:"REQUIRED for a fast lookup: any participating record's timestamp, RFC3339. Approximate is fine (within 48h for traces). Reuse the occurrence's recordedAt that gave you the trace id. Never pass the current time for an old trace."`
}

func (s *server) getTrace(ctx context.Context, req *mcp.CallToolRequest, in getTraceIn) (*mcp.CallToolResult, any, error) {
	if err := validateUUID("id", in.ID); err != nil {
		return nil, nil, err
	}
	recordedAt, err := parseTimestamp("recorded_at", in.RecordedAt)
	if err != nil {
		return nil, nil, err
	}
	sources, err := access.Traces(s.sources(req), in.Source)
	if err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	resp, err := access.GetTrace(ctx, sources, in.ID, recordedAt)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, resp, nil
}
