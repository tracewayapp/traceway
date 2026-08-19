package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	IncidentUpdateInvestigating = "investigating"
	IncidentUpdateIdentified    = "identified"
	IncidentUpdateMonitoring    = "monitoring"
	IncidentUpdateResolved      = "resolved"
	IncidentUpdateNote          = "update"
)

type IncidentUpdate struct {
	Id         int       `json:"id" lit:"id"`
	IncidentId int       `json:"incidentId" lit:"incident_id"`
	Status     string    `json:"status" lit:"status"`
	Message    string    `json:"message" lit:"message"`
	CreatedBy  *int      `json:"createdBy" lit:"created_by"`
	CreatedAt  time.Time `json:"createdAt" lit:"created_at"`
}

type IncidentUpdateCount struct {
	IncidentId int `lit:"incident_id"`
	Count      int `lit:"count"`
}

type OrgIncident struct {
	Id             int        `json:"id" lit:"id"`
	CheckId        *int       `json:"checkId" lit:"check_id"`
	ProjectId      *uuid.UUID `json:"projectId" lit:"project_id"`
	StatusPageId   *int       `json:"statusPageId" lit:"status_page_id"`
	Title          string     `json:"title" lit:"title"`
	StartedAt      time.Time  `json:"startedAt" lit:"started_at"`
	ResolvedAt     *time.Time `json:"resolvedAt" lit:"resolved_at"`
	ErrorMessage   string     `json:"errorMessage" lit:"error_message"`
	CheckName      *string    `json:"checkName" lit:"check_name"`
	StatusPageName *string    `json:"statusPageName" lit:"status_page_name"`
}

func IsValidIncidentUpdateStatus(status string) bool {
	switch status {
	case IncidentUpdateInvestigating, IncidentUpdateIdentified, IncidentUpdateMonitoring, IncidentUpdateResolved, IncidentUpdateNote:
		return true
	}
	return false
}
