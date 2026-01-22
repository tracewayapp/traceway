package models

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token    string            `json:"token"`
	User     UserResponse      `json:"user"`
	Projects []ProjectResponse `json:"projects"`
}

type RegisterRequest struct {
	Email            string `json:"email" binding:"required,email"`
	Name             string `json:"name" binding:"required"`
	Password         string `json:"password" binding:"required,min=8"`
	OrganizationName string `json:"organizationName" binding:"required"`
	ProjectName      string `json:"projectName" binding:"required"`
	Framework        string `json:"framework" binding:"required"`
}

type RegisterResponse struct {
	Token    string            `json:"token"`
	User     UserResponse      `json:"user"`
	Project  ProjectWithToken  `json:"project"`
	Projects []ProjectResponse `json:"projects"`
}
