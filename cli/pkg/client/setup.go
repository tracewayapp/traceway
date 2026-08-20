package client

import (
	"context"
	"net/http"
)

type SetupSessionProject struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Framework string `json:"framework"`
}

type SetupSession struct {
	OrganizationID   int                   `json:"organizationId"`
	OrganizationName string                `json:"organizationName"`
	Email            string                `json:"email"`
	BackendURL       string                `json:"backendUrl"`
	Projects         []SetupSessionProject `json:"projects"`
}

func (c *Client) GetSetupSession(ctx context.Context) (*SetupSession, error) {
	var session SetupSession
	if err := c.do(ctx, http.MethodGet, "/api/setup/session", nil, &session); err != nil {
		return nil, err
	}
	return &session, nil
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

func (c *Client) SubmitSetupPlan(ctx context.Context, projects []SetupPlanProject) error {
	body := map[string]any{"projects": projects}
	return c.do(ctx, http.MethodPut, "/api/setup/plan", body, nil)
}

type SetupProject struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Framework      string `json:"framework"`
	Token          string `json:"token"`
	SourceMapToken string `json:"sourceMapToken"`
	BackendURL     string `json:"backendUrl"`
	Status         string `json:"status"`
}

type SetupPlanStatus struct {
	Status   string         `json:"status"`
	Reason   string         `json:"reason,omitempty"`
	Projects []SetupProject `json:"projects,omitempty"`
}

func (c *Client) GetSetupPlan(ctx context.Context) (*SetupPlanStatus, error) {
	var status SetupPlanStatus
	if err := c.do(ctx, http.MethodGet, "/api/setup/plan", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}
