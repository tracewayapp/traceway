package models

import (
	"time"

	"github.com/google/uuid"
)

type SessionRecording struct {
	Id           uuid.UUID  `json:"id" ch:"id"`
	ProjectId    uuid.UUID  `json:"projectId" ch:"project_id"`
	ExceptionId  uuid.UUID  `json:"exceptionId" ch:"exception_id"`
	SessionId    *uuid.UUID `json:"sessionId,omitempty" ch:"session_id"`
	SegmentIndex int32      `json:"segmentIndex" ch:"segment_index"`
	FilePath     string     `json:"filePath" ch:"file_path"`
	RecordedAt   time.Time  `json:"recordedAt" ch:"recorded_at"`
	EndedAt      *time.Time `json:"endedAt,omitempty" ch:"ended_at"`
}
