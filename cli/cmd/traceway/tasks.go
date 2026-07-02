package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
)

func newTasksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Inspect background task runs",
	}
	cmd.AddCommand(newTasksShowCmd())
	return cmd
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
(/tasks/<name>/<id>?t=...) or a distributed-trace node's recordedAt.`,
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
