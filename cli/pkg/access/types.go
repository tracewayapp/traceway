package access

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Provenance is embedded in every record. Source is set by the fan-out when
// more than one source answered; Links point back at the provider's UI so an
// agent can cite the record; Raw carries provider detail the normalized
// fields cannot hold. All three stay empty on a single-source Traceway
// answer, so the wire shape is unchanged there.
type Provenance struct {
	Source string          `json:"source,omitempty"`
	Links  []Link          `json:"links,omitempty"`
	Raw    json.RawMessage `json:"raw,omitempty"`
}

func (p *Provenance) SetSource(name string) { p.Source = name }

type Link struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// Window is an inclusive time range.
type Window struct {
	From time.Time
	To   time.Time
}

// PageRequest is the request-side pagination control.
type PageRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// Pagination is the response-side pagination block.
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// Lookup addresses one record by id. At is the record's own timestamp
// (recordedAt, or startedAt for sessions); providers with partitioned
// storage use it to bound the lookup.
type Lookup struct {
	ProjectID string
	ID        string
	At        time.Time
}

// Logs

type LogQuery struct {
	ProjectID     string
	Window        Window
	Page          PageRequest
	Search        string
	SearchType    string
	MinSeverity   uint8
	ServiceName   string
	TraceID       string
	SortDirection string
}

type LogRecord struct {
	Provenance
	Id                 uuid.UUID         `json:"id"`
	Timestamp          time.Time         `json:"timestamp"`
	SeverityText       string            `json:"severityText"`
	SeverityNumber     uint8             `json:"severityNumber"`
	ServiceName        string            `json:"serviceName"`
	Body               string            `json:"body"`
	TraceId            string            `json:"traceId,omitempty"`
	SpanId             string            `json:"spanId,omitempty"`
	ResourceAttributes map[string]string `json:"resourceAttributes,omitempty"`
	ScopeName          string            `json:"scopeName,omitempty"`
	LogAttributes      map[string]string `json:"logAttributes,omitempty"`
}

type LogPage struct {
	Data       []LogRecord `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Exceptions

type ExceptionQuery struct {
	ProjectID       string
	Window          Window
	Page            PageRequest
	Search          string
	SearchType      string
	OrderBy         string
	IncludeArchived bool
}

type ExceptionLookup struct {
	ProjectID string
	Hash      string
	Page      PageRequest
}

type ExceptionGroup struct {
	Provenance
	ExceptionHash string                `json:"exceptionHash"`
	StackTrace    string                `json:"stackTrace"`
	FirstSeen     time.Time             `json:"firstSeen"`
	LastSeen      time.Time             `json:"lastSeen"`
	Count         uint64                `json:"count"`
	HourlyTrend   []ExceptionTrendPoint `json:"hourlyTrend,omitempty"`
}

type ExceptionTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Count     uint64    `json:"count"`
}

type ExceptionPage struct {
	Data       []ExceptionGroup `json:"data"`
	Pagination Pagination       `json:"pagination"`
}

// Occurrence is one recorded instance of an exception group.
type Occurrence struct {
	Provenance
	Id                 uuid.UUID         `json:"id"`
	ExceptionHash      string            `json:"exceptionHash"`
	StackTrace         string            `json:"stackTrace"`
	RecordedAt         time.Time         `json:"recordedAt"`
	TraceId            *uuid.UUID        `json:"traceId,omitempty"`
	TraceType          string            `json:"traceType,omitempty"`
	ServerName         string            `json:"serverName,omitempty"`
	AppVersion         string            `json:"appVersion,omitempty"`
	IsMessage          bool              `json:"isMessage,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty"`
	SessionId          *uuid.UUID        `json:"sessionId,omitempty"`
}

type ExceptionDetail struct {
	Group       *ExceptionGroup `json:"group"`
	Occurrences []Occurrence    `json:"occurrences"`
	Pagination  Pagination      `json:"pagination"`
}

type OccurrenceDetail struct {
	Exception        *Occurrence     `json:"exception"`
	SessionId        *uuid.UUID      `json:"sessionId,omitempty"`
	SessionRecording json.RawMessage `json:"sessionRecording,omitempty"`
}

// Endpoints

type EndpointQuery struct {
	ProjectID     string
	Window        Window
	Page          PageRequest
	Search        string
	OrderBy       string
	SortDirection string
}

type EndpointStats struct {
	Provenance
	Endpoint     string        `json:"endpoint"`
	Count        uint64        `json:"count"`
	P50Duration  time.Duration `json:"p50Duration"`
	P95Duration  time.Duration `json:"p95Duration"`
	P99Duration  time.Duration `json:"p99Duration"`
	AvgDuration  time.Duration `json:"avgDuration"`
	LastSeen     time.Time     `json:"lastSeen"`
	Impact       float64       `json:"impact"`
	ImpactReason string        `json:"impactReason"`
}

type EndpointPage struct {
	Data       []EndpointStats `json:"data"`
	Pagination Pagination      `json:"pagination"`
}

// Request is one HTTP request (transaction).
type Request struct {
	Provenance
	Id                 uuid.UUID         `json:"id"`
	ProjectId          uuid.UUID         `json:"projectId"`
	Endpoint           string            `json:"endpoint"`
	Duration           time.Duration     `json:"duration"`
	RecordedAt         time.Time         `json:"recordedAt"`
	StatusCode         int16             `json:"statusCode"`
	BodySize           int32             `json:"bodySize"`
	ClientIP           string            `json:"clientIP"`
	Attributes         map[string]string `json:"attributes"`
	AppVersion         string            `json:"appVersion"`
	ServerName         string            `json:"serverName"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty"`
	SpanId             *uuid.UUID        `json:"spanId,omitempty"`
	IsStream           bool              `json:"isStream"`
	IsRoot             bool              `json:"isRoot"`
}

type RequestDetail struct {
	Endpoint  *Request         `json:"endpoint"`
	Spans     []Span           `json:"spans"`
	HasSpans  bool             `json:"hasSpans"`
	Exception *LinkedException `json:"exception,omitempty"`
	Messages  []LinkedMessage  `json:"messages"`
}

type EndpointChartQuery struct {
	ProjectID       string
	Window          Window
	MetricType      string
	IntervalMinutes int
}

type EndpointChartPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Endpoint  string    `json:"endpoint"`
	Value     float64   `json:"value"`
}

// EndpointChart is the top endpoints ranked by the chart metric, plus an
// Other bucket, each as a time series in milliseconds.
type EndpointChart struct {
	Endpoints []string             `json:"endpoints"`
	Series    []EndpointChartPoint `json:"series"`
}

// SlowEndpoint is an operator's accepted latency allowance for an endpoint;
// OffsetMs 0 means it was never marked.
type SlowEndpoint struct {
	OffsetMs uint32 `json:"offsetMs"`
	Reason   string `json:"reason"`
}

// Metrics

type MetricQueryItem struct {
	Name        string            `json:"name"`
	Aggregation string            `json:"aggregation,omitempty"`
	TagFilters  map[string]string `json:"tagFilters,omitempty"`
	GroupBy     string            `json:"groupBy,omitempty"`
}

type MetricQuery struct {
	ProjectID       string
	Window          Window
	IntervalMinutes int
	Queries         []MetricQueryItem
}

type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricSeries is one query's answer, grouped by tag value ("all" when the
// query had no GroupBy).
type MetricSeries struct {
	Provenance
	Name            string                       `json:"name"`
	Unit            string                       `json:"unit"`
	Series          map[string][]TimeSeriesPoint `json:"series"`
	TruncatedGroups bool                         `json:"truncatedGroups,omitempty"`
}

type MetricResult struct {
	Results         []MetricSeries `json:"results"`
	IntervalMinutes int            `json:"intervalMinutes,omitempty"`
}

type MetricDiscovery struct {
	ProjectID string
	Window    Window
}

type MetricInfo struct {
	Provenance
	Name       string   `json:"name"`
	TagKeys    []string `json:"tagKeys"`
	MetricType string   `json:"metricType,omitempty"`
	Unit       string   `json:"unit,omitempty"`
}

// Detail records shared by endpoints, tasks, sessions and traces

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

// LinkedException is the exception summary attached to a request, task or
// trace node. RecordedAt is preformatted RFC3339, as the server sends it.
type LinkedException struct {
	ExceptionHash string `json:"exceptionHash"`
	StackTrace    string `json:"stackTrace"`
	RecordedAt    string `json:"recordedAt"`
}

type LinkedMessage struct {
	Id            uuid.UUID         `json:"id"`
	ExceptionHash string            `json:"exceptionHash"`
	StackTrace    string            `json:"stackTrace"`
	RecordedAt    string            `json:"recordedAt"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type Task struct {
	Provenance
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

type TaskDetail struct {
	Task      *Task            `json:"task"`
	Spans     []Span           `json:"spans"`
	HasSpans  bool             `json:"hasSpans"`
	Exception *LinkedException `json:"exception,omitempty"`
	Messages  []LinkedMessage  `json:"messages"`
}

type AITrace struct {
	Provenance
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
	ConversationId     string            `json:"conversationId"`
	ToolCallCount      int64             `json:"toolCallCount"`
	ToolNames          []string          `json:"toolNames"`
	Flagged            bool              `json:"flagged"`
	FlaggedTerms       []string          `json:"flaggedTerms"`
}

// AITraceDetail carries the trace plus its stored conversation, passed
// through verbatim.
type AITraceDetail struct {
	AiTrace      *AITrace        `json:"aiTrace"`
	Conversation json.RawMessage `json:"conversation,omitempty"`
}

type Session struct {
	Provenance
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

type SessionLinkedException struct {
	Id            uuid.UUID `json:"id"`
	ExceptionHash string    `json:"exceptionHash"`
	StackTrace    string    `json:"stackTrace"`
	RecordedAt    string    `json:"recordedAt"`
	IsMessage     bool      `json:"isMessage"`
}

type SessionDetail struct {
	Session    *Session                 `json:"session"`
	Exceptions []SessionLinkedException `json:"exceptions"`
}

// TraceNode is one resource that took part in a distributed trace.
type TraceNode struct {
	ProjectId   uuid.UUID        `json:"projectId"`
	ProjectName string           `json:"projectName"`
	TraceType   string           `json:"traceType"`
	Endpoint    *Request         `json:"endpoint,omitempty"`
	Task        *Task            `json:"task,omitempty"`
	AiTrace     *AITrace         `json:"aiTrace,omitempty"`
	Spans       []Span           `json:"spans"`
	Exception   *LinkedException `json:"exception,omitempty"`
}

type Trace struct {
	Provenance
	DistributedTraceId string      `json:"distributedTraceId"`
	Nodes              []TraceNode `json:"nodes"`
}
