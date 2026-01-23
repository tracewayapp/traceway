package models

import (
	"time"
)

type Invitation struct {
	Id             int        `json:"id"`
	OrganizationId int        `json:"organizationId"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	Token          string     `json:"-"`
	InvitedBy      int        `json:"invitedBy"`
	Status         string     `json:"status"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	AcceptedAt     *time.Time `json:"acceptedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type InvitationWithInviter struct {
	Id             int        `json:"id"`
	OrganizationId int        `json:"organizationId"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	InvitedBy      int        `json:"invitedBy"`
	InviterName    string     `json:"inviterName" lit:"inviter_name"`
	Status         string     `json:"status"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	AcceptedAt     *time.Time `json:"acceptedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type InviteUserRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required,oneof=admin user readonly"`
}

type AcceptInvitationRequest struct {
	Name     string `json:"name" binding:"required,min=1"`
	Password string `json:"password" binding:"required,min=8"`
}

type InvitationInfoResponse struct {
	Email           string `json:"email"`
	OrganizationName string `json:"organizationName"`
	InviterName     string `json:"inviterName"`
	ExistsAsUser    bool   `json:"existsAsUser"`
	Role            string `json:"role"`
}
