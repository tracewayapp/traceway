package models

import "time"

type PostMortem struct {
	Id             int         `json:"id" lit:"id"`
	OrganizationId int         `json:"organizationId" lit:"organization_id"`
	IncidentId     *int        `json:"incidentId" lit:"incident_id"`
	Title          string      `json:"title" lit:"title"`
	ContentMd      string      `json:"contentMd" lit:"content_md"`
	Tags           StringSlice `json:"tags" lit:"tags"`
	CreatedBy      *int        `json:"createdBy" lit:"created_by"`
	CreatedAt      time.Time   `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time   `json:"updatedAt" lit:"updated_at"`
}

type PostMortemListItem struct {
	Id             int         `json:"id" lit:"id"`
	OrganizationId int         `json:"organizationId" lit:"organization_id"`
	IncidentId     *int        `json:"incidentId" lit:"incident_id"`
	Title          string      `json:"title" lit:"title"`
	Tags           StringSlice `json:"tags" lit:"tags"`
	CreatedBy      *int        `json:"createdBy" lit:"created_by"`
	CreatedAt      time.Time   `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time   `json:"updatedAt" lit:"updated_at"`
}

type PostMortemRef struct {
	Id         int  `json:"id" lit:"id"`
	IncidentId *int `json:"incidentId" lit:"incident_id"`
}
