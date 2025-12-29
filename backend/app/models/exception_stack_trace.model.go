package models

import "time"

type ExceptionStackTrace struct {
	TransactionId *string   `json:"transactionId" ch:"transaction_id"`
	StackTrace    string    `json:"stackTrace" ch:"stack_trace"`
	RecordedAt    time.Time `json:"recordedAt" ch:"recorded_at"`
}
