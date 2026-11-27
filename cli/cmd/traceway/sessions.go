package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/v2/internal/appdir"
	"github.com/tracewayapp/traceway/cli/v2/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

var sessionsOrderBy = client.SessionsOrderByValues

func newSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Inspect user sessions",
	}
	cmd.AddCommand(newSessionsListCmd())
	cmd.AddCommand(newSessionsShowCmd())
	return cmd
}

func newSessionsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the sessions that started in a time window, newest first",
		Long: `List the project's user sessions (browser, Flutter, Android, iOS) that started
in the time window, newest first. Each row has the session id and startedAt that
"sessions show" takes, its duration, the app version and whether a replay was
recorded. --search matches the session id, the client IP and attribute values
such as a user id or email.`,
		Args: cobra.NoArgs,
		RunE: runSessionsList,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("search", "", "Match the session id, client IP or an attribute value (user id, email, ...)")
	cmd.Flags().String("order-by", "startedAt", "Sort field: startedAt or duration")
	cmd.Flags().String("sort-direction", "desc", "Sort direction: asc or desc")
	return cmd
}

func runSessionsList(cmd *cobra.Command, _ []string) error {
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
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), paginationHint(appdir.Binary+" sessions list"))
	}
	search, _ := cmd.Flags().GetString("search")
	orderBy, _ := cmd.Flags().GetString("order-by")
	if err := validateEnumFlag("--order-by", orderBy, sessionsOrderBy); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint(appdir.Binary+" sessions list", "--order-by", sessionsOrderBy))
	}
	sortDir, _ := cmd.Flags().GetString("sort-direction")
	if err := validateEnumFlag("--sort-direction", sortDir, sortDirections); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint(appdir.Binary+" sessions list", "--sort-direction", sortDirections))
	}

	resp, err := sess.Client().ListSessions(ctx, sess.ProjectID, client.ListSessionsRequest{
		TimeRange:     tr,
		Pagination:    resolvePagination(cmd),
		OrderBy:       orderBy,
		SortDirection: sortDir,
		Search:        strings.TrimSpace(search),
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
		_, _ = fmt.Fprintln(tw, "ID\tSTARTED AT\tDURATION\tAPP VERSION\tREPLAY\tUSER")
		for _, s := range resp.Data {
			replay := "-"
			if s.HasRecording {
				replay = "yes"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				s.Id, s.StartedAt.UTC().Format(time.RFC3339),
				formatDuration(time.Duration(s.Duration)), pickStr(s.AppVersion, "-"), replay, sessionUser(s.Attributes))
		}
		return tw.Flush()
	}
}

// sessionUser is the user the SDK attached to the session, if any.
func sessionUser(attrs map[string]string) string {
	for _, key := range []string{"userId", "user.id", "email", "user.email"} {
		if v := attrs[key]; v != "" {
			return v
		}
	}
	return "-"
}

func newSessionsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <sessionId>",
		Short: "Show one session by id with the exceptions that fired during it",
		Long: `Show a single user session by its UUID, plus the exceptions/messages that fired
during it. This is the detail behind /sessions/<sessionId>.

--started-at bounds the lookup to a window around the session's start (sessions
are partitioned on started_at, not recorded_at), so ClickHouse prunes partitions
instead of scanning all of them. Pass it when you have it: the startedAt of
"sessions list", the URL's from=, or the recordedAt of a linked exception
occurrence (it falls inside the window). Without it the start is looked up
first among the sessions of the last 30 days.`,
		Args: cobra.ExactArgs(1),
		RunE: runSessionsShow,
	}
	addTimestampFlag(cmd, "started-at", "Session start timestamp, RFC3339 (from sessions list, the session's startedAt or a linked occurrence's recordedAt; looked up when omitted)")
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
	c := sess.Client()
	startedAt, err := resolveTimestamp(cmd, "started-at")
	if errors.Is(err, errTimestampMissing) {
		startedAt, err = c.FindSessionStart(ctx, sess.ProjectID, args[0])
		if errors.Is(err, client.ErrSessionNotFound) {
			return renderSessionStartNotFound(cmd.ErrOrStderr(), mode)
		}
		if err != nil {
			return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
		}
	} else if err != nil {
		return renderTimestampError(cmd.ErrOrStderr(), mode, "started-at", err)
	}

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
				e.Id, e.RecordedAt, e.ExceptionHash, firstLine(e.StackTrace))
		}
		return tw.Flush()
	}
}

func renderSessionStartNotFound(errOut io.Writer, mode output.Mode) error {
	env := output.ErrorEnvelope{
		Code:     "not_found",
		Message:  "no session with this id started in the last 30 days",
		Hint:     "pass --started-at for an older session, or check the project (" + appdir.Binary + " sessions list --since 7d)",
		ExitCode: exitcode.NotFound,
	}
	_ = output.RenderError(errOut, mode, env)
	return newCLIError(env.ExitCode, env.Code)
}
