package models

import "time"

type AgentWriteEvent struct {
	Id             string    `json:"id"`
	OrganizationId int       `json:"organizationId"`
	ProjectId      string    `json:"projectId"`
	ProjectName    string    `json:"projectName"`
	UserId         int       `json:"userId"`
	UserEmail      string    `json:"userEmail"`
	CredentialKind string    `json:"credentialKind"`
	CredentialRef  string    `json:"credentialRef"`
	CredentialName string    `json:"credentialName"`
	Method         string    `json:"method"`
	Route          string    `json:"route"`
	Path           string    `json:"path"`
	StatusCode     int       `json:"statusCode"`
	CreatedAt      time.Time `json:"createdAt"`
}

type UpdateAgentAccessRequest struct {
	AgentWriteEnabled *bool `json:"agentWriteEnabled"`
}
