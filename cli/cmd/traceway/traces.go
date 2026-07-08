package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
)

func newTracesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traces",
		Short: "Inspect distributed traces across services",
	}
	cmd.AddCommand(newTracesShowCmd())
	return cmd
}

func newTracesShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <distributedTraceId>",
		Short: "Show a distributed trace: every service node that shares the trace id",
		Long: `Show the full cross-service timeline for a distributed trace id — every
endpoint, task, ai-trace, and exception node that shares it, across all projects
you can see. This is the highest-value root-cause view: it stitches one logical
request together end to end.

The distributed trace id comes from an occurrence's distributedTraceId (in
"exceptions show" / "exceptions occurrence" output) or an endpoint/task's
distributedTraceId.

--recorded-at is REQUIRED. The lookup is bounded to a window around it so
ClickHouse prunes partitions instead of scanning all of them. Use the recordedAt
of the occurrence/node you got the trace id from.`,
		Args: cobra.ExactArgs(1),
		RunE: runTracesShow,
	}
	addTimestampFlag(cmd, "recorded-at", "A node's timestamp, RFC3339 (required; the recordedAt of the occurrence/node you got the trace id from)")
	return cmd
}

func runTracesShow(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := validateUUIDArg(cmd, mode, args[0], "distributed trace id"); err != nil {
		return err
	}
	recordedAt, err := resolveTimestamp(cmd, "recorded-at")
	if err != nil {
		return renderTimestampError(cmd.ErrOrStderr(), mode, "recorded-at", err)
	}

	c := sess.Client()
	resp, err := c.GetDistributedTrace(ctx, args[0], recordedAt)
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
		_, _ = fmt.Fprintf(out, "DISTRIBUTED TRACE: %s\nNODES (%d):\n", resp.DistributedTraceId, len(resp.Nodes))
		tw := output.NewTabWriter(out)
		_, _ = fmt.Fprintln(tw, "PROJECT\tTYPE\tNAME\tERROR")
		for _, n := range resp.Nodes {
			name := "-"
			switch {
			case n.Endpoint != nil:
				name = n.Endpoint.Endpoint
			case n.Task != nil:
				name = n.Task.TaskName
			case n.AiTrace != nil:
				name = n.AiTrace.TraceName
			}
			errCol := "-"
			if n.Exception != nil {
				errCol = truncateHash(n.Exception.ExceptionHash, 12)
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", pickStr(n.ProjectName, "-"), n.TraceType, name, errCol)
		}
		return tw.Flush()
	}
}
