package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
)

func newAiTracesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai-traces",
		Short: "Inspect AI / LLM traces",
	}
	cmd.AddCommand(newAiTracesShowCmd())
	return cmd
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
