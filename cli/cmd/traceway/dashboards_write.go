//go:build write

package main

import (
	"context"
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func newDashboardsImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "import", Short: "Create dashboards from JSON and apply them to the selected project", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var data json.RawMessage
			if err := readJSONFile(cmd, &data); err != nil {
				return jsonFileUsageError(cmd, err)
			}
			mode, _ := cmd.Flags().GetString("mode")
			if err := client.ValidateDashboardImport(data, mode); err != nil {
				return jsonFileUsageError(cmd, err)
			}
			summary := "Import dashboards into the selected project's organization and apply them to this project."
			if mode == "upsert" {
				summary += " Matching names (case-insensitive) replace the entire dashboard in EVERY project using it; there is no concurrent-edit check."
			}
			return runWrite(cmd, []string{summary, string(data)}, map[string]any{"action": "import", "message": "Imported dashboards."},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return c.ImportDashboards(ctx, projectID, data, mode)
				})
		},
	}
	addJSONFileFlag(cmd)
	cmd.Flags().String("mode", "create", "create adds copies; upsert replaces dashboards matched by organization-wide name")
	return cmd
}
