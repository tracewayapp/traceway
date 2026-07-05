package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// sessionsOrderBy maps the user-facing --order-by values to the server's
// snake_case field names for POST /api/sessions.
var sessionsOrderBy = map[string]string{
	"startedAt": "started_at",
	"duration":  "duration",
}

var sessionsOrderByValues = []string{"startedAt", "duration"}

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
		Short: "List user sessions",
		Long: `List user sessions in the window, newest first — the list behind /sessions.

Each row's id and startedAt feed the by-id detail lookup:
"sessions show <id> --started-at <startedAt>".

--attr filters on session attributes with an exact key=value match
(repeatable; filters AND together). Session replay stays dashboard-only.`,
		RunE: runSessionsList,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("search", "", "Free-text search filter")
	cmd.Flags().StringArray("attr", nil, "Attribute filter as key=value (repeatable)")
	cmd.Flags().String("order-by", "startedAt", "Sort field (startedAt, duration)")
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
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			paginationHint("traceway sessions list"))
	}
	page := resolvePagination(cmd)
	search, _ := cmd.Flags().GetString("search")
	orderBy, _ := cmd.Flags().GetString("order-by")
	if err := validateEnumFlag("--order-by", orderBy, sessionsOrderByValues); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway sessions list", "--order-by", sessionsOrderByValues))
	}
	sortDir, _ := cmd.Flags().GetString("sort-direction")
	if err := validateEnumFlag("--sort-direction", sortDir, sortDirections); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway sessions list", "--sort-direction", sortDirections))
	}
	attrRaw, _ := cmd.Flags().GetStringArray("attr")
	attrFilters, err := parseSessionAttrFilters(attrRaw)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			"use --attr key=value (repeatable)")
	}

	c := sess.Client()
	resp, err := c.ListSessions(ctx, sess.ProjectID, client.ListSessionsRequest{
		TimeRange:        tr,
		Pagination:       page,
		Search:           search,
		OrderBy:          sessionsOrderBy[orderBy],
		SortDirection:    sortDir,
		AttributeFilters: attrFilters,
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
		_, _ = fmt.Fprintln(tw, "ID\tSTARTED AT\tDURATION\tAPP VERSION\tSERVER")
		for _, s := range resp.Data {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				s.Id,
				s.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
				formatDuration(time.Duration(s.Duration)),
				pickStr(s.AppVersion, "-"), pickStr(s.ServerName, "-"),
			)
		}
		return tw.Flush()
	}
}

// parseSessionAttrFilters parses --attr values of the form key=value.
// Sessions have a single attribute map, so no scope prefix.
func parseSessionAttrFilters(in []string) ([]client.SessionAttributeFilter, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]client.SessionAttributeFilter, 0, len(in))
	for _, item := range in {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid --attr %q: expected key=value", item)
		}
		out = append(out, client.SessionAttributeFilter{Key: key, Value: value})
	}
	return out, nil
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

	c := sess.Client()
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
