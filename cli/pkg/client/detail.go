package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
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
type Span struct {
	Id           uuid.UUID         `json:"id"`
	TraceId      uuid.UUID         `json:"traceId"`
	ProjectId    uuid.UUID         `json:"projectId"`
	Name         string            `json:"name"`
	StartTime    time.Time         `json:"startTime"`
	Duration     time.Duration     `json:"duration"`
	RecordedAt   time.Time         `json:"recordedAt"`
	ParentSpanId *uuid.UUID        `json:"parentSpanId,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

// Task mirrors models.Task — one run of a background task.
type Task struct {
	Id                 uuid.UUID         `json:"id"`
	ProjectId          uuid.UUID         `json:"projectId"`
	TaskName           string            `json:"taskName"`
	Duration           time.Duration     `json:"duration"`
	RecordedAt         time.Time         `json:"recordedAt"`
	ClientIP           string            `json:"clientIP"`
	Attributes         map[string]string `json:"attributes"`
	AppVersion         string            `json:"appVersion"`
	ServerName         string            `json:"serverName"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty"`
	SpanId             *uuid.UUID        `json:"spanId,omitempty"`
	IsRoot             bool              `json:"isRoot"`
}

// AiTrace mirrors models.AiTrace — one LLM call/operation.
type AiTrace struct {
	Id                 uuid.UUID         `json:"id"`
	ProjectId          uuid.UUID         `json:"projectId"`
	RecordedAt         time.Time         `json:"recordedAt"`
	Duration           time.Duration     `json:"duration"`
	StatusCode         uint8             `json:"statusCode"`
	Model              string            `json:"model"`
	ResponseModel      string            `json:"responseModel"`
	Provider           string            `json:"provider"`
	Operation          string            `json:"operation"`
	InputTokens        int64             `json:"inputTokens"`
	OutputTokens       int64             `json:"outputTokens"`
	TotalTokens        int64             `json:"totalTokens"`
	CachedTokens       int64             `json:"cachedTokens"`
	ReasoningTokens    int64             `json:"reasoningTokens"`
	InputCost          float64           `json:"inputCost"`
	OutputCost         float64           `json:"outputCost"`
	TotalCost          float64           `json:"totalCost"`
	TraceName          string            `json:"traceName"`
	UserId             string            `json:"userId"`
	FinishReason       string            `json:"finishReason"`
	ServerName         string            `json:"serverName"`
	AppVersion         string            `json:"appVersion"`
	StorageKey         string            `json:"storageKey"`
	Attributes         map[string]string `json:"attributes"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty"`
	IsRoot             bool              `json:"isRoot"`
}

// Session mirrors models.Session — one user session that can be replayed.
type Session struct {
	Id                 uuid.UUID         `json:"id"`
	ProjectId          uuid.UUID         `json:"projectId"`
	StartedAt          time.Time         `json:"startedAt"`
	EndedAt            *time.Time        `json:"endedAt,omitempty"`
	Duration           int64             `json:"duration"`
	ClientIP           string            `json:"clientIP"`
	Attributes         map[string]string `json:"attributes"`
	AppVersion         string            `json:"appVersion"`
	ServerName         string            `json:"serverName"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty"`
}

// LinkedException is the exception/message summary attached to endpoint, task,
// and distributed-trace detail responses (backend EndpointExceptionInfo).
// RecordedAt is a preformatted RFC3339 string, matching the server.
type LinkedException struct {
	ExceptionHash string `json:"exceptionHash"`
	StackTrace    string `json:"stackTrace"`
	RecordedAt    string `json:"recordedAt"`
}

// LinkedMessage is a captured message (non-error exception) linked to an
// endpoint or task (backend EndpointMessageInfo).
type LinkedMessage struct {
	Id            uuid.UUID         `json:"id"`
	ExceptionHash string            `json:"exceptionHash"`
	StackTrace    string            `json:"stackTrace"`
	RecordedAt    string            `json:"recordedAt"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// TaskDetailResponse is the body of POST /api/tasks/:taskId.
type TaskDetailResponse struct {
	Task      *Task            `json:"task"`
	Spans     []Span           `json:"spans"`
	HasSpans  bool             `json:"hasSpans"`
	Exception *LinkedException `json:"exception,omitempty"`
	Messages  []LinkedMessage  `json:"messages"`
}

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
type AiTraceDetailResponse struct {
	AiTrace      *AiTrace        `json:"aiTrace"`
	Conversation json.RawMessage `json:"conversation,omitempty"`
}

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
type SessionLinkedException struct {
	Id            uuid.UUID `json:"id"`
	ExceptionHash string    `json:"exceptionHash"`
	StackTrace    string    `json:"stackTrace"`
	RecordedAt    string    `json:"recordedAt"`
	IsMessage     bool      `json:"isMessage"`
}

// SessionDetailResponse is the body of POST /api/sessions/:sessionId.
type SessionDetailResponse struct {
	Session    *Session                 `json:"session"`
	Exceptions []SessionLinkedException `json:"exceptions"`
}

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
type DistributedTraceNode struct {
	ProjectId   uuid.UUID        `json:"projectId"`
	ProjectName string           `json:"projectName"`
	TraceType   string           `json:"traceType"`
	Endpoint    *Endpoint        `json:"endpoint,omitempty"`
	Task        *Task            `json:"task,omitempty"`
	AiTrace     *AiTrace         `json:"aiTrace,omitempty"`
	Spans       []Span           `json:"spans"`
	Exception   *LinkedException `json:"exception,omitempty"`
}

// DistributedTraceResponse is the body of POST /api/distributed-traces/:id —
// the full cross-service request timeline, the highest-value RCA view.
type DistributedTraceResponse struct {
	DistributedTraceId string                 `json:"distributedTraceId"`
	Nodes              []DistributedTraceNode `json:"nodes"`
}

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
