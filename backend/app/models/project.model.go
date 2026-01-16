package models

import (
	"time"

	"github.com/google/uuid"
)

type Project struct {
	Id        uuid.UUID `json:"id" ch:"id"`
	Name      string    `json:"name" ch:"name"`
	Token     string    `json:"token" ch:"token"`
	Framework string    `json:"framework" ch:"framework"`
	CreatedAt time.Time `json:"createdAt" ch:"created_at"`
}

// ProjectResponse omits the token for security in listing endpoints
type ProjectResponse struct {
	Id        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Framework string    `json:"framework"`
	CreatedAt time.Time `json:"createdAt"`
}

// ProjectWithToken includes token - used when creating or viewing connection details
type ProjectWithToken struct {
	Id        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	Framework string    `json:"framework"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToResponse converts a Project to ProjectResponse (without token)
func (p *Project) ToResponse() ProjectResponse {
	return ProjectResponse{
		Id:        p.Id,
		Name:      p.Name,
		Framework: p.Framework,
		CreatedAt: p.CreatedAt,
	}
}

// ToWithToken converts a Project to ProjectWithToken
func (p *Project) ToWithToken() ProjectWithToken {
	return ProjectWithToken{
		Id:        p.Id,
		Name:      p.Name,
		Token:     p.Token,
		Framework: p.Framework,
		CreatedAt: p.CreatedAt,
	}
}
