package client

import (
	"context"
	"net/http"
)

// Project is the minimal project shape we need today. Add fields as commands
// require them; do not pre-emptively mirror the entire upstream model.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Role is the caller's effective role in the project (owner, admin,
	// user, readonly); a readonly role refuses writes whatever the credential.
	Role string `json:"role,omitempty"`
}

// ListProjects returns all projects visible to the authenticated user.
// Upstream: GET /api/projects → []Project (direct array, no pagination wrapper).
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	if err := c.do(ctx, http.MethodGet, "/api/projects", nil, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}
