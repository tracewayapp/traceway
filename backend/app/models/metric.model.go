package models

import "time"

type MetricsRecord struct {
	Name       string    `json:"name" ch:"name"`
	Value      float32   `json:"value" ch:"value"`
	RecordedAt time.Time `json:"recordedAt" ch:"recorded_at"`
}
