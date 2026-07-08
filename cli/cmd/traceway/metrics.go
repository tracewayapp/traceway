package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// metricAggregations is the canonical list of aggregations the server accepts.
// Kept in lockstep with the --aggregation flag's help text.
var metricAggregations = client.MetricAggregations

func newMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Query metric time series",
	}
	cmd.AddCommand(newMetricsQueryCmd())
	return cmd
}

func newMetricsQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query a single metric over time",
		RunE:  runMetricsQuery,
	}
	addTimeRangeFlags(cmd)
	cmd.Flags().String("name", "", "Metric name (required)")
	cmd.Flags().String("aggregation", "avg", "Aggregation: avg, sum, count, min, max, p50, p95, p99")
	cmd.Flags().StringSlice("tag", nil, "Tag filter as key=value (repeatable)")
	cmd.Flags().String("group-by", "", "Tag to group series by")
	cmd.Flags().Int("interval-minutes", 0, "Time bucket size in minutes (0 = auto)")
	return cmd
}

func runMetricsQuery(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	tr, err := resolveTimeRange(cmd)
	if err != nil {
		return renderTimeRangeError(cmd.ErrOrStderr(), mode, err)
	}

	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "--name is required",
			"traceway metrics query --name <metric-name>")
	}
	agg, _ := cmd.Flags().GetString("aggregation")
	if err := validateEnumFlag("--aggregation", agg, metricAggregations); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway metrics query", "--aggregation", metricAggregations))
	}
	groupBy, _ := cmd.Flags().GetString("group-by")
	intervalMin, _ := cmd.Flags().GetInt("interval-minutes")
	tags, _ := cmd.Flags().GetStringSlice("tag")

	tagFilters, err := parseTagFilters(tags)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			"use --tag key=value (repeatable)")
	}

	c := sess.Client()
	resp, err := c.QueryMetrics(ctx, sess.ProjectID, client.QueryMetricsRequest{
		TimeRange:       tr,
		IntervalMinutes: intervalMin,
		Queries: []client.MetricQueryItem{
			{Name: name, Aggregation: agg, TagFilters: tagFilters, GroupBy: groupBy},
		},
	})
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		// Summary table — for actual time-series data, --output json is recommended.
		tw := output.NewTabWriter(cmd.OutOrStdout())
		_, _ = fmt.Fprintln(tw, "METRIC\tUNIT\tGROUP\tPOINTS\tLATEST")
		for _, r := range resp.Results {
			if len(r.Series) == 0 {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t-\t0\t-\n", r.Name, pickStr(r.Unit, "-"))
				continue
			}
			for group, pts := range r.Series {
				latest := "-"
				if len(pts) > 0 {
					latest = fmt.Sprintf("%g", pts[len(pts)-1].Value)
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", r.Name, pickStr(r.Unit, "-"), group, len(pts), latest)
			}
		}
		return tw.Flush()
	}
}

// parseTagFilters parses ["k=v", "x=y"] into {"k":"v","x":"y"}. Returns an
// error if any element is malformed.
func parseTagFilters(in []string) (map[string]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(in))
	for _, item := range in {
		k, v, ok := strings.Cut(item, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --tag %q: expected key=value", item)
		}
		out[k] = v
	}
	return out, nil
}
