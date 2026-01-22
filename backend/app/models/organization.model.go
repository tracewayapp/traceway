package models

import (
	"time"
)

type Organization struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type OrganizationUser struct {
	Id             int       `json:"id"`
	UserId         int       `json:"userId"`
	OrganizationId int       `json:"organizationId"`
	Role           string    `json:"role"` // owner, admin, user
	CreatedAt      time.Time `json:"createdAt"`
}
