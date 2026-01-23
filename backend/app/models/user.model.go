package models

import (
	"time"
)

type User struct {
	Id                       int        `json:"id"`
	Email                    string     `json:"email"`
	Name                     string     `json:"name"`
	Password                 string     `json:"-"`
	CreatedAt                time.Time  `json:"createdAt"`
	PasswordResetToken       *string    `json:"-"`
	PasswordResetExpiresAt   *time.Time `json:"-"`
	PasswordResetRequestedAt *time.Time `json:"-"`
}

type UserResponse struct {
	Id        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		Id:        u.Id,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
	}
}
