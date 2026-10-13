//go:build write

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func newExceptionsArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <hash> [<hash>...]",
		Short: "Archive one or more exception groups",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExceptionsMutation(cmd, args, "archive",
				func(c *client.Client, ctx context.Context, projectID string, hashes []string) error {
					return c.ArchiveExceptions(ctx, projectID, hashes)
				})
		},
	}
}

func newExceptionsUnarchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unarchive <hash> [<hash>...]",
		Short: "Unarchive one or more exception groups",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExceptionsMutation(cmd, args, "unarchive",
				func(c *client.Client, ctx context.Context, projectID string, hashes []string) error {
					return c.UnarchiveExceptions(ctx, projectID, hashes)
				})
		},
	}
}

// runExceptionsMutation is the shared body for archive and unarchive. The
// 'verb' parameter controls the prompt wording and the rendered action label;
// 'doIt' is the client method to call after confirmation passes.
func runExceptionsMutation(
	cmd *cobra.Command,
	hashes []string,
	verb string,
	doIt func(c *client.Client, ctx context.Context, projectID string, hashes []string) error,
) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
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

	c := sess.Client()
	if err := doIt(c, ctx, sess.ProjectID, hashes); err != nil {
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
