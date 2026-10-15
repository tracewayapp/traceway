package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func runRaw(cmd *cobra.Command, call func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error), table func(w io.Writer, v any) error) error {
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	raw, err := call(cmd.Context(), sess.Client(), sess.ProjectID)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}
	return renderRaw(cmd, mode, raw, table)
}

func renderRaw(cmd *cobra.Command, mode output.Mode, raw json.RawMessage, table func(w io.Writer, v any) error) error {
	var v any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
	}
	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), v, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), v, output.ParseFieldsFlag(flagFields))
	}
	if table != nil {
		return table(cmd.OutOrStdout(), v)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return err
}

func intArg(cmd *cobra.Command, value, label string) (int, error) {
	id, err := strconv.Atoi(value)
	if err != nil || id < 1 {
		mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
		return 0, renderUsageError(cmd.ErrOrStderr(), mode, fmt.Sprintf("invalid %s %q: must be a positive integer", label, value), "")
	}
	return id, nil
}

func rows(v any, keys ...string) []map[string]any {
	var list []any
	switch t := v.(type) {
	case []any:
		list = t
	case map[string]any:
		for _, k := range append(keys, "data") {
			if l, ok := t[k].([]any); ok {
				list = l
				break
			}
		}
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func cell(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return "-"
	}
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case string:
		if t == "" {
			return "-"
		}
		return t
	default:
		return fmt.Sprint(t)
	}
}

func listTable(wrapperKey string, columns ...[2]string) func(w io.Writer, v any) error {
	return func(w io.Writer, v any) error {
		tw := output.NewTabWriter(w)
		headers := make([]string, len(columns))
		for i, c := range columns {
			headers[i] = c[1]
		}
		_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
		for _, r := range rows(v, wrapperKey) {
			values := make([]string, len(columns))
			for i, c := range columns {
				values[i] = cell(r, c[0])
			}
			_, _ = fmt.Fprintln(tw, strings.Join(values, "\t"))
		}
		return tw.Flush()
	}
}

func newPagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "pages", Short: "On-call pages: alerts that paged a person"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List on-call pages (default: open and acknowledged)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			status, _ := cmd.Flags().GetString("status")
			mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
			if err := validateEnumFlag("--status", status, client.PageStatuses); err != nil {
				return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), enumFlagHint(appdir.Binary+" pages list", "--status", client.PageStatuses))
			}
			if err := validatePaginationFlags(cmd); err != nil {
				return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), paginationHint(appdir.Binary+" pages list"))
			}
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.ListPages(ctx, projectID, client.ListPagesRequest{Status: status, Pagination: resolvePagination(cmd)})
			}, listTable("data", [2]string{"id", "ID"}, [2]string{"status", "STATUS"}, [2]string{"urgency", "URGENCY"}, [2]string{"subject", "SUBJECT"}, [2]string{"eventCount", "EVENTS"}, [2]string{"createdAt", "OPENED"}))
		},
	}
	list.Flags().String("status", "active", "active, open, acknowledged, or resolved")
	addPaginationFlags(list)
	show := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one page and its delivery log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "page id")
			if err != nil {
				return err
			}
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.GetPage(ctx, projectID, id)
			}, nil)
		},
	}
	cmd.AddCommand(list, show)
	return cmd
}

func newAlertsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "alerts", Short: "Alert rules that notify channels and page on-call"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the project's alert rules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.ListNotificationRules(ctx, projectID)
			}, listTable("rules", [2]string{"id", "ID"}, [2]string{"name", "NAME"}, [2]string{"ruleType", "TYPE"}, [2]string{"enabled", "ENABLED"}, [2]string{"snoozedUntil", "SNOOZED UNTIL"}))
		},
	})
	return cmd
}

func newMonitorsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "monitors", Short: "Uptime monitors (synthetic checks)"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the project's monitors and their current status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.ListChecks(ctx, projectID)
			}, listTable("checks", [2]string{"id", "ID"}, [2]string{"name", "NAME"}, [2]string{"checkType", "TYPE"}, [2]string{"currentStatus", "STATUS"}, [2]string{"intervalSeconds", "INTERVAL (S)"}, [2]string{"enabled", "ENABLED"}))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show one monitor with its recent incidents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "monitor id")
			if err != nil {
				return err
			}
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.GetCheck(ctx, projectID, id)
			}, nil)
		},
	})
	return cmd
}

func newPostMortemsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "post-mortems", Short: "Incident post-mortem documents"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List post-mortems, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			search, _ := cmd.Flags().GetString("search")
			tags, _ := cmd.Flags().GetStringSlice("tag")
			if err := validatePaginationFlags(cmd); err != nil {
				mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
				return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), paginationHint(appdir.Binary+" post-mortems list"))
			}
			page := resolvePagination(cmd)
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.ListPostMortems(ctx, projectID, client.ListPostMortemsRequest{Search: search, Tags: tags, Page: page.Page, PageSize: page.PageSize})
			}, listTable("data", [2]string{"id", "ID"}, [2]string{"title", "TITLE"}, [2]string{"tags", "TAGS"}, [2]string{"incidentId", "INCIDENT"}, [2]string{"updatedAt", "UPDATED"}))
		},
	}
	list.Flags().String("search", "", "Match title, content and tags")
	list.Flags().StringSlice("tag", nil, "Only post-mortems with this tag (repeatable, ANDed)")
	addPaginationFlags(list)
	show := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one post-mortem with its markdown content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "post-mortem id")
			if err != nil {
				return err
			}
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.GetPostMortem(ctx, projectID, id)
			}, nil)
		},
	}
	cmd.AddCommand(list, show)
	return cmd
}
