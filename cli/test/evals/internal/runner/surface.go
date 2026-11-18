package runner

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

// prepareSurface mints the arm's credential and connects its surface: the
// CLI logged in with a PAT (XDG dirs outside the workspace), or an MCP
// session to /mcp or /mcp/write.
func prepareSurface(ctx context.Context, opt Options, c *catalog.Case, arm string, inst *env.Instance, projects []env.Project, primary env.Project, dir, work string) (surface, func(), error) {
	s := surface{submit: submitFor(c.Suite)}
	switch arm {
	case catalog.ArmCLISkill:
		s.skills = []string{"traceway"}
	case catalog.ArmCLIWriteSkill:
		s.skills = []string{"traceway-write", "traceway"}
	case catalog.ArmCLISetupSkill:
		s.skills = []string{"traceway-setup"}
		s.setupShell = true
		s.cliName, s.cliBin = "traceway", opt.Bins.CLI
		return s, nil, nil
	}

	scope := "read"
	if catalog.IsWriteArm(arm) {
		scope = "write"
	}
	if c.Credential.Scope != "" {
		scope = c.Credential.Scope
	}
	jwt := inst.Admin.JWT
	if m := c.Credential.Member; m != "" && m != "admin" && m != "owner" {
		member, err := env.AddMember(ctx, inst.Backend, inst.Admin, m+"@example.com", m)
		if err != nil {
			return s, nil, fmt.Errorf("add %s member: %w", m, err)
		}
		jwt = member.JWT
	}
	if c.Suite == "write" || catalog.IsWriteArm(arm) {
		on := c.AgentAccess == nil || *c.AgentAccess
		if err := env.SetAgentAccess(ctx, inst.Backend, inst.Admin, on); err != nil {
			return s, nil, err
		}
	}
	days := 0
	if scope == "write" {
		days = 1
	}
	pat, err := env.CreatePAT(ctx, inst.Backend, jwt, "eval agent", scope, days)
	if err != nil {
		return s, nil, err
	}

	if catalog.IsMCPArm(arm) {
		path := "/mcp"
		if scope == "write" {
			path = "/mcp/write"
		}
		b, err := tools.ConnectMCP(ctx, inst.Backend.URL+path, pat, primary.ID)
		if err != nil {
			return s, nil, err
		}
		s.mcp = b
		return s, b.Close, nil
	}
	s.cliName, s.cliBin = "traceway", opt.Bins.CLI
	if scope == "write" {
		s.cliName, s.cliBin = "traceway-write", opt.Bins.CLIWrite
	}
	home, err := env.NewCLIHome(filepath.Join(dir, "cli"), s.cliBin)
	if err != nil {
		return s, nil, err
	}
	if err := home.Login(ctx, inst.Backend.URL, pat, primary.ID); err != nil {
		return s, nil, err
	}
	s.cliHome = home
	return s, nil, nil
}
