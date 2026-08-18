package models

import "time"

type CreateSetupTokenRequest struct {
	OrganizationId int `json:"organizationId" binding:"required"`
}

type CreateSetupTokenResponse struct {
	Token          string    `json:"token"`
	Prefix         string    `json:"prefix"`
	ExpiresAt      time.Time `json:"expiresAt"`
	OrganizationId int       `json:"organizationId"`
	BackendUrl     string    `json:"backendUrl"`
}

type SetupPlanDeployment struct {
	Platform     string `json:"platform"`
	Instructions string `json:"instructions"`
}

type SetupPlanProject struct {
	Name       string               `json:"name"`
	Framework  string               `json:"framework"`
	EnvFile    string               `json:"envFile,omitempty"`
	EnvVar     string               `json:"envVar,omitempty"`
	EnvFormat  string               `json:"envFormat,omitempty"`
	Deployment *SetupPlanDeployment `json:"deployment,omitempty"`
}

type SetupPlanPayload struct {
	Projects []SetupPlanProject `json:"projects"`
}
