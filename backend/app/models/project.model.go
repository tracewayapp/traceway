package models

import (
	"os"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	Id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Token          string    `json:"token"`
	Framework      string    `json:"framework"`
	OrganizationId *int      `json:"organizationId"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (p Project) ToProjectWithBackendUrl() *ProjectWithBackendUrl {
	return &ProjectWithBackendUrl{Project: p, BackendUrl: getBackendUrl()}
}

func getBackendUrl() string {
	if url := os.Getenv("APP_BASE_URL"); url != "" {
		return url
	}
	return "https://cloud.tracewayapp.com"
}

type ProjectWithBackendUrl struct {
	Project
	BackendUrl string `json:"backendUrl"`
}
