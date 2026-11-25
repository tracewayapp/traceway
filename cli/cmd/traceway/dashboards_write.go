//go:build write

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func newDashboardsImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "import", Short: "Create dashboards from JSON and apply them to the selected project", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var data json.RawMessage
			if err := readJSONFile(cmd, &data); err != nil {
				return jsonFileUsageError(cmd, err)
			}
			mode, _ := cmd.Flags().GetString("mode")
			if err := client.ValidateDashboardImport(data, mode); err != nil {
				return jsonFileUsageError(cmd, err)
			}
			summary := "Import dashboards into the selected project's organization and apply them to this project."
			if mode == "upsert" {
				summary += " Matching names (case-insensitive) replace the entire dashboard in EVERY project using it; there is no concurrent-edit check."
			}
			return runWrite(cmd, []string{summary, string(data)}, map[string]any{"action": "import", "message": "Imported dashboards."},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return c.ImportDashboards(ctx, projectID, data, mode)
				})
		},
	}
	addJSONFileFlag(cmd)
	cmd.Flags().String("mode", "create", "create adds copies; upsert replaces dashboards matched by organization-wide name")
	return cmd
}

func newDashboardsEmbedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "embed <id>",
		Short: "Issue an expiring read-only link to a dashboard of the selected project",
		Long: "Issue an expiring read-only link to a dashboard of the selected project.\n\n" +
			"The link opens the dashboard without a Traceway account, directly or inside an\n" +
			"iframe, and shows only the signed view: the project, the time range and the tag\n" +
			"filters. It works until it expires (five minutes unless an expiry is given, 90\n" +
			"days at most) and cannot be revoked on its own; removing the dashboard from the\n" +
			"project stops every link issued for that project. The output carries the signed\n" +
			"token, so handle it like a credential.\n\n" +
			"With no range the link shows the last 42 hours. --timeframe picks another\n" +
			"relative window; with an expiry it is resolved each time the link is opened,\n" +
			"without one at issuance. --from and --to pin a fixed range instead.\n\n" +
			"A host application that embeds a dashboard for its own users calls the same\n" +
			"API from its backend for every authorized viewer: see\n" +
			"traceway://knowledge/dashboard-embedding or the traceway-write skill.",
		Example: "  traceway-write dashboards embed 42 --timeframe 7d --expires-in-days 3 --yes\n" +
			"  traceway-write dashboards embed 42 --tag account_id=acme --theme light --yes --output json",
		Args: cobra.ExactArgs(1),
		RunE: runDashboardsEmbed,
	}
	cmd.Flags().String("timeframe", "", "Relative window such as 30m, 42h or 7d, up to 731 days (default: the last 42 hours)")
	cmd.Flags().String("from", "", "Start of a fixed range, RFC 3339 (needs --to, excludes --timeframe)")
	cmd.Flags().String("to", "", "End of a fixed range, RFC 3339 (needs --from)")
	cmd.Flags().StringArray("tag", nil, "Limit every widget to a tag value, as key=value; repeat for several (at most 20)")
	cmd.Flags().Int("expires-in-days", 0, fmt.Sprintf("Lifetime in whole days, 1 to %d (default: five minutes)", client.DashboardEmbedMaxDays))
	cmd.Flags().String("expires-at", "", fmt.Sprintf("Expiry as an RFC 3339 timestamp, at most %d days ahead (excludes --expires-in-days)", client.DashboardEmbedMaxDays))
	cmd.Flags().String("theme", "", "Theme of the linked page: light or dark (default dark)")
	return cmd
}

func runDashboardsEmbed(cmd *cobra.Command, args []string) error {
	id, err := intArg(cmd, args[0], "dashboard id")
	if err != nil {
		return err
	}
	usage := func(message string) error {
		return renderUsageError(cmd.ErrOrStderr(), output.ResolveMode(flagOutput, output.StdoutIsTerminal()), message, cmd.CommandPath()+" --help")
	}
	var opts client.DashboardEmbedOptions
	opts.Timeframe, _ = cmd.Flags().GetString("timeframe")
	opts.Theme, _ = cmd.Flags().GetString("theme")
	opts.ExpiresInDays, _ = cmd.Flags().GetInt("expires-in-days")
	if cmd.Flags().Changed("expires-in-days") && opts.ExpiresInDays < 1 {
		return usage(fmt.Sprintf("--expires-in-days must be between 1 and %d", client.DashboardEmbedMaxDays))
	}
	for flag, target := range map[string]*time.Time{"from": &opts.From, "to": &opts.To, "expires-at": &opts.ExpiresAt} {
		raw, _ := cmd.Flags().GetString(flag)
		if raw == "" {
			continue
		}
		if *target, err = time.Parse(time.RFC3339, raw); err != nil {
			return usage(fmt.Sprintf("--%s must be an RFC 3339 timestamp such as 2026-06-23T14:30:00Z", flag))
		}
	}
	tags, _ := cmd.Flags().GetStringArray("tag")
	for _, tag := range tags {
		key, value, ok := strings.Cut(tag, "=")
		if !ok {
			return usage(fmt.Sprintf("--tag %q must be key=value", tag))
		}
		if opts.TagFilters == nil {
			opts.TagFilters = map[string]string{}
		}
		opts.TagFilters[key] = value
	}
	if err := opts.Validate(); err != nil {
		return usage(err.Error())
	}
	validity := "for five minutes"
	switch {
	case opts.ExpiresInDays == 1:
		validity = "for 1 day"
	case opts.ExpiresInDays > 1:
		validity = fmt.Sprintf("for %d days", opts.ExpiresInDays)
	case !opts.ExpiresAt.IsZero():
		validity = "until " + opts.ExpiresAt.UTC().Format(time.RFC3339)
	}
	summary := fmt.Sprintf("About to issue a read-only link to dashboard %d of the selected project, valid %s. Anyone holding it can view the dashboard without a Traceway account, and it cannot be revoked on its own.", id, validity)

	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := confirmMutation(cmd, []string{summary}); err != nil {
		return err
	}
	raw, err := sess.Client().CreateDashboardEmbed(cmd.Context(), sess.ProjectID, id, opts, sess.URL)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}
	return renderRaw(cmd, mode, raw, func(w io.Writer, v any) error {
		link, _ := v.(map[string]any)
		_, err := fmt.Fprintf(w, "%v\nExpires at %v\n", link["url"], link["expiresAt"])
		return err
	})
}
