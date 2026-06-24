package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func newSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Inspect user sessions",
	}
	cmd.AddCommand(newSessionsShowCmd())
	return cmd
}

func newSessionsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <sessionId>",
		Short: "Show one session by id with the exceptions that fired during it",
		Long: `Show a single user session by its UUID, plus the exceptions/messages that fired
during it. This is the detail behind /sessions/<sessionId>.

--started-at is REQUIRED (sessions are partitioned on started_at, not
recorded_at). The timestamp bounds the lookup to a window around it so ClickHouse
prunes partitions instead of scanning all of them. The session URL carries no t=
param; source the time from the session's startedAt, the URL's from=, or the
recordedAt of a linked exception occurrence (it falls inside the window).`,
		Args: cobra.ExactArgs(1),
		RunE: runSessionsShow,
	}
	addTimestampFlag(cmd, "started-at", "Session start timestamp, RFC3339 (required; from the session startedAt or a linked occurrence's recordedAt)")
	return cmd
}

func runSessionsShow(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := validateUUIDArg(cmd, mode, args[0], "session id"); err != nil {
		return err
	}
	startedAt, err := resolveTimestamp(cmd, "started-at")
	if err != nil {
		return renderTimestampError(cmd.ErrOrStderr(), mode, "started-at", err)
	}

	c := client.New(sess.URL, client.WithJWT(sess.JWT))
	resp, err := c.GetSession(ctx, sess.ProjectID, args[0], startedAt)
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
		if resp.Session != nil {
			s := resp.Session
			_, _ = fmt.Fprintf(out,
				"ID:           %s\nSTARTED AT:   %s\nDURATION:     %s\nSERVER:       %s\nAPP VERSION:  %s\n",
				s.Id, s.StartedAt.Format("2006-01-02 15:04:05"),
				formatDuration(time.Duration(s.Duration)), pickStr(s.ServerName, "-"), pickStr(s.AppVersion, "-"),
			)
		}
		if len(resp.Exceptions) == 0 {
			_, _ = fmt.Fprintln(out, "\nEXCEPTIONS: none")
			return nil
		}
		_, _ = fmt.Fprintf(out, "\nEXCEPTIONS (%d):\n", len(resp.Exceptions))
		tw := output.NewTabWriter(out)
		_, _ = fmt.Fprintln(tw, "ID\tRECORDED AT\tHASH\tFIRST LINE")
		for _, e := range resp.Exceptions {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				e.Id, e.RecordedAt, truncateHash(e.ExceptionHash, 12), firstLine(e.StackTrace))
		}
		return tw.Flush()
	}
}
