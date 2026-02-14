package models

import (
	"time"

	"github.com/google/uuid"
)

type SessionRecording struct {
	Id          uuid.UUID `json:"id" ch:"id"`
	ProjectId   uuid.UUID `json:"projectId" ch:"project_id"`
	ExceptionId uuid.UUID `json:"exceptionId" ch:"exception_id"`
	FilePath    string    `json:"filePath" ch:"file_path"`
	RecordedAt  time.Time `json:"recordedAt" ch:"recorded_at"`
}
