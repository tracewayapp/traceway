package clientmodels

import (
	"backend/app/models"
	"time"
)

type ClientExceptionStackTrace struct {
	TransactionId *string           `json:"transactionId"`
	StackTrace    string            `json:"stackTrace"`
	RecordedAt    time.Time         `json:"recordedAt"`
	Scope         map[string]string `json:"scope"`
	IsMessage     bool              `json:"isMessage"`
}

func (c *ClientExceptionStackTrace) ToExceptionStackTrace(exceptionHash, appVersion, serverName string) models.ExceptionStackTrace {
	return models.ExceptionStackTrace{
		ExceptionHash: exceptionHash,
		TransactionId: c.TransactionId,
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
}

func (c *ClientTransaction) ToTransaction(appVersion, serverName string) models.Transaction {
	return models.Transaction{
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
