package clientmodels

import "time"

type ClientExceptionStackTrace struct {
	TransactionId *string   `json:"transactionId"`
	StackTrace    string    `json:"stackTrace"`
	RecordedAt    time.Time `json:"recordedAt"`
}

type ClientMetricsRecord struct {
	Name       string    `json:"name"`
	Value      float32   `json:"value"`
	RecordedAt time.Time `json:"recordedAt"`
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

type CollectionFrame struct {
	StackTraces  []*ClientExceptionStackTrace `json:"stackTraces"`
	Metrics      []*ClientMetricsRecord       `json:"metrics"`
	Transactions []*ClientTransaction         `json:"transactions"`
}
