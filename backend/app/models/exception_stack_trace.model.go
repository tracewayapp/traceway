package models

import "time"

type ExceptionStackTrace struct {
	ProjectId     string            `json:"projectId" ch:"project_id"`
	TransactionId *string           `json:"transactionId" ch:"transaction_id"`
	ExceptionHash string            `json:"exceptionHash" ch:"exception_hash"`
	StackTrace    string            `json:"stackTrace" ch:"stack_trace"`
	RecordedAt    time.Time         `json:"recordedAt" ch:"recorded_at"`
	Scope         map[string]string `json:"scope" ch:"scope"`
}

type ExceptionTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Count     uint64    `json:"count"`
}

type ExceptionGroup struct {
	ExceptionHash string                `json:"exceptionHash" ch:"exception_hash"`
	StackTrace    string                `json:"stackTrace" ch:"stack_trace"`
	LastSeen      time.Time             `json:"lastSeen" ch:"last_seen"`
	FirstSeen     time.Time             `json:"firstSeen" ch:"first_seen"`
	Count         uint64                `json:"count" ch:"count"`
	HourlyTrend   []ExceptionTrendPoint `json:"hourlyTrend,omitempty"`
}
