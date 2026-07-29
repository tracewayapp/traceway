package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Dashboard struct {
	Id             int       `json:"id" lit:"id"`
	OrganizationId int       `json:"organizationId" lit:"organization_id"`
	Name           string    `json:"name" lit:"name"`
	Description    string    `json:"description" lit:"description"`
	Definition     JSONText  `json:"definition" lit:"definition"`
	TemplateKey    *string   `json:"templateKey" lit:"template_key"`
	CreatedBy      *int      `json:"createdBy" lit:"created_by"`
	CreatedAt      time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" lit:"updated_at"`
}

type ProjectDashboard struct {
	Id          int       `json:"id" lit:"id"`
	ProjectId   uuid.UUID `json:"projectId" lit:"project_id"`
	DashboardId int       `json:"dashboardId" lit:"dashboard_id"`
	Position    int       `json:"position" lit:"position"`
	CreatedAt   time.Time `json:"createdAt" lit:"created_at"`
}

type DashboardAssignment struct {
	DashboardId int       `json:"dashboardId" lit:"dashboard_id"`
	ProjectId   uuid.UUID `json:"projectId" lit:"project_id"`
}

type DashboardTemplate struct {
	Id          int       `json:"id" lit:"id"`
	Key         string    `json:"key" lit:"key"`
	Name        string    `json:"name" lit:"name"`
	Description string    `json:"description" lit:"description"`
	Category    string    `json:"category" lit:"category"`
	Definition  JSONText  `json:"definition" lit:"definition"`
	CreatedAt   time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt   time.Time `json:"updatedAt" lit:"updated_at"`
}

type StarredDashboardWidget struct {
	Id          int       `json:"id" lit:"id"`
	ProjectId   uuid.UUID `json:"projectId" lit:"project_id"`
	DashboardId int       `json:"dashboardId" lit:"dashboard_id"`
	WidgetId    string    `json:"widgetId" lit:"widget_id"`
	Position    int       `json:"position" lit:"position"`
	ColSpan     int       `json:"colSpan" lit:"col_span"`
	Size        string    `json:"size" lit:"size"`
	CreatedAt   time.Time `json:"createdAt" lit:"created_at"`
}

type DashboardDefinition struct {
	SchemaVersion int               `json:"schemaVersion"`
	Widgets       []DashboardWidget `json:"widgets"`
}

type DashboardWidget struct {
	Id         string          `json:"id"`
	Title      string          `json:"title"`
	WidgetType string          `json:"widgetType"`
	Config     json.RawMessage `json:"config"`
}

type DashboardExport struct {
	SchemaVersion int               `json:"schemaVersion"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Widgets       []DashboardWidget `json:"widgets"`
}

type DashboardBundleExport struct {
	SchemaVersion int               `json:"schemaVersion"`
	Dashboards    []DashboardExport `json:"dashboards"`
}
