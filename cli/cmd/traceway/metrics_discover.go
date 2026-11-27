package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

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
			}, discoverTable)
		},
	}
	addTimeRangeFlags(cmd)
	cmd.Flags().String("name", "", "Metric name for tag-value discovery (requires --key)")
	cmd.Flags().String("key", "", "Tag key for value discovery (requires --name)")
	return cmd
}

// discoverTable renders {"metrics": [...]} as one row per metric and the tag-value answer {"values": [...]} as one
// value per row.
func discoverTable(w io.Writer, v any) error {
	tw := output.NewTabWriter(w)
	if m, ok := v.(map[string]any); ok {
		if values, ok := m["values"].([]any); ok {
			_, _ = fmt.Fprintln(tw, "VALUE")
			for _, value := range values {
				_, _ = fmt.Fprintln(tw, fmt.Sprint(value))
			}
			return tw.Flush()
		}
	}
	_, _ = fmt.Fprintln(tw, "NAME\tTYPE\tUNIT\tTAG KEYS")
	for _, r := range rows(v, "metrics") {
		keys := "-"
		if list, ok := r["tagKeys"].([]any); ok && len(list) > 0 {
			names := make([]string, len(list))
			for i, k := range list {
				names[i] = fmt.Sprint(k)
			}
			keys = strings.Join(names, ", ")
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", cell(r, "name"), cell(r, "metricType"), cell(r, "unit"), keys)
	}
	return tw.Flush()
}
