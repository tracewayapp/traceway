package clientmodels

import (
	"backend/app/models"
	"time"
)

type ClientExceptionStackTrace struct {
	TransactionId *string   `json:"transactionId"`
	StackTrace    string    `json:"stackTrace"`
	RecordedAt    time.Time `json:"recordedAt"`
}

func (c *ClientExceptionStackTrace) ToExceptionStackTrace(exceptionHash string) models.ExceptionStackTrace {
	return models.ExceptionStackTrace{
		ExceptionHash: exceptionHash,
		TransactionId: c.TransactionId,
		StackTrace:    c.StackTrace,
		RecordedAt:    c.RecordedAt,
	}
}

type ClientMetricRecord struct {
	Name       string    `json:"name"`
	Value      float64   `json:"value"`
	RecordedAt time.Time `json:"recordedAt"`
}

func (c *ClientMetricRecord) ToMetricRecord() models.MetricRecord {
	return models.MetricRecord{
		Name:       c.Name,
		Value:      c.Value,
		RecordedAt: c.RecordedAt,
	}
}

type ClientTransaction struct {
	Id         string        `json:"id"`
	Endpoint   string        `json:"endpoint"`
	Duration   time.Duration `json:"duration"`
	RecordedAt time.Time     `json:"recordedAt"`
	StatusCode int           `json:"statusCode"`
	BodySize   int           `json:"bodySize"`
	ClientIP   string        `json:"clientIP"`
}

func (c *ClientTransaction) ToTransaction() models.Transaction {
	return models.Transaction{
		Id:         c.Id,
		Endpoint:   c.Endpoint,
		Duration:   c.Duration,
		RecordedAt: c.RecordedAt,
		StatusCode: int32(c.StatusCode),
		BodySize:   int32(c.BodySize),
		ClientIP:   c.ClientIP,
	}
}

type CollectionFrame struct {
	StackTraces  []*ClientExceptionStackTrace `json:"stackTraces"`
	Metrics      []*ClientMetricRecord        `json:"metrics"`
	Transactions []*ClientTransaction         `json:"transactions"`
}
