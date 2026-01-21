package models

import (
	"os"
	"time"

	"github.com/google/uuid"
)

func getBackendUrl() string {
	if url := os.Getenv("BACKEND_URL"); url != "" {
		return url
	}
	return "https://tracewayapp.com"
}

type Project struct {
	Id        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	Framework string    `json:"framework"`
	CreatedAt time.Time `json:"createdAt"`
}

type ProjectResponse struct {
	Id         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Framework  string    `json:"framework"`
	CreatedAt  time.Time `json:"createdAt"`
	BackendUrl string    `json:"backendUrl"`
}

type ProjectWithToken struct {
	Id         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Token      string    `json:"token"`
	Framework  string    `json:"framework"`
	CreatedAt  time.Time `json:"createdAt"`
	BackendUrl string    `json:"backendUrl"`
}

func (p *Project) ToResponse() ProjectResponse {
	return ProjectResponse{
		Id:         p.Id,
		Name:       p.Name,
		Framework:  p.Framework,
		CreatedAt:  p.CreatedAt,
		BackendUrl: getBackendUrl(),
	}
}

func (p *Project) ToWithToken() ProjectWithToken {
	return ProjectWithToken{
		Id:         p.Id,
		Name:       p.Name,
		Token:      p.Token,
		Framework:  p.Framework,
		CreatedAt:  p.CreatedAt,
		BackendUrl: getBackendUrl(),
	}
}
