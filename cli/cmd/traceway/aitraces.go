package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// aiTracesOrderBy maps the user-facing --order-by values to the server's
// snake_case field names for POST /api/ai-traces/grouped.
var aiTracesOrderBy = map[string]string{
	"count":       "count",
	"p50":         "p50_duration",
	"p95":         "p95_duration",
	"avg":         "avg_duration",
	"totalTokens": "total_tokens",
	"totalCost":   "total_cost",
	"lastSeen":    "last_seen",
}

var aiTracesOrderByValues = []string{"count", "p50", "p95", "avg", "totalTokens", "totalCost", "lastSeen"}

func newAiTracesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai-traces",
		Short: "Inspect AI / LLM traces",
	}
	cmd.AddCommand(newAiTracesListCmd())
	cmd.AddCommand(newAiTracesShowCmd())
	return cmd
}

func newAiTracesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List AI traces grouped by name with token/cost/duration stats",
		Long: `List AI/LLM traces grouped by trace name, with per-name call counts, token
totals, cost, and p50/p95/avg durations — the list behind /ai-traces. The
default order is totalCost, which surfaces the most expensive trace names.

For one specific call, get its id and recordedAt (from a dashboard URL's t=
param or a distributed-trace node) and use "ai-traces show".

--root-filter separates traces that started their own distributed trace (root)
from traces that ran inside another request's trace (non-root).`,
		RunE: runAiTracesList,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("search", "", "Free-text search filter for trace names")
	cmd.Flags().String("order-by", "totalCost", "Sort field (count, p50, p95, avg, totalTokens, totalCost, lastSeen)")
	cmd.Flags().String("sort-direction", "desc", "Sort direction: asc or desc")
	cmd.Flags().String("root-filter", "all", "Trace position filter: all, root, non-root")
	return cmd
}

func runAiTracesList(cmd *cobra.Command, _ []string) error {
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
			paginationHint("traceway ai-traces list"))
	}
	page := resolvePagination(cmd)
	search, _ := cmd.Flags().GetString("search")
	orderBy, _ := cmd.Flags().GetString("order-by")
	if err := validateEnumFlag("--order-by", orderBy, aiTracesOrderByValues); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway ai-traces list", "--order-by", aiTracesOrderByValues))
	}
	sortDir, _ := cmd.Flags().GetString("sort-direction")
	if err := validateEnumFlag("--sort-direction", sortDir, sortDirections); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway ai-traces list", "--sort-direction", sortDirections))
	}
	rootFilter, _ := cmd.Flags().GetString("root-filter")
	if err := validateEnumFlag("--root-filter", rootFilter, rootFilterValues); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway ai-traces list", "--root-filter", rootFilterValues))
	}

	c := sess.Client()
	resp, err := c.ListAiTraces(ctx, sess.ProjectID, client.ListAiTracesRequest{
		TimeRange:     tr,
		Pagination:    page,
		Search:        search,
		OrderBy:       aiTracesOrderBy[orderBy],
		SortDirection: sortDir,
		RootFilter:    rootFilters[rootFilter],
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
		_, _ = fmt.Fprintln(tw, "TRACE\tCOUNT\tP50\tP95\tTOKENS\tCOST\tLAST SEEN")
		for _, t := range resp.Data {
			_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%d\t%.4f\t%s\n",
				t.TraceName, t.Count,
				formatDuration(t.P50Duration),
				formatDuration(t.P95Duration),
				t.TotalTokens, t.TotalCost,
				t.LastSeen.Format("2006-01-02 15:04:05"),
			)
		}
		return tw.Flush()
	}
}

func newAiTracesShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <traceId>",
		Short: "Show one AI trace by id with its conversation",
		Long: `Show a single AI/LLM trace by its UUID, including token/cost stats and the
stored conversation. This is the detail behind /ai-traces/<name>/<traceId>.

--recorded-at is REQUIRED. The ai_traces table is daily-partitioned; the
timestamp bounds the lookup to a window around it so ClickHouse prunes
partitions instead of scanning all of them. Source it from the dashboard URL's
t= param (/ai-traces/<name>/<id>?t=...) or a distributed-trace node's
recordedAt.`,
		Args: cobra.ExactArgs(1),
		RunE: runAiTracesShow,
	}
	addTimestampFlag(cmd, "recorded-at", "Trace timestamp, RFC3339 (required; from the URL t= param)")
	return cmd
}

func runAiTracesShow(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := validateUUIDArg(cmd, mode, args[0], "trace id"); err != nil {
		return err
	}
	recordedAt, err := resolveTimestamp(cmd, "recorded-at")
	if err != nil {
		return renderTimestampError(cmd.ErrOrStderr(), mode, "recorded-at", err)
	}

	c := sess.Client()
	resp, err := c.GetAiTrace(ctx, sess.ProjectID, args[0], recordedAt)
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
		if resp.AiTrace != nil {
			a := resp.AiTrace
			_, _ = fmt.Fprintf(out,
				"ID:           %s\nTRACE:        %s\nRECORDED AT:  %s\nMODEL:        %s\nPROVIDER:     %s\nOPERATION:    %s\nTOKENS:       %d in / %d out / %d total\nCOST:         %.6f\nDURATION:     %s\n",
				a.Id, a.TraceName,
				a.RecordedAt.Format("2006-01-02 15:04:05"),
				pickStr(a.Model, "-"), pickStr(a.Provider, "-"), pickStr(a.Operation, "-"),
				a.InputTokens, a.OutputTokens, a.TotalTokens,
				a.TotalCost, formatDuration(a.Duration),
			)
			if a.DistributedTraceId != nil {
				_, _ = fmt.Fprintf(out, "TRACE ID:     %s\n", a.DistributedTraceId.String())
			}
		}
		if len(resp.Conversation) > 0 {
			_, _ = fmt.Fprintln(out, "\nCONVERSATION available (use --output json to view).")
		}
		return nil
	}
}
