package clientmodels

import (
	"github.com/tracewayapp/traceway/backend/app/models"
	"encoding/json"
	"time"

	"github.com/google/uuid"
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
	DistributedTraceId *string           `json:"distributedTraceId"`
	DebugIds           map[string]string `json:"debugIds"`
}

func (c *ClientExceptionStackTrace) ToExceptionStackTrace(exceptionHash, appVersion, serverName string) models.ExceptionStackTrace {
	traceType := "endpoint"
	if c.IsTask {
		traceType = "task"
	}

	var traceId *uuid.UUID
	if c.TraceId != nil {
		if parsed, err := uuid.Parse(*c.TraceId); err == nil {
			traceId = &parsed
		}
	}

	var distributedTraceId *uuid.UUID
	if c.DistributedTraceId != nil {
		if parsed, err := uuid.Parse(*c.DistributedTraceId); err == nil {
			distributedTraceId = &parsed
		}
	}

	var sessionId *uuid.UUID
	if c.SessionId != nil {
		if parsed, err := uuid.Parse(*c.SessionId); err == nil {
			sessionId = &parsed
		}
	}

	return models.ExceptionStackTrace{
		ExceptionHash:      exceptionHash,
		TraceId:            traceId,
		TraceType:          traceType,
		StackTrace:         c.StackTrace,
		RecordedAt:         c.RecordedAt,
		Attributes:         c.Attributes,
		IsMessage:          c.IsMessage,
		AppVersion:         appVersion,
		ServerName:         serverName,
		DistributedTraceId: distributedTraceId,
		SessionId:          sessionId,
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
	Id                 string            `json:"id"`
	Endpoint           string            `json:"endpoint"`
	Duration           time.Duration     `json:"duration"`
	RecordedAt         time.Time         `json:"recordedAt"`
	StatusCode         int               `json:"statusCode"`
	BodySize           int               `json:"bodySize"`
	ClientIP           string            `json:"clientIP"`
	Attributes         map[string]string `json:"attributes"`
	Spans              []*ClientSpan     `json:"spans"`
	IsTask             bool              `json:"isTask"`
	DistributedTraceId string            `json:"distributedTraceId"`
}

const streamAttributeKey = "traceway.is_stream"

// ParsedId returns the trace ID as uuid.UUID
func (c *ClientTrace) ParsedId() uuid.UUID {
	if parsed, err := uuid.Parse(c.Id); err == nil {
		return parsed
	}
	return uuid.New()
}

func (c *ClientTrace) parsedDistributedTraceId() *uuid.UUID {
	if c.DistributedTraceId == "" {
		return nil
	}
	if parsed, err := uuid.Parse(c.DistributedTraceId); err == nil {
		return &parsed
	}
	return nil
}

func (c *ClientTrace) ToEndpoint(appVersion, serverName string) models.Endpoint {
	return models.Endpoint{
		Id:                 c.ParsedId(),
		Endpoint:           c.Endpoint,
		Duration:           c.Duration,
		RecordedAt:         c.RecordedAt,
		StatusCode:         int16(c.StatusCode),
		BodySize:           int32(c.BodySize),
		ClientIP:           c.ClientIP,
		Attributes:         c.Attributes,
		AppVersion:         appVersion,
		ServerName:         serverName,
		DistributedTraceId: c.parsedDistributedTraceId(),
		IsStream:           c.Attributes[streamAttributeKey] == "true",
		IsRoot:             true,
	}
}

func (c *ClientTrace) ToTask(appVersion, serverName string) models.Task {
	return models.Task{
		Id:                 c.ParsedId(),
		TaskName:           c.Endpoint,
		Duration:           c.Duration,
		RecordedAt:         c.RecordedAt,
		ClientIP:           c.ClientIP,
		Attributes:         c.Attributes,
		AppVersion:         appVersion,
		ServerName:         serverName,
		DistributedTraceId: c.parsedDistributedTraceId(),
		IsRoot:             true,
	}
}

type ClientSpan struct {
	Id        string        `json:"id"`
	Name      string        `json:"name"`
	StartTime time.Time     `json:"startTime"`
	Duration  time.Duration `json:"duration"`
}

// ParsedId returns the span ID as uuid.UUID
func (c *ClientSpan) ParsedId() uuid.UUID {
	if parsed, err := uuid.Parse(c.Id); err == nil {
		return parsed
	}
	return uuid.New()
}

func (c *ClientSpan) ToSpan(traceId uuid.UUID) models.Span {
	return models.Span{
		Id:      c.ParsedId(),
		TraceId: traceId,
		Name:          c.Name,
		StartTime:     c.StartTime,
		Duration:      c.Duration,
		RecordedAt:    time.Now(),
	}
}

type ClientSessionRecording struct {
	ExceptionId  string          `json:"exceptionId"`
	SessionId    string          `json:"sessionId,omitempty"`
	SegmentIndex int32           `json:"segmentIndex,omitempty"`
	Events       json.RawMessage `json:"events"`
	// Logs and Actions are opaque to the backend — they ride into S3 alongside
	// Events without ever being inspected. App console logs from session
	// recordings are intentionally NOT inserted into the OTel logs ClickHouse
	// table; they live exclusively inside the S3 recording file.
	Logs      json.RawMessage `json:"logs,omitempty"`
	Actions   json.RawMessage `json:"actions,omitempty"`
	StartedAt *time.Time      `json:"startedAt,omitempty"`
	EndedAt   *time.Time      `json:"endedAt,omitempty"`
}

type ClientSession struct {
	Id                 string            `json:"id"`
	StartedAt          time.Time         `json:"startedAt"`
	EndedAt            *time.Time        `json:"endedAt,omitempty"`
	ClientIP           string            `json:"clientIP"`
	Attributes         map[string]string `json:"attributes"`
	DistributedTraceId string            `json:"distributedTraceId,omitempty"`
}

func (c *ClientSession) ToSession(appVersion, serverName string) models.Session {
	id, err := uuid.Parse(c.Id)
	if err != nil {
		id = uuid.New()
	}

	var distributedTraceId *uuid.UUID
	if c.DistributedTraceId != "" {
		if parsed, err := uuid.Parse(c.DistributedTraceId); err == nil {
			distributedTraceId = &parsed
		}
	}

	var duration int64
	if c.EndedAt != nil {
		duration = c.EndedAt.Sub(c.StartedAt).Nanoseconds()
		if duration < 0 {
			duration = 0
		}
	}

	return models.Session{
		Id:                 id,
		StartedAt:          c.StartedAt,
		EndedAt:            c.EndedAt,
		Duration:           duration,
		ClientIP:           c.ClientIP,
		Attributes:         c.Attributes,
		AppVersion:         appVersion,
		ServerName:         serverName,
		DistributedTraceId: distributedTraceId,
	}
}

type CollectionFrame struct {
	StackTraces       []*ClientExceptionStackTrace `json:"stackTraces"`
	Metrics           []*ClientMetricRecord        `json:"metrics"`
	Traces            []*ClientTrace               `json:"traces"`
	SessionRecordings []*ClientSessionRecording    `json:"sessionRecordings"`
	Sessions          []*ClientSession             `json:"sessions"`
}
