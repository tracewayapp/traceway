package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func newMetricsDiscoverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "discover", Short: "Discover recorded metric names, types, units and tag keys, or values for one tag", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, _ := cmd.Flags().GetString("name")
			key, _ := cmd.Flags().GetString("key")
			if (name == "") != (key == "") {
				return jsonFileUsageError(cmd, fmt.Errorf("pass both --name and --key to discover tag values"))
			}
			tr, err := resolveTimeRange(cmd)
			if err != nil {
				return renderTimeRangeError(cmd.ErrOrStderr(), output.ResolveMode(flagOutput, output.StdoutIsTerminal()), err)
			}
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.DiscoverMetrics(ctx, projectID, tr, name, key)
			}, nil)
		},
	}
	addTimeRangeFlags(cmd)
	cmd.Flags().String("name", "", "Metric name for tag-value discovery (requires --key)")
	cmd.Flags().String("key", "", "Tag key for value discovery (requires --name)")
	return cmd
}
