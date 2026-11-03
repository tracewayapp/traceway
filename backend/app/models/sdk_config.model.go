package models

import (
	"time"

	"github.com/google/uuid"
)

// SdkConfigRule targets remote collection settings at the OpenTelemetry SDK clients of a project whose reported
// attributes match its conditions. Rules apply in position order; see the sdkconfig package for both documents.
type SdkConfigRule struct {
	Id         int       `json:"id" lit:"id"`
	ProjectId  uuid.UUID `json:"projectId" lit:"project_id"`
	Name       string    `json:"name" lit:"name"`
	Position   int       `json:"position" lit:"position"`
	Enabled    bool      `json:"enabled" lit:"enabled"`
	Conditions JSONText  `json:"conditions" lit:"conditions"`
	Settings   JSONText  `json:"settings" lit:"settings"`
	CreatedBy  *int      `json:"createdBy" lit:"created_by"`
	CreatedAt  time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt  time.Time `json:"updatedAt" lit:"updated_at"`
}

// SdkCapture is an on-demand capture started from the dashboard: until it expires or is cancelled, the clients
// matching its conditions upload what they buffered and stream everything.
type SdkCapture struct {
	DeliveredAt *time.Time `json:"deliveredAt" lit:"delivered_at"`
	AppliedAt   *time.Time `json:"appliedAt" lit:"applied_at"`
	ReceivedAt  *time.Time `json:"receivedAt" lit:"received_at"`
	Id          uuid.UUID  `json:"id" lit:"id"`
	ProjectId   uuid.UUID  `json:"projectId" lit:"project_id"`
	Label       string     `json:"label" lit:"label"`
	Conditions  JSONText   `json:"conditions" lit:"conditions"`
	Actions     JSONText   `json:"actions" lit:"actions"`
	ExpiresAt   time.Time  `json:"expiresAt" lit:"expires_at"`
	CancelledAt *time.Time `json:"cancelledAt" lit:"cancelled_at"`
	CreatedBy   *int       `json:"createdBy" lit:"created_by"`
	CreatedAt   time.Time  `json:"createdAt" lit:"created_at"`
}
