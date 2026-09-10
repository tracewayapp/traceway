package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

var (
	logsSearchTypes = client.LogsSearchTypes
	sortDirections  = client.SortDirections
)

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Query log records",
	}
	cmd.AddCommand(newLogsQueryCmd())
	return cmd
}

func newLogsQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query logs by time, service, severity, or trace",
		RunE:  runLogsQuery,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("service", "", "Filter by service name")
	cmd.Flags().Uint8("min-severity", 0, "Minimum OTel severity number (1=TRACE, 5=DEBUG, 9=INFO, 13=WARN, 17=ERROR, 21=FATAL)")
	cmd.Flags().String("trace-id", "", "Filter to a specific OpenTelemetry trace ID")
	cmd.Flags().String("search", "", "Free-text search in body")
	cmd.Flags().String("search-type", "body", "Search type: body or attribute")
	cmd.Flags().String("order-by", "timestamp", "Sort field")
	cmd.Flags().String("sort-direction", "desc", "Sort direction: asc or desc")
	return cmd
}

func runLogsQuery(cmd *cobra.Command, _ []string) error {
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
			paginationHint("traceway logs query"))
	}
	page := resolvePagination(cmd)
	service, _ := cmd.Flags().GetString("service")
	minSev, _ := cmd.Flags().GetUint8("min-severity")
	traceID, _ := cmd.Flags().GetString("trace-id")
	search, _ := cmd.Flags().GetString("search")
	searchType, _ := cmd.Flags().GetString("search-type")
	if err := validateEnumFlag("--search-type", searchType, logsSearchTypes); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway logs query", "--search-type", logsSearchTypes))
	}
	if orderBy, _ := cmd.Flags().GetString("order-by"); orderBy != "timestamp" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "--order-by must be timestamp",
			"traceway logs query --order-by timestamp")
	}
	sortDir, _ := cmd.Flags().GetString("sort-direction")
	if err := validateEnumFlag("--sort-direction", sortDir, sortDirections); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway logs query", "--sort-direction", sortDirections))
	}

	sources, err := logsSources(sess)
	if err != nil {
		return renderSourceError(cmd.ErrOrStderr(), mode, err)
	}
	resp, failed, err := access.QueryLogs(ctx, sources, access.LogQuery{
		ProjectID:     sess.ProjectID,
		Window:        tr,
		Page:          page,
		ServiceName:   service,
		MinSeverity:   minSev,
		TraceID:       traceID,
		Search:        search,
		SearchType:    searchType,
		SortDirection: sortDir,
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
		_, _ = fmt.Fprintln(tw, col.header("TIMESTAMP\tSEVERITY\tSERVICE\tBODY"))
		for _, l := range resp.Data {
			_, _ = fmt.Fprintf(tw, col.cell(l.Source)+"%s\t%s\t%s\t%s\n",
				l.Timestamp.Format("2006-01-02 15:04:05"),
				pickStr(l.SeverityText, "-"),
				pickStr(l.ServiceName, "-"),
				firstLine(l.Body),
			)
		}
		return tw.Flush()
	}
}

func logsSources(sess *session) ([]access.LogsAccess, error) {
	resolver, err := sess.Resolver()
	if err != nil {
		return nil, err
	}
	return access.Logs(resolver, flagSource)
}
