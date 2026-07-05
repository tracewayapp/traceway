package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

var (
	logsSearchTypes = []string{"body", "attribute"}
	sortDirections  = []string{"asc", "desc"}
)

// severityNames maps OTel severity names to the bottom of their number range,
// so --min-severity accepts both "17" and "error".
var severityNames = map[string]uint8{
	"trace": 1,
	"debug": 5,
	"info":  9,
	"warn":  13,
	"error": 17,
	"fatal": 21,
}

// logAttrScopes are the attribute maps an --attr filter can target.
var logAttrScopes = []string{"resource", "scope", "log"}

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
		Short: "Query logs by time, service, severity, trace, or attribute",
		Long: `Query log records with any combination of filters.

--min-severity takes an OTel severity number (1-24) or a name: trace, debug,
info, warn, error, fatal.

--attr filters on the log's attribute maps with an exact key=value match.
Prefix the key with a scope to pick the map: resource:, scope:, or log:
(default log). Repeatable; multiple filters AND together.

--trace-id matches one OTel trace. --distributed-trace-id (a UUID from
"traces show" or an occurrence's distributedTraceId) matches every OTel trace
in that distributed trace across services; --exclude-trace-id drops one member
trace from that set, e.g. to hide the noisy caller and keep the callees.

A body --search over more than 24 hours requires at least one other filter
(service, severity, trace, or attribute); the server rejects wider unscoped
scans.`,
		RunE: runLogsQuery,
	}
	addTimeRangeFlags(cmd)
	addPaginationFlags(cmd)
	cmd.Flags().String("service", "", "Filter by service name")
	cmd.Flags().String("min-severity", "", "Minimum OTel severity: a number (1-24) or trace|debug|info|warn|error|fatal")
	cmd.Flags().String("trace-id", "", "Filter to a specific OpenTelemetry trace ID")
	cmd.Flags().String("distributed-trace-id", "", "Filter to every OTel trace under one distributed trace (UUID)")
	cmd.Flags().String("exclude-trace-id", "", "OTel trace ID to exclude (only with --distributed-trace-id)")
	cmd.Flags().StringArray("attr", nil, "Attribute filter as [resource:|scope:|log:]key=value (repeatable)")
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
	minSevRaw, _ := cmd.Flags().GetString("min-severity")
	minSev, err := parseSeverity(minSevRaw)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			"traceway logs query --min-severity <1-24|trace|debug|info|warn|error|fatal>")
	}
	traceID, _ := cmd.Flags().GetString("trace-id")
	distributedTraceID, _ := cmd.Flags().GetString("distributed-trace-id")
	if distributedTraceID != "" {
		if _, err := uuid.Parse(distributedTraceID); err != nil {
			return renderUsageError(cmd.ErrOrStderr(), mode,
				fmt.Sprintf("invalid --distributed-trace-id %q: must be a UUID", distributedTraceID), "")
		}
	}
	excludeTraceID, _ := cmd.Flags().GetString("exclude-trace-id")
	if excludeTraceID != "" && distributedTraceID == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"--exclude-trace-id only works together with --distributed-trace-id",
			"traceway logs query --distributed-trace-id <uuid> --exclude-trace-id <otel-trace-id>")
	}
	attrRaw, _ := cmd.Flags().GetStringArray("attr")
	attrFilters, err := parseLogAttrFilters(attrRaw)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			"use --attr [resource:|scope:|log:]key=value (repeatable)")
	}
	search, _ := cmd.Flags().GetString("search")
	searchType, _ := cmd.Flags().GetString("search-type")
	if err := validateEnumFlag("--search-type", searchType, logsSearchTypes); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway logs query", "--search-type", logsSearchTypes))
	}
	orderBy, _ := cmd.Flags().GetString("order-by")
	sortDir, _ := cmd.Flags().GetString("sort-direction")
	if err := validateEnumFlag("--sort-direction", sortDir, sortDirections); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			enumFlagHint("traceway logs query", "--sort-direction", sortDirections))
	}

	c := sess.Client()
	resp, err := c.QueryLogs(ctx, sess.ProjectID, client.QueryLogsRequest{
		TimeRange:          tr,
		Pagination:         page,
		ServiceName:        service,
		MinSeverity:        minSev,
		TraceId:            traceID,
		DistributedTraceId: distributedTraceID,
		ExcludeTraceId:     excludeTraceID,
		AttributeFilters:   attrFilters,
		Search:             search,
		SearchType:         searchType,
		OrderBy:            orderBy,
		SortDirection:      sortDir,
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
		_, _ = fmt.Fprintln(tw, "TIMESTAMP\tSEVERITY\tSERVICE\tBODY")
		for _, l := range resp.Data {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				l.Timestamp.Format("2006-01-02 15:04:05"),
				pickStr(l.SeverityText, "-"),
				pickStr(l.ServiceName, "-"),
				firstLine(l.Body),
			)
		}
		return tw.Flush()
	}
}

// parseSeverity resolves a --min-severity value: empty (no filter), an OTel
// severity number 1-24, or a level name from severityNames.
func parseSeverity(s string) (uint8, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}
	if n, ok := severityNames[s]; ok {
		return n, nil
	}
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil || n < 1 || n > 24 {
		return 0, fmt.Errorf("invalid --min-severity %q: use a number 1-24 or one of trace, debug, info, warn, error, fatal", s)
	}
	return uint8(n), nil
}

// parseLogAttrFilters parses --attr values of the form [scope:]key=value into
// LogAttributeFilters. The scope prefix picks the attribute map (resource,
// scope, or log); without one the filter targets the log's own attributes.
func parseLogAttrFilters(in []string) ([]client.LogAttributeFilter, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]client.LogAttributeFilter, 0, len(in))
	for _, item := range in {
		kv, value, ok := strings.Cut(item, "=")
		if !ok || kv == "" {
			return nil, fmt.Errorf("invalid --attr %q: expected [scope:]key=value", item)
		}
		scope := "log"
		key := kv
		if prefix, rest, hasScope := strings.Cut(kv, ":"); hasScope {
			if err := validateEnumFlag("--attr scope", prefix, logAttrScopes); err != nil {
				return nil, fmt.Errorf("invalid --attr %q: scope prefix must be one of %s", item, strings.Join(logAttrScopes, ", "))
			}
			scope, key = prefix, rest
		}
		if key == "" {
			return nil, fmt.Errorf("invalid --attr %q: key is empty", item)
		}
		out = append(out, client.LogAttributeFilter{Scope: scope, Key: key, Value: value})
	}
	return out, nil
}
