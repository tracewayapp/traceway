package client

import (
	"github.com/tracewayapp/traceway/cli/pkg/access"

	"context"
	"net/http"
	"net/url"
	"time"
)

// recordedAtBody is the body for the by-id detail endpoints that bound their
// ClickHouse lookup to a window around recordedAt. The CLI always sends it:
// without it the server falls back to scanning every daily partition.
type recordedAtBody struct {
	RecordedAt time.Time `json:"recordedAt"`
}

// startedAtBody is recordedAtBody's session equivalent — the sessions table is
// partitioned on started_at, so its detail endpoint takes startedAt instead.
type startedAtBody struct {
	StartedAt time.Time `json:"startedAt"`
}

// Span mirrors models.Span. Returned inside endpoint, task, and distributed
// trace detail responses.
type Span = access.Span

// Task mirrors models.Task — one run of a background task.
type Task = access.Task

// AiTrace mirrors models.AiTrace — one LLM call/operation.
type AiTrace = access.AITrace

// Session mirrors models.Session — one user session that can be replayed.
type Session = access.Session

// LinkedException is the exception/message summary attached to endpoint, task,
// and distributed-trace detail responses (backend EndpointExceptionInfo).
// RecordedAt is a preformatted RFC3339 string, matching the server.
type LinkedException = access.LinkedException

// LinkedMessage is a captured message (non-error exception) linked to an
// endpoint or task (backend EndpointMessageInfo).
type LinkedMessage = access.LinkedMessage

// TaskDetailResponse is the body of POST /api/tasks/:taskId.
type TaskDetailResponse = access.TaskDetail

// GetTask returns one task run plus its spans and linked exceptions/messages.
// recordedAt should be the task's recordedAt (from a notification, the URL's
// t= param, or a distributed-trace node) so the lookup prunes partitions.
func (c *Client) GetTask(ctx context.Context, projectID, id string, recordedAt time.Time) (*TaskDetailResponse, error) {
	path := "/api/tasks/" + url.PathEscape(id) + "?projectId=" + url.QueryEscape(projectID)
	var resp TaskDetailResponse
	if err := c.do(ctx, http.MethodPost, path, recordedAtBody{RecordedAt: recordedAt}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AiTraceDetailResponse is the body of POST /api/ai-traces/:traceId. The
// conversation blob is passed through verbatim (server stores it opaquely).
type AiTraceDetailResponse = access.AITraceDetail

// GetAiTrace returns one AI trace plus its stored conversation. recordedAt is
// the trace's recordedAt and is required for a fast (partition-pruned) lookup.
func (c *Client) GetAiTrace(ctx context.Context, projectID, id string, recordedAt time.Time) (*AiTraceDetailResponse, error) {
	path := "/api/ai-traces/" + url.PathEscape(id) + "?projectId=" + url.QueryEscape(projectID)
	var resp AiTraceDetailResponse
	if err := c.do(ctx, http.MethodPost, path, recordedAtBody{RecordedAt: recordedAt}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SessionLinkedException is one exception/message that fired during a session
// (backend SessionExceptionInfo). Unlike LinkedException it carries isMessage.
type SessionLinkedException = access.SessionLinkedException

// SessionDetailResponse is the body of POST /api/sessions/:sessionId.
type SessionDetailResponse = access.SessionDetail

// GetSession returns one session plus the exceptions that fired during it.
// startedAt (not recordedAt) bounds the lookup — the sessions table is
// partitioned on started_at. Use the session's startedAt, or the recordedAt of
// a linked exception occurrence, which falls inside the ±24h window.
func (c *Client) GetSession(ctx context.Context, projectID, id string, startedAt time.Time) (*SessionDetailResponse, error) {
	path := "/api/sessions/" + url.PathEscape(id) + "?projectId=" + url.QueryEscape(projectID)
	var resp SessionDetailResponse
	if err := c.do(ctx, http.MethodPost, path, startedAtBody{StartedAt: startedAt}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DistributedTraceNode is one resource (endpoint/task/ai-trace/exception) that
// participated in a distributed trace, scoped to its originating project.
type DistributedTraceNode = access.TraceNode

// DistributedTraceResponse is the body of POST /api/distributed-traces/:id —
// the full cross-service request timeline, the highest-value RCA view.
type DistributedTraceResponse = access.Trace

// GetDistributedTrace returns every node sharing a distributed trace id across
// all projects the user can see. The route resolves projects from the JWT, so
// no projectId query param is sent. recordedAt bounds the lookup to ±48h.
func (c *Client) GetDistributedTrace(ctx context.Context, id string, recordedAt time.Time) (*DistributedTraceResponse, error) {
	path := "/api/distributed-traces/" + url.PathEscape(id)
	var resp DistributedTraceResponse
	if err := c.do(ctx, http.MethodPost, path, recordedAtBody{RecordedAt: recordedAt}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
