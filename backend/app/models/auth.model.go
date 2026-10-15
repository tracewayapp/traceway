package models

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	// Scope turns the login into an agent credential (the CLI's password
	// login) instead of a dashboard session.
	Scope string `json:"scope"`
}

type UserOrganizationResponse struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Timezone string `json:"timezone"`
}

type LoginResponse struct {
	Token string `json:"token"`
	// RefreshToken and ExpiresIn are set only on a scoped login, whose Token
	// is then a short-lived access token in a revocable refresh family.
	RefreshToken  string                      `json:"refreshToken,omitempty"`
	ExpiresIn     int                         `json:"expiresIn,omitempty"`
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
	ProjectName      string `json:"projectName"`
	Framework        string `json:"framework"`
	CaptchaToken     string `json:"captchaToken"`
}

type RegisterResponse struct {
	Token         string                      `json:"token"`
	User          UserResponse                `json:"user"`
	Project       *ProjectWithBackendUrl      `json:"project,omitempty"`
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
