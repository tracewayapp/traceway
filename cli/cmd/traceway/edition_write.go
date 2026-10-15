//go:build write

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

const (
	editionWrite   = true
	editionShort   = "CLI for the Traceway observability platform, with write access"
	deviceClientID = "traceway-cli-write"
	loginScope     = client.ScopeWrite
)

func init() {
	appdir.Name = "traceway-write"
	appdir.Binary = "traceway-write"
}

func addWriteCommands(root *cobra.Command) {
	for _, sub := range root.Commands() {
		switch sub.Name() {
		case "exceptions":
			sub.AddCommand(newExceptionsArchiveCmd(), newExceptionsUnarchiveCmd())
		case "pages":
			sub.AddCommand(newPagesAckCmd(), newPagesResolveCmd())
		case "alerts":
			sub.AddCommand(newAlertsEnableCmd(true), newAlertsEnableCmd(false), newAlertsSnoozeCmd())
		case "monitors":
			sub.AddCommand(newMonitorsRunCmd())
		case "post-mortems":
			sub.AddCommand(newPostMortemsCreateCmd(), newPostMortemsUpdateCmd())
		case "endpoints":
			sub.AddCommand(newEndpointsMarkSlowCmd())
		}
	}
}

func runWrite(cmd *cobra.Command, summary []string, result map[string]any, call func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error)) error {
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := confirmMutation(cmd, summary); err != nil {
		return err
	}
	raw, err := call(cmd.Context(), sess.Client(), sess.ProjectID)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}
	if len(raw) == 0 {
		raw, _ = json.Marshal(result)
	}
	return renderRaw(cmd, mode, raw, func(w io.Writer, _ any) error {
		_, err := fmt.Fprintln(w, result["message"])
		return err
	})
}

func newPagesAckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ack <id>",
		Short: "Acknowledge an open page so it stops escalating",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "page id")
			if err != nil {
				return err
			}
			return runWrite(cmd, []string{fmt.Sprintf("About to acknowledge page %d. Escalation stops for everyone.", id)},
				map[string]any{"action": "acknowledge", "id": id, "message": fmt.Sprintf("Acknowledged page %d.", id)},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return nil, c.AcknowledgePage(ctx, projectID, id)
				})
		},
	}
}

func newPagesResolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve <id>",
		Short: "Resolve a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "page id")
			if err != nil {
				return err
			}
			archive, _ := cmd.Flags().GetBool("archive-issue")
			summary := fmt.Sprintf("About to resolve page %d.", id)
			if archive {
				summary = fmt.Sprintf("About to resolve page %d and archive the issue it was opened for.", id)
			}
			return runWrite(cmd, []string{summary},
				map[string]any{"action": "resolve", "id": id, "archivedIssue": archive, "message": fmt.Sprintf("Resolved page %d.", id)},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return nil, c.ResolvePage(ctx, projectID, id, archive)
				})
		},
	}
	cmd.Flags().Bool("archive-issue", false, "Also archive the issue the page was opened for")
	return cmd
}

func newAlertsEnableCmd(enabled bool) *cobra.Command {
	verb, use := "disable", "disable <id>"
	if enabled {
		verb, use = "enable", "enable <id>"
	}
	return &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("%s an alert rule", map[bool]string{true: "Enable", false: "Disable"}[enabled]),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "rule id")
			if err != nil {
				return err
			}
			return runWrite(cmd, []string{fmt.Sprintf("About to %s alert rule %d.", verb, id)},
				map[string]any{"action": verb, "id": id, "message": fmt.Sprintf("Alert rule %d is %sd.", id, verb)},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return c.SetNotificationRuleEnabled(ctx, projectID, id, enabled)
				})
		},
	}
}

func newAlertsSnoozeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snooze <id>",
		Short: "Snooze an alert rule (muted for N minutes, then back on its own)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "rule id")
			if err != nil {
				return err
			}
			minutes, _ := cmd.Flags().GetInt("minutes")
			if minutes < 0 {
				mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
				return renderUsageError(cmd.ErrOrStderr(), mode, "--minutes must be 0 or greater", appdir.Binary+" alerts snooze <id> --minutes 60")
			}
			summary := fmt.Sprintf("About to snooze alert rule %d for %d minutes.", id, minutes)
			message := fmt.Sprintf("Snoozed alert rule %d for %d minutes.", id, minutes)
			if minutes == 0 {
				summary = fmt.Sprintf("About to clear the snooze on alert rule %d.", id)
				message = fmt.Sprintf("Cleared the snooze on alert rule %d.", id)
			}
			return runWrite(cmd, []string{summary},
				map[string]any{"action": "snooze", "id": id, "minutes": minutes, "message": message},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return c.SnoozeNotificationRule(ctx, projectID, id, minutes)
				})
		},
	}
	cmd.Flags().Int("minutes", 60, "Snooze duration in minutes; 0 clears the snooze")
	return cmd
}

func newMonitorsRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <id>",
		Short: "Queue an immediate run of a monitor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "monitor id")
			if err != nil {
				return err
			}
			return runWrite(cmd, []string{fmt.Sprintf("About to queue a run of monitor %d.", id)},
				map[string]any{"action": "run", "id": id, "message": fmt.Sprintf("Queued a run of monitor %d.", id)},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return c.RunCheck(ctx, projectID, id)
				})
		},
	}
}

func readContentFlag(cmd *cobra.Command) (*string, error) {
	path, _ := cmd.Flags().GetString("content-file")
	if path == "" {
		return nil, nil
	}
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(cmd.InOrStdin())
	} else {
		b, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	content := string(b)
	return &content, nil
}

func newPostMortemsCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a post-mortem",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
			title, _ := cmd.Flags().GetString("title")
			if title == "" {
				return renderUsageError(cmd.ErrOrStderr(), mode, "--title is required", appdir.Binary+" post-mortems create --title <title> --content-file notes.md")
			}
			content, err := readContentFlag(cmd)
			if err != nil {
				return renderUsageError(cmd.ErrOrStderr(), mode, "reading --content-file: "+err.Error(), "")
			}
			tags, _ := cmd.Flags().GetStringSlice("tag")
			incident, _ := cmd.Flags().GetInt("incident")
			in := client.PostMortemInput{Title: title, Tags: tags}
			if content != nil {
				in.ContentMd = *content
			}
			if incident > 0 {
				in.IncidentId = &incident
			}
			return runWrite(cmd, []string{fmt.Sprintf("About to create post-mortem %q.", title)},
				map[string]any{"action": "create", "message": "Created the post-mortem."},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return c.CreatePostMortem(ctx, projectID, in)
				})
		},
	}
	cmd.Flags().String("title", "", "Post-mortem title")
	cmd.Flags().String("content-file", "", "Markdown file with the body, or - for stdin")
	cmd.Flags().StringSlice("tag", nil, "Tag (repeatable)")
	cmd.Flags().Int("incident", 0, "Incident id to link")
	return cmd
}

func newPostMortemsUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a post-mortem; only the flags you pass change",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
			id, err := intArg(cmd, args[0], "post-mortem id")
			if err != nil {
				return err
			}
			var patch client.PostMortemPatch
			if cmd.Flags().Changed("title") {
				title, _ := cmd.Flags().GetString("title")
				patch.Title = &title
			}
			if patch.ContentMd, err = readContentFlag(cmd); err != nil {
				return renderUsageError(cmd.ErrOrStderr(), mode, "reading --content-file: "+err.Error(), "")
			}
			if cmd.Flags().Changed("tag") {
				tags, _ := cmd.Flags().GetStringSlice("tag")
				patch.Tags = &tags
			}
			if incident, _ := cmd.Flags().GetInt("incident"); incident > 0 {
				patch.IncidentId = &incident
			}
			patch.UnlinkIncident, _ = cmd.Flags().GetBool("unlink-incident")
			patch.ExpectedUpdatedAt, _ = cmd.Flags().GetString("expected-updated-at")
			if patch.IncidentId != nil && patch.UnlinkIncident {
				return renderUsageError(cmd.ErrOrStderr(), mode, "pass --incident or --unlink-incident, not both", "")
			}
			if patch.ContentMd != nil && patch.ExpectedUpdatedAt == "" {
				return renderUsageError(cmd.ErrOrStderr(), mode, "--content-file replaces the whole body, so --expected-updated-at is required",
					"pass the updatedAt from the '"+appdir.Binary+" post-mortems show <id>' your revision is based on")
			}
			if patch.Title == nil && patch.ContentMd == nil && patch.Tags == nil && patch.IncidentId == nil && !patch.UnlinkIncident {
				return renderUsageError(cmd.ErrOrStderr(), mode, "nothing to update", appdir.Binary+" post-mortems update <id> --title <title> | --content-file <file> --expected-updated-at <t> | --tag <tag> | --incident <id> | --unlink-incident")
			}
			return runWrite(cmd, []string{fmt.Sprintf("About to update post-mortem %d.", id)},
				map[string]any{"action": "update", "id": id, "message": fmt.Sprintf("Updated post-mortem %d.", id)},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return c.UpdatePostMortem(ctx, projectID, id, patch)
				})
		},
	}
	cmd.Flags().String("title", "", "New title")
	cmd.Flags().String("content-file", "", "Markdown file replacing the body, or - for stdin")
	cmd.Flags().StringSlice("tag", nil, "Replace the tags (repeatable)")
	cmd.Flags().Int("incident", 0, "Incident id to link")
	cmd.Flags().Bool("unlink-incident", false, "Remove the incident link")
	cmd.Flags().String("expected-updated-at", "", "updatedAt of the version the edit is based on (from post-mortems show); a newer document fails with a conflict instead of being overwritten")
	return cmd
}

func newEndpointsMarkSlowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mark-slow <endpoint>",
		Short: "Mark an endpoint as expected-slow (--offset-ms 0 removes the mark)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			offset, _ := cmd.Flags().GetUint32("offset-ms")
			reason, _ := cmd.Flags().GetString("reason")
			endpoint := args[0]
			prompt := fmt.Sprintf("About to set the accepted latency of %q to %dms.", endpoint, offset)
			message := fmt.Sprintf("Set the accepted latency of %s to %dms.", endpoint, offset)
			if offset == 0 {
				prompt = fmt.Sprintf("About to remove the expected-slow mark from %q.", endpoint)
				message = fmt.Sprintf("Removed the expected-slow mark from %s.", endpoint)
			}
			return runWrite(cmd, []string{prompt},
				map[string]any{"action": "mark-slow", "endpoint": endpoint, "offsetMs": offset, "message": message},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return nil, c.SetSlowEndpoint(ctx, projectID, client.SetSlowEndpointRequest{Endpoint: endpoint, OffsetMs: offset, Reason: reason})
				})
		},
	}
	cmd.Flags().Uint32("offset-ms", 0, "Accepted extra latency in milliseconds; 0 removes the mark (required)")
	cmd.Flags().String("reason", "", "Why this latency is acceptable")
	// Required because 0 removes the mark: a forgotten flag must not.
	_ = cmd.MarkFlagRequired("offset-ms")
	return cmd
}
