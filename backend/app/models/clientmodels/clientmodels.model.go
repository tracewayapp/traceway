package clientmodels

import (
	"backend/app/models"
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

	return models.ExceptionStackTrace{
		ExceptionHash: exceptionHash,
		TraceId:       traceId,
		TraceType:     traceType,
		StackTrace:    c.StackTrace,
		RecordedAt:    c.RecordedAt,
		Attributes:    c.Attributes,
		IsMessage:     c.IsMessage,
		AppVersion:    appVersion,
		ServerName:    serverName,
	}
}

type ClientMetricRecord struct {
	Name       string    `json:"name"`
	Value      float64   `json:"value"`
	RecordedAt time.Time `json:"recordedAt"`
}

func (c *ClientMetricRecord) ToMetricRecord(serverName string) models.MetricRecord {
	return models.MetricRecord{
		Name:       c.Name,
		Value:      c.Value,
		RecordedAt: c.RecordedAt,
		ServerName: serverName,
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

// ParsedId returns the trace ID as uuid.UUID
func (c *ClientTrace) ParsedId() uuid.UUID {
	if parsed, err := uuid.Parse(c.Id); err == nil {
		return parsed
	}
	return uuid.New()
}

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
	ExceptionId string          `json:"exceptionId"`
	Events      json.RawMessage `json:"events"`
}

type CollectionFrame struct {
	StackTraces       []*ClientExceptionStackTrace `json:"stackTraces"`
	Metrics           []*ClientMetricRecord        `json:"metrics"`
	Traces            []*ClientTrace               `json:"traces"`
	SessionRecordings []*ClientSessionRecording    `json:"sessionRecordings"`
}
