package models

import "time"

type ExceptionStackTrace struct {
	TransactionId *string   `json:"transactionId" ch:"transaction_id"`
	ExceptionHash string    `json:"exceptionHash" ch:"exception_hash"`
	StackTrace    string    `json:"stackTrace" ch:"stack_trace"`
	RecordedAt    time.Time `json:"recordedAt" ch:"recorded_at"`
}
