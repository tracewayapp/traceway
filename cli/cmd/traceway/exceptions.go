package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

var (
	exceptionsOrderBy     = client.ExceptionsOrderByValues
	exceptionsSearchTypes = client.ExceptionsSearchTypes
)

func newExceptionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exceptions",
		Short: "Query exception groups and occurrences",
	}
	cmd.AddCommand(newExceptionsListCmd())
	cmd.AddCommand(newExceptionsShowCmd())
	cmd.AddCommand(newExceptionsOccurrenceCmd())
	cmd.AddCommand(newExceptionsArchiveCmd())
	cmd.AddCommand(newExceptionsUnarchiveCmd())
	return cmd
}

func newExceptionsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent exception groups",
		RunE:  runExceptionsList,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("search", "", "Free-text search filter")
	cmd.Flags().String("search-type", "text", "Search type: text or regex")
	cmd.Flags().Bool("include-archived", false, "Include archived exceptions")
	cmd.Flags().String("order-by", "lastSeen", "Sort field (lastSeen, firstSeen, count)")
	return cmd
}

func runExceptionsList(cmd *cobra.Command, _ []string) error {
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
			paginationHint("traceway exceptions list"))
	}
	page := resolvePagination(cmd)
	search, _ := cmd.Flags().GetString("search")
	searchType, _ := cmd.Flags().GetString("search-type")
	if err := validateEnumFlag("--search-type", searchType, exceptionsSearchTypes); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway exceptions list", "--search-type", exceptionsSearchTypes))
	}
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	orderBy, _ := cmd.Flags().GetString("order-by")
	if err := validateEnumFlag("--order-by", orderBy, exceptionsOrderBy); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway exceptions list", "--order-by", exceptionsOrderBy))
	}

	sources, err := exceptionSources(sess)
	if err != nil {
		return renderSourceError(cmd.ErrOrStderr(), mode, err)
	}
	resp, failed, err := access.ListExceptions(ctx, sources, access.ExceptionQuery{
		ProjectID:       sess.ProjectID,
		Window:          tr,
		Page:            page,
		Search:          search,
		SearchType:      searchType,
		IncludeArchived: includeArchived,
		OrderBy:         orderBy,
	})
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}
	reportSourceFailures(cmd.ErrOrStderr(), failed)

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		col := sourceColumnFor(sources)
		tw := output.NewTabWriter(cmd.OutOrStdout())
		_, _ = fmt.Fprintln(tw, col.header("HASH\tCOUNT\tLAST SEEN\tFIRST SEEN\tFIRST LINE"))
		for _, e := range resp.Data {
			hash := e.ExceptionHash
			if len(hash) > 12 {
				hash = hash[:12]
			}
			_, _ = fmt.Fprintf(tw, col.cell(e.Source)+"%s\t%d\t%s\t%s\t%s\n",
				hash, e.Count,
				e.LastSeen.Format("2006-01-02 15:04:05"),
				e.FirstSeen.Format("2006-01-02 15:04:05"),
				firstLine(e.StackTrace),
			)
		}
		return tw.Flush()
	}
}

func newExceptionsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <hash>",
		Short: "Show a single exception group with its occurrences",
		Args:  cobra.ExactArgs(1),
		RunE:  runExceptionsShow,
	}
	addPaginationFlags(cmd)
	return cmd
}

func runExceptionsShow(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := validatePaginationFlags(cmd); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			paginationHint("traceway exceptions show"))
	}
	page := resolvePagination(cmd)
	page.PageSize = pickDefault(page.PageSize, 20) // detail uses 20 by default

	sources, err := exceptionSources(sess)
	if err != nil {
		return renderSourceError(cmd.ErrOrStderr(), mode, err)
	}
	resp, err := access.GetException(ctx, sources, access.ExceptionLookup{ProjectID: sess.ProjectID, Hash: args[0], Page: page})
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		// Group header, then occurrences table.
		if resp.Group != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"HASH:        %s\nCOUNT:       %d\nFIRST SEEN:  %s\nLAST SEEN:   %s\n\nSTACK TRACE:\n%s\n\nOCCURRENCES (%d):\n",
				resp.Group.ExceptionHash, resp.Group.Count,
				resp.Group.FirstSeen.Format("2006-01-02 15:04:05"),
				resp.Group.LastSeen.Format("2006-01-02 15:04:05"),
				resp.Group.StackTrace,
				len(resp.Occurrences),
			)
		}
		tw := output.NewTabWriter(cmd.OutOrStdout())
		_, _ = fmt.Fprintln(tw, "ID\tRECORDED AT\tSERVER\tTRACE TYPE")
		for _, occ := range resp.Occurrences {
			traceType := occ.TraceType
			if traceType == "" {
				traceType = "-"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				occ.Id.String(),
				occ.RecordedAt.Format("2006-01-02 15:04:05"),
				pickStr(occ.ServerName, "-"),
				traceType,
			)
		}
		return tw.Flush()
	}
}

func newExceptionsOccurrenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "occurrence <exceptionId>",
		Short: "Show one exception occurrence by id (fast, time-bounded)",
		Long: `Show a single exception occurrence by its UUID.

Unlike "exceptions show <hash>" (which groups by stack-trace hash and paginates
occurrences), this resolves one occurrence directly and additionally returns its
linked sessionId and an inline session recording.

--recorded-at is REQUIRED. Occurrences live in a daily-partitioned table; the
timestamp bounds the lookup to a window around it so ClickHouse prunes
partitions instead of scanning all of them. Source it from the dashboard URL's
t= param (/issues/<hash>/<id>?t=...), an "exceptions show" occurrence's
recordedAt, or a notification's "Occurred at" (convert "2006-01-02 15:04:05 UTC"
to RFC3339, i.e. "2006-01-02T15:04:05Z").`,
		Args: cobra.ExactArgs(1),
		RunE: runExceptionsOccurrence,
	}
	addTimestampFlag(cmd, "recorded-at", "Occurrence timestamp, RFC3339 (required; from the URL t= param or a notification's Occurred at)")
	return cmd
}

func runExceptionsOccurrence(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := validateUUIDArg(cmd, mode, args[0], "exception id"); err != nil {
		return err
	}
	recordedAt, err := resolveTimestamp(cmd, "recorded-at")
	if err != nil {
		return renderTimestampError(cmd.ErrOrStderr(), mode, "recorded-at", err)
	}

	sources, err := exceptionSources(sess)
	if err != nil {
		return renderSourceError(cmd.ErrOrStderr(), mode, err)
	}
	resp, err := access.GetOccurrence(ctx, sources, access.Lookup{ProjectID: sess.ProjectID, ID: args[0], At: recordedAt})
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		occ := resp.Exception
		if occ == nil {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "No occurrence returned.")
			return err
		}
		out := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(out,
			"ID:           %s\nHASH:         %s\nRECORDED AT:  %s\nSERVER:       %s\nAPP VERSION:  %s\nTRACE TYPE:   %s\n",
			occ.Id, occ.ExceptionHash,
			occ.RecordedAt.Format("2006-01-02 15:04:05"),
			pickStr(occ.ServerName, "-"),
			pickStr(occ.AppVersion, "-"),
			pickStr(occ.TraceType, "-"),
		)
		if occ.DistributedTraceId != nil {
			_, _ = fmt.Fprintf(out, "TRACE ID:     %s\n", occ.DistributedTraceId.String())
		}
		sid := resp.SessionId
		if sid == nil {
			sid = occ.SessionId
		}
		if sid != nil {
			_, _ = fmt.Fprintf(out, "SESSION ID:   %s\n", sid.String())
		}
		_, _ = fmt.Fprintf(out, "\nSTACK TRACE:\n%s\n", occ.StackTrace)
		return nil
	}
}

func newExceptionsArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <hash> [<hash>...]",
		Short: "Archive one or more exception groups",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExceptionsMutation(cmd, args, "archive", access.ExceptionAccess.ArchiveExceptions)
		},
	}
}

func newExceptionsUnarchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unarchive <hash> [<hash>...]",
		Short: "Unarchive one or more exception groups",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExceptionsMutation(cmd, args, "unarchive", access.ExceptionAccess.UnarchiveExceptions)
		},
	}
}

// runExceptionsMutation is the shared body for archive and unarchive. The
// 'verb' parameter controls the prompt wording and the rendered action label;
// 'apply' is the access method to call after confirmation passes. An archive
// changes state at one source, so with several bound the caller names it.
func runExceptionsMutation(
	cmd *cobra.Command,
	hashes []string,
	verb string,
	apply func(source access.ExceptionAccess, ctx context.Context, projectID string, hashes []string) error,
) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	sources, err := exceptionSources(sess)
	if err != nil {
		return renderSourceError(cmd.ErrOrStderr(), mode, err)
	}
	if len(sources) > 1 {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"several sources answer exceptions; say which one holds these hashes",
			"traceway exceptions "+verb+" --source <name> <hash>")
	}

	summary := []string{
		fmt.Sprintf("About to %s %d exception group(s):", verb, len(hashes)),
	}
	for _, h := range hashes {
		summary = append(summary, "  - "+truncateHash(h, 12))
	}
	if err := confirmMutation(cmd, summary); err != nil {
		return err
	}

	if err := apply(sources[0], ctx, sess.ProjectID, hashes); err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}

	result := map[string]any{
		"action": verb,
		"count":  len(hashes),
		"hashes": hashes,
	}
	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), result, nil)
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), result, nil)
	default:
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%sd %d exception group(s).\n", verb, len(hashes))
		return err
	}
}

// truncateHash returns hash, or its first n chars if longer. Used for
// human-readable summaries; the full hash always goes to the API.
func truncateHash(hash string, n int) string {
	if len(hash) <= n {
		return hash
	}
	return hash[:n]
}

func exceptionSources(sess *session) ([]access.ExceptionAccess, error) {
	resolver, err := sess.Resolver()
	if err != nil {
		return nil, err
	}
	return access.Exceptions(resolver, flagSource)
}
