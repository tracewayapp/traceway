package models

import "time"

type Segment struct {
	Id            string        `json:"id" ch:"id"`
	TransactionId string        `json:"transactionId" ch:"transaction_id"`
	ProjectId     string        `json:"projectId" ch:"project_id"`
	Name          string        `json:"name" ch:"name"`
	StartTime     time.Time     `json:"startTime" ch:"start_time"`
	Duration      time.Duration `json:"duration" ch:"duration"`
	RecordedAt    time.Time     `json:"recordedAt" ch:"recorded_at"`
}
