package models

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserOrganizationResponse struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Timezone string `json:"timezone"`
}

type LoginResponse struct {
	Token         string                      `json:"token"`
	User          UserResponse                `json:"user"`
	Projects      []*ProjectWithBackendUrl    `json:"projects"`
	Organizations []*UserOrganizationResponse `json:"organizations"`
}

type RegisterRequest struct {
	Email            string `json:"email" binding:"required,email"`
	Name             string `json:"name" binding:"required"`
	Password         string `json:"password" binding:"required,min=8"`
	OrganizationName string `json:"organizationName" binding:"required"`
	Timezone         string `json:"timezone" binding:"required"`
	ProjectName      string `json:"projectName" binding:"required"`
	Framework        string `json:"framework" binding:"required"`
}

type RegisterResponse struct {
	Token         string                      `json:"token"`
	User          UserResponse                `json:"user"`
	Project       ProjectWithBackendUrl       `json:"project"`
	Projects      []*ProjectWithBackendUrl    `json:"projects"`
	Organizations []*UserOrganizationResponse `json:"organizations"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8"`
}

type PasswordResetTokenInfo struct {
	Valid bool   `json:"valid"`
	Email string `json:"email,omitempty"`
}
