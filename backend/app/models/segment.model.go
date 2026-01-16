package models

import (
	"time"

	"github.com/google/uuid"
)

type Segment struct {
	Id            uuid.UUID     `json:"id" ch:"id"`
	TransactionId uuid.UUID     `json:"transactionId" ch:"transaction_id"`
	ProjectId     uuid.UUID     `json:"projectId" ch:"project_id"`
	Name          string        `json:"name" ch:"name"`
	StartTime     time.Time     `json:"startTime" ch:"start_time"`
	Duration      time.Duration `json:"duration" ch:"duration"`
	RecordedAt    time.Time     `json:"recordedAt" ch:"recorded_at"`
}
