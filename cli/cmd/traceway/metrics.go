package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// metricAggregations is the canonical list of aggregations the server accepts.
// Kept in lockstep with the --aggregation flag's help text.
var metricAggregations = []string{"avg", "sum", "count", "min", "max", "p50", "p95", "p99"}

func newMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Query metric time series",
	}
	cmd.AddCommand(newMetricsListCmd())
	cmd.AddCommand(newMetricsTagsCmd())
	cmd.AddCommand(newMetricsQueryCmd())
	return cmd
}

func newMetricsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Discover metric names, tag keys, and units",
		Long: `List the metric names that received data points in the window, with the tag
keys observed on each and the type/unit from the project's metric registry
(when set). Use this before "metrics query" instead of guessing names.

Unlike other commands the default window is the server's: the last 7 days.
Pass --since/--from/--to to narrow it.`,
		RunE: runMetricsList,
	}
	addTimeRangeFlags(cmd)
	cmd.Flags().String("search", "", "Substring filter on metric names (applied client-side)")
	return cmd
}

func runMetricsList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	// Zero TimeRange = let the server default to 7d, which suits discovery
	// better than the CLI's usual 1h.
	var tr client.TimeRange
	if cmd.Flags().Changed("since") || cmd.Flags().Changed("from") || cmd.Flags().Changed("to") {
		tr, err = resolveTimeRange(cmd)
		if err != nil {
			return renderTimeRangeError(cmd.ErrOrStderr(), mode, err)
		}
	}
	search, _ := cmd.Flags().GetString("search")

	c := sess.Client()
	resp, err := c.DiscoverMetrics(ctx, sess.ProjectID, tr)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}
	if search != "" {
		filtered := resp.Metrics[:0]
		for _, m := range resp.Metrics {
			if strings.Contains(m.Name, search) {
				filtered = append(filtered, m)
			}
		}
		resp.Metrics = filtered
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		tw := output.NewTabWriter(cmd.OutOrStdout())
		_, _ = fmt.Fprintln(tw, "NAME\tTYPE\tUNIT\tTAG KEYS")
		for _, m := range resp.Metrics {
			tagKeys := "-"
			if len(m.TagKeys) > 0 {
				tagKeys = strings.Join(m.TagKeys, ",")
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				m.Name, pickStr(m.MetricType, "-"), pickStr(m.Unit, "-"), tagKeys)
		}
		return tw.Flush()
	}
}

func newMetricsTagsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags <metric-name> [<tag-key>]",
		Short: "Discover a metric's tag keys, or the values of one tag key",
		Long: `With one argument, list the tag keys observed on the metric. With a tag key as
the second argument, list the values observed for it — ready to plug into
"metrics query --tag key=value" or "--group-by key".

Both forms scan the server's discovery window (the last 7 days).`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runMetricsTags,
	}
	return cmd
}

func runMetricsTags(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	c := sess.Client()
	name := args[0]

	if len(args) == 2 {
		resp, err := c.DiscoverMetricTagValues(ctx, sess.ProjectID, name, args[1])
		if err != nil {
			return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
		}
		switch mode {
		case output.ModeJSON:
			return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
		case output.ModeYAML:
			return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
		default:
			out := cmd.OutOrStdout()
			for _, v := range resp.Values {
				_, _ = fmt.Fprintln(out, v)
			}
			return nil
		}
	}

	resp, err := c.DiscoverMetrics(ctx, sess.ProjectID, client.TimeRange{})
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}
	for _, m := range resp.Metrics {
		if m.Name != name {
			continue
		}
		keys := struct {
			Name    string   `json:"name"`
			TagKeys []string `json:"tagKeys"`
		}{m.Name, m.TagKeys}
		switch mode {
		case output.ModeJSON:
			return output.RenderJSON(cmd.OutOrStdout(), keys, output.ParseFieldsFlag(flagFields))
		case output.ModeYAML:
			return output.RenderYAML(cmd.OutOrStdout(), keys, output.ParseFieldsFlag(flagFields))
		default:
			out := cmd.OutOrStdout()
			for _, k := range m.TagKeys {
				_, _ = fmt.Fprintln(out, k)
			}
			return nil
		}
	}
	_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
		Code:     "not_found",
		Message:  fmt.Sprintf("metric %q not seen in the discovery window", name),
		Hint:     "traceway metrics list",
		ExitCode: exitcode.NotFound,
	})
	return newCLIError(exitcode.NotFound, "not_found")
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
