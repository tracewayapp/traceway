package models

import (
	"time"

	"github.com/google/uuid"
)

type SourceMap struct {
	Id        int       `json:"id"`
	ProjectId uuid.UUID `json:"projectId"`
	Version   string    `json:"version"`
	FileName  string    `json:"fileName"`
	StorageKey string   `json:"storageKey"`
	FileSize  int64     `json:"fileSize"`
	UploadedAt time.Time `json:"uploadedAt"`
}
