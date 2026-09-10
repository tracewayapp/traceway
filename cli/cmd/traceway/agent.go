package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Fix-agent attempts: list them, inspect one, report a CI run",
		Long: `Attempts are the fix agent's runs on issues. "agent attempts" lists the
project's attempts, "agent show" prints one with its links and events, and
"agent report" records a run that happened outside Traceway (the auto-fix
GitHub Actions contract) as an attempt on the issue page.`,
	}
	cmd.AddCommand(newAgentAttemptsCmd())
	cmd.AddCommand(newAgentShowCmd())
	cmd.AddCommand(newAgentReportCmd())
	return cmd
}

func newAgentAttemptsCmd() *cobra.Command {
	var status string
	var page, pageSize int
	cmd := &cobra.Command{
		Use:   "attempts",
		Short: "List the project's attempts, newest first",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
			sess, err := loadSession()
			if err != nil {
				return renderSessionError(cmd.ErrOrStderr(), mode, err)
			}
			result, err := sess.Client().ListAttempts(cmd.Context(), sess.ProjectID, status, client.PaginationParams{Page: page, PageSize: pageSize})
			if err != nil {
				return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
			}
			switch mode {
			case output.ModeJSON:
				return output.RenderJSON(cmd.OutOrStdout(), result, output.ParseFieldsFlag(flagFields))
			case output.ModeYAML:
				return output.RenderYAML(cmd.OutOrStdout(), result, output.ParseFieldsFlag(flagFields))
			}
			tw := output.NewTabWriter(cmd.OutOrStdout())
			_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tSUBJECT\tSTATUS\tEXECUTOR\tCOST\tCREATED")
			for _, attempt := range result.Data {
				_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t$%.2f\t%s\n", attempt.ID, attempt.Number, attempt.SubjectRef, attempt.Status, attempt.Executor, attempt.CostUSD, attempt.CreatedAt.Format("2006-01-02 15:04"))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "only attempts in this status (queued, running, needs_input, awaiting_review, merged, analyzed, failed, ...)")
	cmd.Flags().IntVar(&page, "page", 1, "page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 25, "attempts per page (max 100)")
	return cmd
}

type attemptShowResponse struct {
	Attempt client.Attempt        `json:"attempt"`
	Links   []client.AttemptLink  `json:"links"`
	Events  []client.AttemptEvent `json:"events"`
}

func newAgentShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <attempt-id>",
		Short: "Print one attempt with its links and event stream",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
			sess, err := loadSession()
			if err != nil {
				return renderSessionError(cmd.ErrOrStderr(), mode, err)
			}
			api := sess.Client()
			detail, err := api.GetAttempt(cmd.Context(), sess.ProjectID, args[0])
			if err != nil {
				return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
			}
			events, _, err := api.AttemptEvents(cmd.Context(), sess.ProjectID, args[0], 0)
			if err != nil {
				return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
			}
			resp := attemptShowResponse{Attempt: detail.Attempt, Links: detail.Links, Events: events}
			switch mode {
			case output.ModeJSON:
				return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
			case output.ModeYAML:
				return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
			}
			out := cmd.OutOrStdout()
			a := resp.Attempt
			_, _ = fmt.Fprintf(out, "Attempt %d on %s (%s)\nStatus: %s   Executor: %s   Agent: %s %s   Cost: $%.2f   Turns: %d\n", a.Number, a.SubjectRef, a.ID, a.Status, a.Executor, a.Agent, a.Model, a.CostUSD, a.Turns)
			if a.Error != "" {
				_, _ = fmt.Fprintf(out, "Error: %s\n", a.Error)
			}
			for _, link := range resp.Links {
				_, _ = fmt.Fprintf(out, "Link: %s %s %s %s\n", link.Provider, link.Kind, link.ExternalRef, link.URL)
			}
			for _, event := range resp.Events {
				text, _ := event.Payload["text"].(string)
				if status, ok := event.Payload["status"].(string); ok && text == "" {
					text = status
				}
				_, _ = fmt.Fprintf(out, "%4d  %-16s %s\n", event.Seq, event.Kind, strings.SplitN(text, "\n", 2)[0])
			}
			return nil
		},
	}
}

func newAgentReportCmd() *cobra.Command {
	var report client.AttemptReport
	var reportFile string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Record a run that happened outside Traceway as an attempt on the issue",
		Long: `The auto-fix GitHub Actions contract runs the agent in CI. "agent report"
gives that run an attempt on the issue page: --status fixed with the pull
request it opened, or --status analysis with the report alone. The report
text comes from --report-file (the agent's report file; its STATUS and HASH
lines are dropped) or --report.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
			if reportFile != "" {
				raw, err := os.ReadFile(reportFile)
				if err != nil {
					return renderUsageError(cmd.ErrOrStderr(), mode, "cannot read the report file: "+err.Error(), "traceway agent report --report-file <path>")
				}
				report.Report = stripReportHeader(string(raw))
			}
			if strings.TrimSpace(report.Report) == "" {
				return renderUsageError(cmd.ErrOrStderr(), mode, "a report is required", "traceway agent report --hash <hash> --status fixed|analysis --report-file <path>")
			}
			if report.Status != "fixed" && report.Status != "analysis" {
				return renderUsageError(cmd.ErrOrStderr(), mode, "--status must be fixed or analysis", "traceway agent report --status fixed --pr <url>")
			}
			sess, err := loadSession()
			if err != nil {
				return renderSessionError(cmd.ErrOrStderr(), mode, err)
			}
			attempt, err := sess.Client().ReportAttempt(cmd.Context(), sess.ProjectID, report)
			if err != nil {
				return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
			}
			switch mode {
			case output.ModeJSON:
				return output.RenderJSON(cmd.OutOrStdout(), map[string]any{"attempt": attempt}, output.ParseFieldsFlag(flagFields))
			case output.ModeYAML:
				return output.RenderYAML(cmd.OutOrStdout(), map[string]any{"attempt": attempt}, output.ParseFieldsFlag(flagFields))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Recorded attempt %d on %s (%s): %s\n", attempt.Number, attempt.SubjectRef, attempt.ID, attempt.Status)
			return nil
		},
	}
	cmd.Flags().StringVar(&report.Hash, "hash", "", "the exception hash the run was about (required)")
	cmd.Flags().StringVar(&report.Status, "status", "", "fixed or analysis (required)")
	cmd.Flags().StringVar(&report.Branch, "branch", "", "the branch the fix was pushed to")
	cmd.Flags().StringVar(&report.PullRequestURL, "pr", "", "the pull request URL (required for fixed)")
	cmd.Flags().StringVar(&reportFile, "report-file", "", "file holding the agent's report")
	cmd.Flags().StringVar(&report.Report, "report", "", "the report text, when not reading a file")
	_ = cmd.MarkFlagRequired("hash")
	_ = cmd.MarkFlagRequired("status")
	return cmd
}

// stripReportHeader drops the STATUS and HASH lines the auto-fix contract
// puts at the top of a report file, leaving the markdown body.
func stripReportHeader(raw string) string {
	lines := strings.Split(raw, "\n")
	for len(lines) > 0 && (strings.HasPrefix(lines[0], "STATUS:") || strings.HasPrefix(lines[0], "HASH:") || strings.HasPrefix(lines[0], "SUBJECT:")) {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
