package main

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

var endpointsOrderBy = client.EndpointsOrderByValues

var endpointChartMetricTypes = client.EndpointChartMetricTypes

func newEndpointsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "Query HTTP endpoint performance",
	}
	cmd.AddCommand(newEndpointsListCmd())
	cmd.AddCommand(newEndpointsChartCmd())
	cmd.AddCommand(newEndpointsSlowCmd())
	cmd.AddCommand(newEndpointsShowCmd())
	return cmd
}

func newEndpointsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List endpoints with p50/p95/p99 latency stats",
		RunE:  runEndpointsList,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("search", "", "Free-text search filter for endpoint names")
	cmd.Flags().String("order-by", "impact", "Sort field (impact, count, p95, lastSeen)")
	cmd.Flags().String("sort-direction", "desc", "Sort direction: asc or desc")
	return cmd
}

func runEndpointsList(cmd *cobra.Command, _ []string) error {
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
	if err := validatePaginationFlags(cmd); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			paginationHint("traceway endpoints list"))
	}
	page := resolvePagination(cmd)
	search, _ := cmd.Flags().GetString("search")
	orderBy, _ := cmd.Flags().GetString("order-by")
	if err := validateEnumFlag("--order-by", orderBy, endpointsOrderBy); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway endpoints list", "--order-by", endpointsOrderBy))
	}
	sortDir, _ := cmd.Flags().GetString("sort-direction")
	if err := validateEnumFlag("--sort-direction", sortDir, sortDirections); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway endpoints list", "--sort-direction", sortDirections))
	}

	c := sess.Client()
	resp, err := c.ListEndpoints(ctx, sess.ProjectID, client.ListEndpointsRequest{
		TimeRange:     tr,
		Pagination:    page,
		Search:        search,
		OrderBy:       orderBy,
		SortDirection: sortDir,
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
		tw := output.NewTabWriter(cmd.OutOrStdout())
		_, _ = fmt.Fprintln(tw, "ENDPOINT\tCOUNT\tP50\tP95\tP99\tIMPACT\tLAST SEEN")
		for _, e := range resp.Data {
			_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%.2f\t%s\n",
				e.Endpoint, e.Count,
				formatDuration(e.P50Duration),
				formatDuration(e.P95Duration),
				formatDuration(e.P99Duration),
				e.Impact,
				e.LastSeen.Format("2006-01-02 15:04:05"),
			)
		}
		return tw.Flush()
	}
}

func newEndpointsChartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chart",
		Short: "Endpoint latency bucketed over time (find when latency changed)",
		Long: `Show endpoint latency as a time series, the curve behind /endpoints in the
dashboard. The server returns the top 5 endpoints ranked by --metric-type plus
an "Other" bucket, each bucketed by --interval-minutes, so you can read down the
buckets to find when a latency jump started.

This always returns the top endpoints by the chosen metric; it does not filter
to one route. To track a specific endpoint that is not in the top 5, bisect
"endpoints list --search <name>" over narrowing --from/--to windows instead.

Values are in milliseconds. Use --output json for the full per-bucket series.`,
		RunE: runEndpointsChart,
	}
	addTimeRangeFlags(cmd)
	cmd.Flags().String("metric-type", "p95", "Metric to chart: total_time, p50, p95, p99")
	cmd.Flags().Int("interval-minutes", 0, "Time bucket size in minutes (0 = server default of 5)")
	return cmd
}

func runEndpointsChart(cmd *cobra.Command, _ []string) error {
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
	metricType, _ := cmd.Flags().GetString("metric-type")
	if err := validateEnumFlag("--metric-type", metricType, endpointChartMetricTypes); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway endpoints chart", "--metric-type", endpointChartMetricTypes))
	}
	intervalMin, _ := cmd.Flags().GetInt("interval-minutes")

	c := sess.Client()
	resp, err := c.GetEndpointChart(ctx, sess.ProjectID, client.EndpointChartRequest{
		TimeRange:       tr,
		MetricType:      metricType,
		IntervalMinutes: intervalMin,
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
		return renderEndpointChartTable(cmd.OutOrStdout(), resp)
	}
}

// renderEndpointChartTable prints a per-endpoint summary of the time series
// (point count and min/max/latest value in ms), in the server's ranked order.
// The full per-bucket series is only in --output json.
func renderEndpointChartTable(out io.Writer, resp *client.EndpointChartResponse) error {
	type seriesAgg struct {
		points     int
		min, max   float64
		latest     float64
		latestTime time.Time
		seen       bool
	}
	byEndpoint := make(map[string]*seriesAgg, len(resp.Endpoints))
	for _, p := range resp.Series {
		a := byEndpoint[p.Endpoint]
		if a == nil {
			a = &seriesAgg{min: p.Value, max: p.Value}
			byEndpoint[p.Endpoint] = a
		}
		a.points++
		if p.Value < a.min {
			a.min = p.Value
		}
		if p.Value > a.max {
			a.max = p.Value
		}
		if !a.seen || p.Timestamp.After(a.latestTime) {
			a.latest = p.Value
			a.latestTime = p.Timestamp
			a.seen = true
		}
	}

	tw := output.NewTabWriter(out)
	_, _ = fmt.Fprintln(tw, "ENDPOINT\tPOINTS\tMIN(ms)\tMAX(ms)\tLATEST(ms)")
	for _, name := range resp.Endpoints {
		a := byEndpoint[name]
		if a == nil {
			_, _ = fmt.Fprintf(tw, "%s\t0\t-\t-\t-\n", name)
			continue
		}
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\t%.1f\n", name, a.points, a.min, a.max, a.latest)
	}
	return tw.Flush()
}

func newEndpointsSlowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slow <endpoint>",
		Short: "Show the user-configured slow allowance for one endpoint",
		Long: `Report whether an endpoint has been marked slow by an operator, and by how
much. The allowance (offsetMs) is added to the apdex and impact thresholds for
that endpoint server-side, so latency up to the offset is an accepted baseline
rather than a regression. The reason is the operator's note.

An endpoint that was never marked slow returns offsetMs 0 and an empty reason.

The offset shifts apdex/impact only; the raw p50/p95/p99 from "endpoints list"
and "endpoints chart" are NOT adjusted by it. Pass the full route name, e.g.
"GET /api/users/:id".`,
		Args: cobra.ExactArgs(1),
		RunE: runEndpointsSlow,
	}
	return cmd
}

func runEndpointsSlow(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}

	c := sess.Client()
	resp, err := c.GetSlowEndpoint(ctx, sess.ProjectID, args[0])
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
		if resp.OffsetMs == 0 && resp.Reason == "" {
			_, _ = fmt.Fprintf(out, "ENDPOINT:   %s\nMARKED:     no (default thresholds)\n", args[0])
			return nil
		}
		_, _ = fmt.Fprintf(out, "ENDPOINT:   %s\nMARKED:     yes\nALLOWANCE:  %dms\nREASON:     %s\n",
			args[0], resp.OffsetMs, pickStr(resp.Reason, "-"))
		return nil
	}
}

func newEndpointsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <endpointId>",
		Short: "Show one request (transaction) by id with spans and linked errors",
		Long: `Show a single endpoint request (transaction) by its UUID, with its span
waterfall and any linked exception/messages.

This is the per-request detail behind /endpoints/<name>/<endpointId>. For grouped
p50/p95/p99 stats by route, use "endpoints list --search <name>" instead.

--recorded-at is REQUIRED. The endpoints table is daily-partitioned; the
timestamp bounds the lookup to a window around it so ClickHouse prunes
partitions instead of scanning all of them. Source it from the dashboard URL's
t= param (/endpoints/<name>/<id>?t=...) or a distributed-trace node's
recordedAt.`,
		Args: cobra.ExactArgs(1),
		RunE: runEndpointsShow,
	}
	addTimestampFlag(cmd, "recorded-at", "Transaction timestamp, RFC3339 (required; from the URL t= param)")
	return cmd
}

func runEndpointsShow(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := validateUUIDArg(cmd, mode, args[0], "endpoint id"); err != nil {
		return err
	}
	recordedAt, err := resolveTimestamp(cmd, "recorded-at")
	if err != nil {
		return renderTimestampError(cmd.ErrOrStderr(), mode, "recorded-at", err)
	}

	c := sess.Client()
	resp, err := c.GetEndpoint(ctx, sess.ProjectID, args[0], recordedAt)
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
		if resp.Endpoint != nil {
			e := resp.Endpoint
			_, _ = fmt.Fprintf(out,
				"ID:           %s\nENDPOINT:     %s\nRECORDED AT:  %s\nDURATION:     %s\nSTATUS:       %d\nSERVER:       %s\nAPP VERSION:  %s\n",
				e.Id, e.Endpoint,
				e.RecordedAt.Format("2006-01-02 15:04:05"),
				formatDuration(e.Duration), e.StatusCode,
				pickStr(e.ServerName, "-"), pickStr(e.AppVersion, "-"),
			)
			if e.DistributedTraceId != nil {
				_, _ = fmt.Fprintf(out, "TRACE ID:     %s\n", e.DistributedTraceId.String())
			}
		}
		renderSpansTable(out, resp.Spans)
		renderLinkedErrors(out, resp.Exception, resp.Messages)
		return nil
	}
}

// renderSpansTable prints a span waterfall (name + duration), or a count line
// when there are none. Shared by endpoints show and tasks show.
func renderSpansTable(out io.Writer, spans []client.Span) {
	if len(spans) == 0 {
		_, _ = fmt.Fprintln(out, "\nSPANS: none")
		return
	}
	_, _ = fmt.Fprintf(out, "\nSPANS (%d):\n", len(spans))
	tw := output.NewTabWriter(out)
	_, _ = fmt.Fprintln(tw, "NAME\tSTART\tDURATION")
	for _, s := range spans {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n",
			s.Name, s.StartTime.Format("15:04:05.000"), formatDuration(s.Duration))
	}
	_ = tw.Flush()
}

// renderLinkedErrors prints the exception and message summaries attached to an
// endpoint or task detail. Shared by endpoints show and tasks show.
func renderLinkedErrors(out io.Writer, exc *client.LinkedException, messages []client.LinkedMessage) {
	if exc != nil {
		_, _ = fmt.Fprintf(out, "\nLINKED EXCEPTION (%s):\n%s\n", exc.ExceptionHash, firstLine(exc.StackTrace))
	}
	if len(messages) > 0 {
		_, _ = fmt.Fprintf(out, "\nMESSAGES (%d):\n", len(messages))
		for _, m := range messages {
			_, _ = fmt.Fprintf(out, "  - %s\n", firstLine(m.StackTrace))
		}
	}
}

// formatDuration renders a Duration as a human-readable string.
// time.Duration's String() does this already (e.g. "50ms"); we just shorten
// for very small values.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	return d.String()
}
