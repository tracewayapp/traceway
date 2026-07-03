package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type emptyIn struct{}

type listProjectsResult struct {
	Projects         []projectOut `json:"projects"`
	DefaultProjectID string       `json:"defaultProjectId,omitempty"`
}

type projectOut struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *server) listProjects(ctx context.Context, req *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	projects, err := s.client(req).ListProjects(ctx)
	if err != nil {
		return nil, nil, s.apiErr(err)
	}
	out := listProjectsResult{Projects: make([]projectOut, 0, len(projects)), DefaultProjectID: s.cfg.DefaultProjectID}
	for _, p := range projects {
		out.Projects = append(out.Projects, projectOut{ID: p.ID, Name: p.Name})
	}
	return nil, out, nil
}
