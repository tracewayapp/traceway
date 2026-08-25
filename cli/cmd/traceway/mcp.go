package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/pkg/client"
	"github.com/tracewayapp/traceway/cli/pkg/mcpserver"
)

func newMcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the Traceway MCP server on stdio",
		Long: `Serve the Traceway MCP server over stdio for MCP clients like Claude Code:

  claude mcp add traceway -- traceway mcp

Credentials come from the CLI session (run "traceway login" first) and the
default project from "traceway projects use". Both can be overridden per call
via each tool's project_id parameter.

Headless environments can skip the CLI session entirely by setting
TRACEWAY_URL and TRACEWAY_TOKEN (a personal access token or JWT), plus an
optional TRACEWAY_PROJECT default project id.`,
		Args: cobra.NoArgs,
		RunE: runMcp,
	}
}

// runMcp builds the authenticated client and serves MCP on stdio. Everything
// written to stdout must be JSON-RPC, so all diagnostics go to stderr.
func runMcp(cmd *cobra.Command, _ []string) error {
	cfg, err := resolveMcpConfig(cmd.ErrOrStderr())
	if err != nil {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "traceway mcp:", err)
		return newCLIError(exitcode.Auth, "not_authenticated")
	}

	srv := mcpserver.New(cfg)
	err = srv.Run(cmd.Context(), &mcp.StdioTransport{})
	if err != nil && !errors.Is(err, cmd.Context().Err()) && !isClientDisconnect(err) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "traceway mcp:", err)
		return newCLIError(exitcode.Generic, "mcp_server_failed")
	}
	return nil
}

// isClientDisconnect reports whether the server stopped because the client
// closed stdin, the normal way a stdio MCP server exits. The SDK types most
// disconnects (io.EOF or mcp.ErrConnectionClosed wraps), but its jsonrpc2
// shutdown path stringifies the EOF behind an internal sentinel as
// "server is closing: EOF", so that one shape is matched textually. Never
// swallow an io.ErrUnexpectedEOF: a truncated frame is a real failure.
func isClientDisconnect(err error) bool {
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, mcp.ErrConnectionClosed) {
		return true
	}
	msg := err.Error()
	if msg == io.EOF.Error() {
		return true
	}
	return strings.HasPrefix(msg, "server is closing") && strings.HasSuffix(msg, ": "+io.EOF.Error())
}

// resolveMcpConfig picks the credential source: an explicit --profile always
// wins, then the TRACEWAY_URL/TRACEWAY_TOKEN env override (headless/CI), then
// the CLI session. A missing login fails fast with an actionable message: a
// stdio MCP server has no TTY to run the interactive login on.
func resolveMcpConfig(errOut io.Writer) (mcpserver.Config, error) {
	url, token := os.Getenv("TRACEWAY_URL"), os.Getenv("TRACEWAY_TOKEN")
	if url != "" || token != "" {
		if flagProfile != "" {
			_, _ = fmt.Fprintln(errOut, "traceway mcp: ignoring TRACEWAY_URL/TRACEWAY_TOKEN because --profile was given")
		} else {
			if url == "" || token == "" {
				return mcpserver.Config{}, errors.New("TRACEWAY_URL and TRACEWAY_TOKEN must be set together")
			}
			projectID := flagProject
			if projectID == "" {
				projectID = os.Getenv("TRACEWAY_PROJECT")
			}
			return mcpserver.Config{
				Client:           client.New(url, client.WithJWT(token)),
				DefaultProjectID: projectID,
				InstanceURL:      url,
				Version:          version,
				AuthHint:         "the TRACEWAY_TOKEN this server was started with is expired or revoked; rotate it and restart the MCP server",
			}, nil
		}
	}

	sess, err := loadSessionOpts(false)
	if err != nil {
		return mcpserver.Config{}, fmt.Errorf("not logged in (%w): run 'traceway login --url https://<instance>' in a terminal, then reconnect this MCP server", err)
	}
	return mcpserver.Config{
		Client:           sess.Client(),
		DefaultProjectID: sess.ProjectID,
		InstanceURL:      sess.URL,
		Version:          version,
	}, nil
}
