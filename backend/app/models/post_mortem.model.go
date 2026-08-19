package models

import (
	"time"

	"github.com/google/uuid"
)

type PostMortem struct {
	Id             int         `json:"id" lit:"id"`
	OrganizationId int         `json:"organizationId" lit:"organization_id"`
	ProjectId      uuid.UUID   `json:"projectId" lit:"project_id"`
	IncidentId     *int        `json:"incidentId" lit:"incident_id"`
	Title          string      `json:"title" lit:"title"`
	ContentMd      string      `json:"contentMd" lit:"content_md"`
	Tags           StringSlice `json:"tags" lit:"tags"`
	CreatedBy      *int        `json:"createdBy" lit:"created_by"`
	UpdatedBy      *int        `json:"updatedBy" lit:"updated_by"`
	CreatedAt      time.Time   `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time   `json:"updatedAt" lit:"updated_at"`
}

type PostMortemDetail struct {
	Id             int         `json:"id" lit:"id"`
	OrganizationId int         `json:"organizationId" lit:"organization_id"`
	ProjectId      uuid.UUID   `json:"projectId" lit:"project_id"`
	IncidentId     *int        `json:"incidentId" lit:"incident_id"`
	Title          string      `json:"title" lit:"title"`
	ContentMd      string      `json:"contentMd" lit:"content_md"`
	Tags           StringSlice `json:"tags" lit:"tags"`
	CreatedBy      *int        `json:"createdBy" lit:"created_by"`
	UpdatedBy      *int        `json:"updatedBy" lit:"updated_by"`
	CreatedAt      time.Time   `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time   `json:"updatedAt" lit:"updated_at"`
	CreatedByName  *string     `json:"createdByName" lit:"created_by_name"`
	UpdatedByName  *string     `json:"updatedByName" lit:"updated_by_name"`
}

type PostMortemListItem struct {
	Id             int         `json:"id" lit:"id"`
	OrganizationId int         `json:"organizationId" lit:"organization_id"`
	ProjectId      uuid.UUID   `json:"projectId" lit:"project_id"`
	IncidentId     *int        `json:"incidentId" lit:"incident_id"`
	Title          string      `json:"title" lit:"title"`
	Tags           StringSlice `json:"tags" lit:"tags"`
	CreatedBy      *int        `json:"createdBy" lit:"created_by"`
	UpdatedBy      *int        `json:"updatedBy" lit:"updated_by"`
	CreatedAt      time.Time   `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time   `json:"updatedAt" lit:"updated_at"`
	CreatedByName  *string     `json:"createdByName" lit:"created_by_name"`
	UpdatedByName  *string     `json:"updatedByName" lit:"updated_by_name"`
}

type PostMortemRef struct {
	Id         int  `json:"id" lit:"id"`
	IncidentId *int `json:"incidentId" lit:"incident_id"`
}

const (
	PostMortemEventCreated = "created"
	PostMortemEventUpdated = "updated"
)

type PostMortemEvent struct {
	Id           int         `json:"id" lit:"id"`
	PostMortemId int         `json:"postMortemId" lit:"post_mortem_id"`
	UserId       *int        `json:"userId" lit:"user_id"`
	Action       string      `json:"action" lit:"action"`
	Changes      StringSlice `json:"changes" lit:"changes"`
	CreatedAt    time.Time   `json:"createdAt" lit:"created_at"`
}

type PostMortemEventItem struct {
	Id           int         `json:"id" lit:"id"`
	PostMortemId int         `json:"postMortemId" lit:"post_mortem_id"`
	UserId       *int        `json:"userId" lit:"user_id"`
	Action       string      `json:"action" lit:"action"`
	Changes      StringSlice `json:"changes" lit:"changes"`
	CreatedAt    time.Time   `json:"createdAt" lit:"created_at"`
	UserName     *string     `json:"userName" lit:"user_name"`
}
