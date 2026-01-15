package clientmodels

import (
	"backend/app/models"
	"time"
)

type ClientExceptionStackTrace struct {
	TransactionId *string           `json:"transactionId"`
	IsTask        bool              `json:"isTask"`
	StackTrace    string            `json:"stackTrace"`
	RecordedAt    time.Time         `json:"recordedAt"`
	Scope         map[string]string `json:"scope"`
	IsMessage     bool              `json:"isMessage"`
}

func (c *ClientExceptionStackTrace) ToExceptionStackTrace(exceptionHash, appVersion, serverName string) models.ExceptionStackTrace {
	transactionType := "endpoint"
	if c.IsTask {
		transactionType = "task"
	}
	return models.ExceptionStackTrace{
		ExceptionHash:   exceptionHash,
		TransactionId:   c.TransactionId,
		TransactionType: transactionType,
		StackTrace:      c.StackTrace,
		RecordedAt:      c.RecordedAt,
		Scope:           c.Scope,
		IsMessage:       c.IsMessage,
		AppVersion:      appVersion,
		ServerName:      serverName,
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

type ClientTransaction struct {
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

func (c *ClientTransaction) ToEndpoint(appVersion, serverName string) models.Endpoint {
	return models.Endpoint{
		Id:         c.Id,
		Endpoint:   c.Endpoint,
		Duration:   c.Duration,
		RecordedAt: c.RecordedAt,
		StatusCode: int32(c.StatusCode),
		BodySize:   int32(c.BodySize),
		ClientIP:   c.ClientIP,
		Scope:      c.Scope,
		AppVersion: appVersion,
		ServerName: serverName,
	}
}

func (c *ClientTransaction) ToTask(appVersion, serverName string) models.Task {
	return models.Task{
		Id:         c.Id,
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

func (c *ClientSegment) ToSegment(transactionId string) models.Segment {
	return models.Segment{
		Id:            c.Id,
		TransactionId: transactionId,
		Name:          c.Name,
		StartTime:     c.StartTime,
		Duration:      c.Duration,
		RecordedAt:    time.Now(),
	}
}

type CollectionFrame struct {
	StackTraces  []*ClientExceptionStackTrace `json:"stackTraces"`
	Metrics      []*ClientMetricRecord        `json:"metrics"`
	Transactions []*ClientTransaction         `json:"transactions"`
}
