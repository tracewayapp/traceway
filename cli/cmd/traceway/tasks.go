package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// tasksListOrderBy maps the user-facing --order-by values to the server's
// snake_case field names for POST /api/tasks/grouped.
var tasksListOrderBy = map[string]string{
	"impact":   "impact",
	"count":    "count",
	"p50":      "p50_duration",
	"p95":      "p95_duration",
	"avg":      "avg_duration",
	"lastSeen": "last_seen",
}

var tasksListOrderByValues = []string{"impact", "count", "p50", "p95", "avg", "lastSeen"}

// tasksRunsOrderBy maps the user-facing --order-by values for the run-level
// endpoints (POST /api/tasks and /api/tasks/task).
var tasksRunsOrderBy = map[string]string{
	"recordedAt": "recorded_at",
	"duration":   "duration",
}

var tasksRunsOrderByValues = []string{"recordedAt", "duration"}

// rootFilters maps the user-facing --root-filter values to the server's
// rootFilter body field. Shared by tasks list and ai-traces list.
var rootFilters = map[string]string{
	"all":      "",
	"root":     "root",
	"non-root": "non_root",
}

var rootFilterValues = []string{"all", "root", "non-root"}

func newTasksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Query background task runs",
	}
	cmd.AddCommand(newTasksListCmd())
	cmd.AddCommand(newTasksRunsCmd())
	cmd.AddCommand(newTasksShowCmd())
	return cmd
}

func newTasksListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List background tasks grouped by name with duration stats",
		Long: `List background tasks grouped by task name, with per-name run counts and
p50/p95/avg durations — the list behind /tasks in the dashboard and the tasks
analog of "endpoints list".

The default order is impact (count times p95-p50 spread), which surfaces tasks
that are both busy and erratic. Use "tasks runs --task <name>" to drill into
one name's individual runs.

--root-filter separates tasks that started their own distributed trace (root)
from tasks that ran inside another request's trace (non-root).`,
		RunE: runTasksList,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("search", "", "Free-text search filter for task names")
	cmd.Flags().String("order-by", "impact", "Sort field (impact, count, p50, p95, avg, lastSeen)")
	cmd.Flags().String("sort-direction", "desc", "Sort direction: asc or desc")
	cmd.Flags().String("root-filter", "all", "Trace position filter: all, root, non-root")
	return cmd
}

func runTasksList(cmd *cobra.Command, _ []string) error {
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
			paginationHint("traceway tasks list"))
	}
	page := resolvePagination(cmd)
	search, _ := cmd.Flags().GetString("search")
	orderBy, _ := cmd.Flags().GetString("order-by")
	if err := validateEnumFlag("--order-by", orderBy, tasksListOrderByValues); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway tasks list", "--order-by", tasksListOrderByValues))
	}
	sortDir, _ := cmd.Flags().GetString("sort-direction")
	if err := validateEnumFlag("--sort-direction", sortDir, sortDirections); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway tasks list", "--sort-direction", sortDirections))
	}
	rootFilter, _ := cmd.Flags().GetString("root-filter")
	if err := validateEnumFlag("--root-filter", rootFilter, rootFilterValues); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway tasks list", "--root-filter", rootFilterValues))
	}

	c := sess.Client()
	resp, err := c.ListTasks(ctx, sess.ProjectID, client.ListTasksRequest{
		TimeRange:     tr,
		Pagination:    page,
		Search:        search,
		OrderBy:       tasksListOrderBy[orderBy],
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
		_, _ = fmt.Fprintln(tw, "TASK\tCOUNT\tP50\tP95\tAVG\tLAST SEEN")
		for _, t := range resp.Data {
			_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n",
				t.TaskName, t.Count,
				formatDuration(t.P50Duration),
				formatDuration(t.P95Duration),
				formatDuration(t.AvgDuration),
				t.LastSeen.Format("2006-01-02 15:04:05"),
			)
		}
		return tw.Flush()
	}
}

func newTasksRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List individual task runs, optionally scoped to one task name",
		Long: `List individual background task runs, newest first. With --task the list is
scoped to one task name and includes its aggregate stats (count, avg/median/
p95/p99 in ms, throughput per minute) — the view behind /tasks/<name>.

Each run's id and recordedAt feed the by-id detail lookup:
"tasks show <id> --recorded-at <recordedAt>".

Without --task the server ignores --sort-direction (always desc).`,
		RunE: runTasksRuns,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("task", "", "Scope to one task name (also returns aggregate stats)")
	cmd.Flags().String("order-by", "recordedAt", "Sort field (recordedAt, duration)")
	cmd.Flags().String("sort-direction", "desc", "Sort direction: asc or desc (needs --task)")
	return cmd
}

func runTasksRuns(cmd *cobra.Command, _ []string) error {
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
			paginationHint("traceway tasks runs"))
	}
	page := resolvePagination(cmd)
	taskName, _ := cmd.Flags().GetString("task")
	orderBy, _ := cmd.Flags().GetString("order-by")
	if err := validateEnumFlag("--order-by", orderBy, tasksRunsOrderByValues); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway tasks runs", "--order-by", tasksRunsOrderByValues))
	}
	sortDir, _ := cmd.Flags().GetString("sort-direction")
	if err := validateEnumFlag("--sort-direction", sortDir, sortDirections); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway tasks runs", "--sort-direction", sortDirections))
	}

	c := sess.Client()
	resp, err := c.ListTaskRuns(ctx, sess.ProjectID, taskName, client.ListTaskRunsRequest{
		TimeRange:     tr,
		Pagination:    page,
		OrderBy:       tasksRunsOrderBy[orderBy],
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
		out := cmd.OutOrStdout()
		if resp.Stats != nil {
			s := resp.Stats
			_, _ = fmt.Fprintf(out,
				"COUNT:       %d\nAVG:         %.1fms\nMEDIAN:      %.1fms\nP95:         %.1fms\nP99:         %.1fms\nTHROUGHPUT:  %.2f/min\n\n",
				s.Count, s.AvgDuration, s.MedianDuration, s.P95Duration, s.P99Duration, s.Throughput)
		}
		tw := output.NewTabWriter(out)
		_, _ = fmt.Fprintln(tw, "ID\tTASK\tDURATION\tRECORDED AT")
		for _, t := range resp.Data {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				t.Id, t.TaskName, formatDuration(t.Duration),
				t.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
			)
		}
		return tw.Flush()
	}
}

func newTasksShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <taskId>",
		Short: "Show one background task run by id with spans and linked errors",
		Long: `Show a single background task run by its UUID, with its span waterfall and any
linked exception/messages. This is the detail behind /tasks/<name>/<taskId>.

--recorded-at is REQUIRED. The tasks table is daily-partitioned; the timestamp
bounds the lookup to a window around it so ClickHouse prunes partitions instead
of scanning all of them. Source it from the dashboard URL's t= param
(/tasks/<name>/<id>?t=...), a "tasks runs" row, or a distributed-trace node's
recordedAt.`,
		Args: cobra.ExactArgs(1),
		RunE: runTasksShow,
	}
	addTimestampFlag(cmd, "recorded-at", "Task timestamp, RFC3339 (required; from the URL t= param)")
	return cmd
}

func runTasksShow(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := validateUUIDArg(cmd, mode, args[0], "task id"); err != nil {
		return err
	}
	recordedAt, err := resolveTimestamp(cmd, "recorded-at")
	if err != nil {
		return renderTimestampError(cmd.ErrOrStderr(), mode, "recorded-at", err)
	}

	c := sess.Client()
	resp, err := c.GetTask(ctx, sess.ProjectID, args[0], recordedAt)
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
		if resp.Task != nil {
			t := resp.Task
			_, _ = fmt.Fprintf(out,
				"ID:           %s\nTASK:         %s\nRECORDED AT:  %s\nDURATION:     %s\nSERVER:       %s\nAPP VERSION:  %s\n",
				t.Id, t.TaskName,
				t.RecordedAt.Format("2006-01-02 15:04:05"),
				formatDuration(t.Duration),
				pickStr(t.ServerName, "-"), pickStr(t.AppVersion, "-"),
			)
			if t.DistributedTraceId != nil {
				_, _ = fmt.Fprintf(out, "TRACE ID:     %s\n", t.DistributedTraceId.String())
			}
		}
		renderSpansTable(out, resp.Spans)
		renderLinkedErrors(out, resp.Exception, resp.Messages)
		return nil
	}
}
