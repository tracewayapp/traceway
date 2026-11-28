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
	Name      string `json:"name"`
	Framework string `json:"framework"`
	EnvFile   string `json:"envFile,omitempty"`
	// EnvVar holds the project token. EnvFormat "connectionString" is the 1.x form, still accepted from old plans.
	EnvVar    string `json:"envVar,omitempty"`
	EnvFormat string `json:"envFormat,omitempty"`
	// UrlEnvVar names the variable for the server URL, written only for a server other than Traceway Cloud
	// (default: EnvVar with TOKEN replaced by URL).
	UrlEnvVar string `json:"urlEnvVar,omitempty"`
	// UploadTokenEnvFile and UploadTokenEnvVar ask for the upload token (source maps, symbols, mappings, dSYMs).
	UploadTokenEnvFile string               `json:"uploadTokenEnvFile,omitempty"`
	UploadTokenEnvVar  string               `json:"uploadTokenEnvVar,omitempty"`
	Deployment         *SetupPlanDeployment `json:"deployment,omitempty"`
}

type SetupPlanPayload struct {
	Projects []SetupPlanProject `json:"projects"`
}
