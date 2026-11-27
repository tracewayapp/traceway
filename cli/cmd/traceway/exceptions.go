package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/v2/internal/appdir"
	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
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
			paginationHint(appdir.Binary+" exceptions list"))
	}
	page := resolvePagination(cmd)
	search, _ := cmd.Flags().GetString("search")
	searchType, _ := cmd.Flags().GetString("search-type")
	if err := validateEnumFlag("--search-type", searchType, exceptionsSearchTypes); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint(appdir.Binary+" exceptions list", "--search-type", exceptionsSearchTypes))
	}
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	orderBy, _ := cmd.Flags().GetString("order-by")
	if err := validateEnumFlag("--order-by", orderBy, exceptionsOrderBy); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint(appdir.Binary+" exceptions list", "--order-by", exceptionsOrderBy))
	}

	c := sess.Client()
	resp, err := c.ListExceptions(ctx, sess.ProjectID, client.ListExceptionsRequest{
		TimeRange:       tr,
		Pagination:      page,
		Search:          search,
		SearchType:      searchType,
		IncludeArchived: includeArchived,
		OrderBy:         orderBy,
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
		_, _ = fmt.Fprintln(tw, "HASH\tCOUNT\tLAST SEEN\tFIRST SEEN\tFIRST LINE")
		for _, e := range resp.Data {
			_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n",
				e.ExceptionHash, e.Count,
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
		Long: `Show a single exception group with its occurrences.

<hash> is the full hash from "exceptions list", or a unique prefix of it (at
least 4 hex characters) among the groups seen in the last 30 days.`,
		Args: cobra.ExactArgs(1),
		RunE: runExceptionsShow,
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
			paginationHint(appdir.Binary+" exceptions show"))
	}
	page := resolvePagination(cmd)
	page.PageSize = pickDefault(page.PageSize, 20) // detail uses 20 by default

	c := sess.Client()
	resp, err := c.GetException(ctx, sess.ProjectID, args[0], page)
	if errors.Is(err, client.ErrNotFound) && hashPrefixRegex.MatchString(args[0]) {
		hash, resolveErr := resolveExceptionHashPrefix(ctx, c, sess.ProjectID, args[0])
		if resolveErr != nil {
			return renderHashPrefixError(cmd.ErrOrStderr(), mode, args[0], err, resolveErr)
		}
		resp, err = c.GetException(ctx, sess.ProjectID, hash, page)
	}
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
				"HASH:        %s\nCOUNT:       %d\nFIRST SEEN:  %s\nLAST SEEN:   %s\n",
				resp.Group.ExceptionHash, resp.Group.Count,
				resp.Group.FirstSeen.Format("2006-01-02 15:04:05"),
				resp.Group.LastSeen.Format("2006-01-02 15:04:05"),
			)
			if related := resp.RelatedEntity; related != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "RELATED:     %s %s (%s, recorded %s)\n",
					related.TraceType, related.Name, related.Id,
					related.RecordedAt.UTC().Format(time.RFC3339Nano))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nSTACK TRACE:\n%s\n\nOCCURRENCES (%d):\n",
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

	c := sess.Client()
	resp, err := c.GetExceptionById(ctx, sess.ProjectID, args[0], recordedAt)
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
		renderTraceId(out, occ.TraceId)
		if related := resp.RelatedEntity; related != nil {
			_, _ = fmt.Fprintf(out, "RELATED:      %s %s (%s, recorded %s)\n",
				related.TraceType, related.Name, related.Id,
				related.RecordedAt.UTC().Format(time.RFC3339Nano))
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

// A full hash is 16 characters and is looked up as it is, so only a shorter one is a prefix.
var hashPrefixRegex = regexp.MustCompile(`^[0-9a-fA-F]{4,15}$`)

const (
	hashPrefixLookback = 30 * 24 * time.Hour
	hashPrefixMaxPages = 10
)

var errHashPrefixNoMatch = errors.New("no exception group matches the prefix")

type errHashPrefixAmbiguous struct{ matches []string }

func (e *errHashPrefixAmbiguous) Error() string {
	return "the prefix matches " + strings.Join(e.matches, ", ")
}

// resolveExceptionHashPrefix finds the one group of the last 30 days whose hash starts with prefix, so a hash cut
// short (an older CLI printed 12 characters) still opens its group.
func resolveExceptionHashPrefix(ctx context.Context, c *client.Client, projectID, prefix string) (string, error) {
	prefix = strings.ToLower(prefix)
	now := time.Now().UTC()
	var matches []string
	for page := 1; page <= hashPrefixMaxPages; page++ {
		resp, err := c.ListExceptions(ctx, projectID, client.ListExceptionsRequest{
			TimeRange:       client.TimeRange{From: now.Add(-hashPrefixLookback), To: now},
			Pagination:      client.PaginationParams{Page: page, PageSize: 100},
			IncludeArchived: true,
			OrderBy:         "lastSeen",
		})
		if err != nil {
			return "", err
		}
		for _, e := range resp.Data {
			if strings.HasPrefix(strings.ToLower(e.ExceptionHash), prefix) {
				matches = append(matches, e.ExceptionHash)
			}
		}
		if len(resp.Data) < 100 || int64(page) >= resp.Pagination.TotalPages {
			break
		}
	}
	switch len(matches) {
	case 0:
		return "", errHashPrefixNoMatch
	case 1:
		return matches[0], nil
	default:
		return "", &errHashPrefixAmbiguous{matches: matches}
	}
}

func renderHashPrefixError(errOut io.Writer, mode output.Mode, prefix string, notFound, resolveErr error) error {
	var ambiguous *errHashPrefixAmbiguous
	switch {
	case errors.As(resolveErr, &ambiguous):
		return renderUsageError(errOut, mode,
			fmt.Sprintf("exception hash prefix %q matches %d groups: %s", prefix, len(ambiguous.matches), strings.Join(ambiguous.matches, ", ")),
			"pass more characters of the hash, or the full hash from "+appdir.Binary+" exceptions list")
	case errors.Is(resolveErr, errHashPrefixNoMatch):
		env := classifyError(notFound, false)
		env.Hint = "no exception group of the last 30 days starts with " + prefix + "; " + appdir.Binary + " exceptions list --since 30d prints the full hashes"
		_ = output.RenderError(errOut, mode, env)
		return newCLIError(env.ExitCode, env.Code)
	default:
		return renderAPIError(errOut, mode, resolveErr, false)
	}
}
