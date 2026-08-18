package client

import (
	"context"
	"net/http"
)

// SetupSessionProject is a project as listed by the setup session endpoint
// (names only; the session never exposes ingest tokens).
type SetupSessionProject struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Framework string `json:"framework"`
}

// SetupSession describes the organization a setup token is scoped to.
type SetupSession struct {
	OrganizationID   int                   `json:"organizationId"`
	OrganizationName string                `json:"organizationName"`
	Email            string                `json:"email"`
	BackendURL       string                `json:"backendUrl"`
	Projects         []SetupSessionProject `json:"projects"`
}

// GetSetupSession validates a setup token and describes its organization.
// Upstream: GET /api/setup/session (bearer tws_ setup token).
func (c *Client) GetSetupSession(ctx context.Context) (*SetupSession, error) {
	var session SetupSession
	if err := c.do(ctx, http.MethodGet, "/api/setup/session", nil, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// SetupPlanDeployment carries hosting placement info for a credential that
// cannot be hardcoded locally. Instructions may contain the literal <token>
// placeholder; only the Traceway website substitutes the real value.
type SetupPlanDeployment struct {
	Platform     string `json:"platform"`
	Instructions string `json:"instructions"`
}

// SetupPlanProject is one project row of a submitted setup plan.
type SetupPlanProject struct {
	Name       string               `json:"name"`
	Framework  string               `json:"framework"`
	EnvFile    string               `json:"envFile,omitempty"`
	EnvVar     string               `json:"envVar,omitempty"`
	EnvFormat  string               `json:"envFormat,omitempty"`
	Deployment *SetupPlanDeployment `json:"deployment,omitempty"`
}

// SubmitSetupPlan submits (or replaces) the setup token's draft plan for the
// user to approve in the dashboard.
// Upstream: PUT /api/setup/plan (bearer tws_ setup token).
func (c *Client) SubmitSetupPlan(ctx context.Context, projects []SetupPlanProject) error {
	body := map[string]any{"projects": projects}
	return c.do(ctx, http.MethodPut, "/api/setup/plan", body, nil)
}

// SetupProject is a provisioned project as returned once a plan is approved.
type SetupProject struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Framework      string `json:"framework"`
	Token          string `json:"token"`
	SourceMapToken string `json:"sourceMapToken"`
	BackendURL     string `json:"backendUrl"`
	Status         string `json:"status"` // "created" | "existing"
}

// SetupPlanStatus is the state of the setup token's latest plan.
type SetupPlanStatus struct {
	Status   string         `json:"status"` // "none" | "pending" | "approved" | "rejected"
	Reason   string         `json:"reason,omitempty"`
	Projects []SetupProject `json:"projects,omitempty"`
}

// GetSetupPlan reports the latest plan's status; once approved it carries the
// provisioned projects including their ingest tokens.
// Upstream: GET /api/setup/plan (bearer tws_ setup token).
func (c *Client) GetSetupPlan(ctx context.Context) (*SetupPlanStatus, error) {
	var status SetupPlanStatus
	if err := c.do(ctx, http.MethodGet, "/api/setup/plan", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}
