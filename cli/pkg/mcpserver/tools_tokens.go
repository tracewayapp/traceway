package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func (s *server) addTokenWriteTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "create_read_token", Description: "Create a read-only personal access token (twp_) for the signed-in user, for example for a host application's backend that issues dashboard links. It reads everything the user can read, never expires unless expires_in_days is given, and only the dashboard (Account > Personal Access Tokens) revokes it. Write tokens cannot be created here. The result carries the token once: hand it to the secret store that needs it (or tell the user how to store it) and never repeat it in chat, commit it or log it. Recorded in the organization's agent activity log. Only create when requested by the user; check list_personal_access_tokens first to avoid duplicates.", OutputSchema: passThroughSchema, Annotations: additive()}, s.createReadToken)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_personal_access_tokens", Description: "List the signed-in user's personal access tokens: id, name, prefix, scope, last use and expiry. The tokens themselves are never returned.", OutputSchema: passThroughSchema, Annotations: readOnly()}, s.listPersonalAccessTokens)
}

type createReadTokenIn struct {
	projectIn
	Name          string `json:"name" jsonschema:"Name shown in Account > Personal Access Tokens, at most 100 characters, such as the application and purpose."`
	ExpiresInDays int    `json:"expires_in_days,omitempty" jsonschema:"Lifetime in whole days, 1 to 3650. Omit for a token that never expires."`
}

func (s *server) createReadToken(ctx context.Context, req *mcp.CallToolRequest, in createReadTokenIn) (*mcp.CallToolResult, any, error) {
	id, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := client.ValidatePersonalAccessToken(in.Name, in.ExpiresInDays); err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	return s.raw(s.client(req).CreateReadToken(ctx, id, in.Name, in.ExpiresInDays))
}

func (s *server) listPersonalAccessTokens(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	return s.raw(s.client(req).ListPersonalAccessTokens(ctx))
}
