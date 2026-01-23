package models

import (
	"time"
)

type Organization struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"createdAt"`
}

type OrganizationUser struct {
	Id             int       `json:"id"`
	UserId         int       `json:"userId"`
	OrganizationId int       `json:"organizationId"`
	Role           string    `json:"role"` // owner, admin, user, readonly
	CreatedAt      time.Time `json:"createdAt"`
}

// OrganizationMember represents a user with their role in an organization
type OrganizationMember struct {
	Id        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// OrganizationSettingsResponse is returned for the settings page
type OrganizationSettingsResponse struct {
	Organization *Organization             `json:"organization"`
	Members      []*OrganizationMember     `json:"members"`
	Invitations  []*InvitationWithInviter  `json:"invitations"`
	UserRole     string                    `json:"userRole"`
}

// UpdateMemberRoleRequest is the request body for updating a member's role
type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=admin user readonly"`
}
