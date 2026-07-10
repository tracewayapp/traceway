package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type WidgetGroup struct {
	Id          int       `json:"id" lit:"id"`
	ProjectId   uuid.UUID `json:"projectId" lit:"project_id"`
	Name        string    `json:"name" lit:"name"`
	Description string    `json:"description" lit:"description"`
	IsDefault   bool      `json:"isDefault" lit:"is_default"`
	CreatedBy   *int      `json:"createdBy" lit:"created_by"`
	CreatedAt   time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt   time.Time `json:"updatedAt" lit:"updated_at"`
}

type WidgetGroupWidget struct {
	Id            int             `json:"id" lit:"id"`
	WidgetGroupId int             `json:"widgetGroupId" lit:"widget_group_id"`
	Title         string          `json:"title" lit:"title"`
	WidgetType    string          `json:"widgetType" lit:"widget_type"`
	Config        json.RawMessage `json:"config" lit:"config"`
	Position      int             `json:"position" lit:"position"`
	CreatedAt     time.Time       `json:"createdAt" lit:"created_at"`
	UpdatedAt     time.Time       `json:"updatedAt" lit:"updated_at"`
}

type WidgetGroupWidgetWithStar struct {
	Id            int             `json:"id" lit:"id"`
	WidgetGroupId int             `json:"widgetGroupId" lit:"widget_group_id"`
	Title         string          `json:"title" lit:"title"`
	WidgetType    string          `json:"widgetType" lit:"widget_type"`
	Config        json.RawMessage `json:"config" lit:"config"`
	Position      int             `json:"position" lit:"position"`
	IsStarred     bool            `json:"isStarred" lit:"is_starred"`
	CreatedAt     time.Time       `json:"createdAt" lit:"created_at"`
	UpdatedAt     time.Time       `json:"updatedAt" lit:"updated_at"`
}

type StarredWidget struct {
	Id        int       `json:"id" lit:"id"`
	WidgetId  int       `json:"widgetId" lit:"widget_id"`
	Position  int       `json:"position" lit:"position"`
	ColSpan   int       `json:"colSpan" lit:"col_span"`
	Size      string    `json:"size" lit:"size"`
	CreatedAt time.Time `json:"createdAt" lit:"created_at"`
}

type StarredWidgetWithHome struct {
	Id            int             `json:"id" lit:"id"`
	WidgetGroupId int             `json:"widgetGroupId" lit:"widget_group_id"`
	Title         string          `json:"title" lit:"title"`
	WidgetType    string          `json:"widgetType" lit:"widget_type"`
	Config        json.RawMessage `json:"config" lit:"config"`
	Position      int             `json:"position" lit:"position"`
	HomePosition  int             `json:"homePosition" lit:"home_position"`
	HomeColSpan   int             `json:"homeColSpan" lit:"home_col_span"`
	HomeSize      string          `json:"homeSize" lit:"home_size"`
	CreatedAt     time.Time       `json:"createdAt" lit:"created_at"`
	UpdatedAt     time.Time       `json:"updatedAt" lit:"updated_at"`
}

type WidgetGroupWithWidgets struct {
	WidgetGroup
	Widgets []WidgetGroupWidgetWithStar `json:"widgets"`
}
