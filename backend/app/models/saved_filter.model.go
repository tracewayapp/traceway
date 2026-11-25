package models

import (
	"time"

	"github.com/google/uuid"
)

// SavedFilter is a named set of filters of one dashboard page, shared by the
// members of its project. Query is the page's URL query string (without the
// project id), so applying one is the same as opening a link.
type SavedFilter struct {
	Id        int       `json:"id" lit:"id"`
	ProjectId uuid.UUID `json:"projectId" lit:"project_id"`
	Page      string    `json:"page" lit:"page"`
	Name      string    `json:"name" lit:"name"`
	Query     string    `json:"query" lit:"query"`
	CreatedBy *int      `json:"createdBy" lit:"created_by"`
	CreatedAt time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" lit:"updated_at"`
}

type SavedFilterListItem struct {
	Id            int       `json:"id" lit:"id"`
	ProjectId     uuid.UUID `json:"projectId" lit:"project_id"`
	Page          string    `json:"page" lit:"page"`
	Name          string    `json:"name" lit:"name"`
	Query         string    `json:"query" lit:"query"`
	CreatedBy     *int      `json:"createdBy" lit:"created_by"`
	CreatedAt     time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt     time.Time `json:"updatedAt" lit:"updated_at"`
	CreatedByName *string   `json:"createdByName" lit:"created_by_name"`
}
