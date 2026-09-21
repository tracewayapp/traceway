package clientmodels

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type ClientExceptionStackTrace struct {
	TraceId            *string           `json:"traceId"`
	IsTask             bool              `json:"isTask"`
	StackTrace         string            `json:"stackTrace"`
	RecordedAt         time.Time         `json:"recordedAt"`
	Attributes         map[string]string `json:"attributes"`
	IsMessage          bool              `json:"isMessage"`
	SessionRecordingId *string           `json:"sessionRecordingId"`
	SessionId          *string           `json:"sessionId"`
	DebugIds           map[string]string `json:"debugIds"`
}

func (c *ClientExceptionStackTrace) ToExceptionStackTrace(exceptionHash, appVersion, serverName string) models.ExceptionStackTrace {
	traceType := "endpoint"
	if c.IsTask {
		traceType = "task"
	}

	// In the native protocol a "trace" is one endpoint or task run, and traceId names it. That run is stored as the root
	// span of a trace, so the exception sits on that span.
	var traceId, spanId string
	if c.TraceId != nil {
		if parsed, err := uuid.Parse(*c.TraceId); err == nil && parsed != uuid.Nil {
			spanId = hexId(parsed)
			traceId = spanId
		}
	}

	var sessionId *uuid.UUID
	if c.SessionId != nil {
		if parsed, err := uuid.Parse(*c.SessionId); err == nil {
			sessionId = &parsed
		}
	}

	return models.ExceptionStackTrace{
		ExceptionHash: exceptionHash,
		TraceId:       traceId,
		SpanId:        spanId,
		TraceType:     traceType,
		StackTrace:    c.StackTrace,
		RecordedAt:    c.RecordedAt,
		Attributes:    c.Attributes,
		IsMessage:     c.IsMessage,
		AppVersion:    appVersion,
		ServerName:    serverName,
		SessionId:     sessionId,
	}
}

type ClientMetricRecord struct {
	Name       string            `json:"name"`
	Value      float64           `json:"value"`
	RecordedAt time.Time         `json:"recordedAt"`
	Tags       map[string]string `json:"tags,omitempty"`
}

func (c *ClientMetricRecord) ToMetricPoint(serverName string) models.MetricPoint {
	tags := make(map[string]string, len(c.Tags)+1)
	for k, v := range c.Tags {
		tags[k] = v
	}
	if serverName != "" {
		tags["server_name"] = serverName
	}
	return models.MetricPoint{
		Name:       c.Name,
		Value:      c.Value,
		Tags:       tags,
		RecordedAt: c.RecordedAt,
	}
}

type ClientTrace struct {
	Id         string            `json:"id"`
	Endpoint   string            `json:"endpoint"`
	Duration   time.Duration     `json:"duration"`
	RecordedAt time.Time         `json:"recordedAt"`
	StatusCode int               `json:"statusCode"`
	BodySize   int               `json:"bodySize"`
	ClientIP   string            `json:"clientIP"`
	Attributes map[string]string `json:"attributes"`
	Spans      []*ClientSpan     `json:"spans"`
	IsTask     bool              `json:"isTask"`
}

const streamAttributeKey = "traceway.is_stream"

// ParsedId returns the trace ID as uuid.UUID
func (c *ClientTrace) ParsedId() uuid.UUID {
	if parsed, err := uuid.Parse(c.Id); err == nil && parsed != uuid.Nil {
		return parsed
	}
	c.Id = uuid.NewString()
	return uuid.MustParse(c.Id)
}

func hexId(id uuid.UUID) string { return hex.EncodeToString(id[:]) }

// SpanId is the id of the run itself: a native trace is stored as the root span of a trace.
func (c *ClientTrace) SpanId() string { return hexId(c.ParsedId()) }

func (c *ClientTrace) TraceId() string { return c.SpanId() }

func (c *ClientTrace) ToEndpoint(appVersion, serverName string) models.Endpoint {
	return models.Endpoint{
		Id:         c.ParsedId(),
		Endpoint:   c.Endpoint,
		Duration:   c.Duration,
		RecordedAt: c.RecordedAt,
		StatusCode: int16(c.StatusCode),
		BodySize:   int32(c.BodySize),
		ClientIP:   c.ClientIP,
		Attributes: c.Attributes,
		AppVersion: appVersion,
		ServerName: serverName,
		TraceId:    c.TraceId(),
		SpanId:     c.SpanId(),
		IsStream:   c.Attributes[streamAttributeKey] == "true",
		IsRoot:     true,
	}
}

func (c *ClientTrace) ToTask(appVersion, serverName string) models.Task {
	return models.Task{
		Id:         c.ParsedId(),
		TaskName:   c.Endpoint,
		Duration:   c.Duration,
		RecordedAt: c.RecordedAt,
		ClientIP:   c.ClientIP,
		Attributes: c.Attributes,
		AppVersion: appVersion,
		ServerName: serverName,
		TraceId:    c.TraceId(),
		SpanId:     c.SpanId(),
		IsRoot:     true,
	}
}

type ClientSpan struct {
	ParentSpanId string            `json:"parentSpanId,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	Id           string            `json:"id"`
	Name         string            `json:"name"`
	StartTime    time.Time         `json:"startTime"`
	Duration     time.Duration     `json:"duration"`
}

// ParsedId returns the span ID as uuid.UUID
func (c *ClientSpan) ParsedId() uuid.UUID {
	if parsed, err := uuid.Parse(c.Id); err == nil && parsed != uuid.Nil {
		return parsed
	}
	c.Id = uuid.NewString()
	return uuid.MustParse(c.Id)
}

// ToSpan places a client span in its run's trace. A span that names no parent hangs under the run's root span.
func (c *ClientSpan) ToSpan(run *ClientTrace, projectId uuid.UUID, serverName string) models.Span {
	parent := run.SpanId()
	if parsed, err := uuid.Parse(c.ParentSpanId); err == nil && parsed != uuid.Nil {
		parent = hexId(parsed)
	}
	return models.Span{
		ProjectId:    projectId,
		TraceId:      run.TraceId(),
		SpanId:       hexId(c.ParsedId()),
		ParentSpanId: parent,
		Attributes:   c.Attributes,
		Name:         c.Name,
		StartTime:    c.StartTime,
		Duration:     c.Duration,
		RecordedAt:   run.RecordedAt,
		ServiceName:  serverName,
	}
}

const (
	spanKindInternal = 1
	spanKindServer   = 2
	spanStatusError  = 2
)

// RootSpan is the run itself as a span, so a native trace reads like any other: a root with its children under it.
func (c *ClientTrace) RootSpan(projectId uuid.UUID, serverName string) models.Span {
	span := models.Span{
		ProjectId: projectId, TraceId: c.TraceId(), SpanId: c.SpanId(), Name: c.Endpoint, StartTime: c.RecordedAt,
		Duration: c.Duration, RecordedAt: c.RecordedAt, SpanKind: spanKindServer, ServiceName: serverName, Attributes: c.Attributes,
	}
	if c.IsTask {
		span.SpanKind = spanKindInternal
	}
	if c.StatusCode >= 500 {
		span.StatusCode = spanStatusError
	}
	return span
}

type ClientSessionRecording struct {
	ExceptionId  string          `json:"exceptionId"`
	SessionId    string          `json:"sessionId,omitempty"`
	SegmentIndex int32           `json:"segmentIndex,omitempty"`
	Events       json.RawMessage `json:"events"`
	// Logs and Actions ride into S3 alongside Events. Actions are opaque to the
	// backend; Logs are inspected only to source-map symbolicate the stack
	// trace inside console.error lines (see symbolicateRecordingErrorLogs) for
	// JS projects, then stored. App console logs from session recordings are
	// intentionally NOT inserted into the OTel logs ClickHouse table; they live
	// exclusively inside the S3 recording file.
	Logs      json.RawMessage `json:"logs,omitempty"`
	Actions   json.RawMessage `json:"actions,omitempty"`
	StartedAt *time.Time      `json:"startedAt,omitempty"`
	EndedAt   *time.Time      `json:"endedAt,omitempty"`
}

type ClientSession struct {
	Id         string            `json:"id"`
	StartedAt  time.Time         `json:"startedAt"`
	EndedAt    *time.Time        `json:"endedAt,omitempty"`
	ClientIP   string            `json:"clientIP"`
	Attributes map[string]string `json:"attributes"`
}

func (c *ClientSession) ToSession(appVersion, serverName string) models.Session {
	id, err := uuid.Parse(c.Id)
	if err != nil {
		id = uuid.New()
	}

	var duration int64
	if c.EndedAt != nil {
		duration = c.EndedAt.Sub(c.StartedAt).Nanoseconds()
		if duration < 0 {
			duration = 0
		}
	}

	return models.Session{
		Id:         id,
		StartedAt:  c.StartedAt,
		EndedAt:    c.EndedAt,
		Duration:   duration,
		ClientIP:   c.ClientIP,
		Attributes: c.Attributes,
		AppVersion: appVersion,
		ServerName: serverName,
	}
}

type CollectionFrame struct {
	StackTraces       []*ClientExceptionStackTrace `json:"stackTraces"`
	Metrics           []*ClientMetricRecord        `json:"metrics"`
	Traces            []*ClientTrace               `json:"traces"`
	SessionRecordings []*ClientSessionRecording    `json:"sessionRecordings"`
	Sessions          []*ClientSession             `json:"sessions"`
}

func (f *CollectionFrame) Validate() error {
	if f == nil {
		return fmt.Errorf("collection frame must not be null")
	}
	for _, exception := range f.StackTraces {
		if exception == nil {
			return fmt.Errorf("exception must not be null")
		}
	}
	for _, session := range f.Sessions {
		if session == nil {
			return fmt.Errorf("session must not be null")
		}
	}
	return nil
}
