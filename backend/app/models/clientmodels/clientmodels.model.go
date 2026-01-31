package clientmodels

import (
	"backend/app/models"
	"time"

	"github.com/google/uuid"
)

type ClientExceptionStackTrace struct {
	TraceId    *string           `json:"traceId"`
	IsTask     bool              `json:"isTask"`
	StackTrace string            `json:"stackTrace"`
	RecordedAt time.Time         `json:"recordedAt"`
	Scope      map[string]string `json:"scope"`
	IsMessage  bool              `json:"isMessage"`
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
		Scope:         c.Scope,
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
	Scope      map[string]string `json:"scope"`
	Segments   []*ClientSegment  `json:"segments"`
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
		Scope:      c.Scope,
		AppVersion: appVersion,
		ServerName: serverName,
	}
}

func (c *ClientTrace) ToTask(appVersion, serverName string) models.Task {
	return models.Task{
		Id:         c.ParsedId(),
		TaskName:   c.Endpoint, // Endpoint field is used as task name
		Duration:   c.Duration,
		RecordedAt: c.RecordedAt,
		ClientIP:   c.ClientIP,
		Scope:      c.Scope,
		AppVersion: appVersion,
		ServerName: serverName,
	}
}

type ClientSegment struct {
	Id        string        `json:"id"`
	Name      string        `json:"name"`
	StartTime time.Time     `json:"startTime"`
	Duration  time.Duration `json:"duration"`
}

// ParsedId returns the segment ID as uuid.UUID
func (c *ClientSegment) ParsedId() uuid.UUID {
	if parsed, err := uuid.Parse(c.Id); err == nil {
		return parsed
	}
	return uuid.New()
}

func (c *ClientSegment) ToSegment(traceId uuid.UUID) models.Segment {
	return models.Segment{
		Id:      c.ParsedId(),
		TraceId: traceId,
		Name:          c.Name,
		StartTime:     c.StartTime,
		Duration:      c.Duration,
		RecordedAt:    time.Now(),
	}
}

type CollectionFrame struct {
	StackTraces  []*ClientExceptionStackTrace `json:"stackTraces"`
	Metrics      []*ClientMetricRecord        `json:"metrics"`
	Traces       []*ClientTrace               `json:"traces"`
}
