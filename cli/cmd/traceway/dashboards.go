package main

import (
	"context"
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func newDashboardsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dashboards", Short: "Inspect organization dashboards applied to the selected project"}
	cmd.AddCommand(newDashboardsValidateCmd(), &cobra.Command{
		Use: "library", Short: "List dashboards and project assignments across accessible organizations", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRaw(cmd, func(ctx context.Context, c *client.Client, _ string) (json.RawMessage, error) {
				return c.DashboardLibrary(ctx)
			}, nil)
		},
	}, &cobra.Command{
		Use: "list", Short: "List dashboards applied to the selected project", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.ListDashboards(ctx, projectID)
			}, nil)
		},
	}, &cobra.Command{
		Use: "export <id>", Short: "Export a dashboard as importable JSON", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "dashboard id")
			if err != nil {
				return err
			}
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.ExportDashboard(ctx, projectID, id)
			}, nil)
		},
	})
	return cmd
}

func newDashboardsValidateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "validate", Short: "Validate dashboard JSON locally without login or writes", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var data json.RawMessage
			if err := readJSONFile(cmd, &data); err != nil {
				return jsonFileUsageError(cmd, err)
			}
			if err := client.ValidateDashboardImport(data, "create"); err != nil {
				return jsonFileUsageError(cmd, err)
			}
			return renderRaw(cmd, output.ResolveMode(flagOutput, output.StdoutIsTerminal()), json.RawMessage(`{"valid":true}`), nil)
		}}
	addJSONFileFlag(cmd)
	return cmd
}
