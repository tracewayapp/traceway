//go:build write

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func addOpampWriteCommands(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		if sub.Name() == "rules" {
			sub.AddCommand(newOpampSaveRuleCmd(false), newOpampSaveRuleCmd(true), newOpampDeleteRuleCmd(), newOpampReorderRulesCmd())
		}
	}
}

func newOpampSaveRuleCmd(update bool) *cobra.Command {
	verb, use := "create", "create"
	args := cobra.NoArgs
	if update {
		verb, use, args = "update", "update <id>", cobra.ExactArgs(1)
	}
	cmd := &cobra.Command{
		Use: use, Short: "Save a complete rule: name, enabled, conditions and settings", Args: args,
		RunE: func(cmd *cobra.Command, args []string) error {
			var in client.SDKConfigRuleInput
			if err := readOpampJSON(cmd, &in); err != nil {
				return opampUsageError(cmd, err)
			}
			if strings.TrimSpace(in.Name) == "" || in.Conditions == nil || in.Settings == nil {
				return opampUsageError(cmd, fmt.Errorf("--file must contain name, conditions and settings (use {} for all clients)"))
			}
			id := 0
			if update {
				var err error
				id, err = intArg(cmd, args[0], "rule id")
				if err != nil {
					return err
				}
				in.ExpectedUpdatedAt, _ = cmd.Flags().GetString("expected-updated-at")
				if _, err := time.Parse(time.RFC3339Nano, in.ExpectedUpdatedAt); err != nil {
					return opampUsageError(cmd, fmt.Errorf("--expected-updated-at must be the updatedAt timestamp from rules list"))
				}
			}
			body, _ := json.Marshal(in)
			return runWrite(cmd, []string{fmt.Sprintf("About to %s OpAMP rule %q (id %d). Settings affect matching browsers at their next poll.", verb, in.Name, id), string(body)},
				map[string]any{"action": verb, "message": "Saved the OpAMP rule."},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					if update {
						return c.UpdateSDKConfigRule(ctx, projectID, id, in)
					}
					return c.CreateSDKConfigRule(ctx, projectID, in)
				})
		},
	}
	addOpampFileFlag(cmd)
	if update {
		cmd.Flags().String("expected-updated-at", "", "updatedAt from rules list; stale edits fail with 409")
		_ = cmd.MarkFlagRequired("expected-updated-at")
	}
	return cmd
}

func newOpampDeleteRuleCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <id>", Short: "Delete a rule; earlier rules and SDK defaults take over", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := intArg(cmd, args[0], "rule id")
			if err != nil {
				return err
			}
			return runWrite(cmd, []string{fmt.Sprintf("About to delete OpAMP rule %d. Earlier rules and SDK defaults take over.", id)},
				map[string]any{"action": "delete", "id": id, "message": "Deleted the OpAMP rule."},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return nil, c.DeleteSDKConfigRule(ctx, projectID, id)
				})
		},
	}
}

func newOpampReorderRulesCmd() *cobra.Command {
	return &cobra.Command{
		Use: "reorder <id>...", Short: "Set every rule's evaluation order; list every id exactly once", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]int, 0, len(args))
			for _, arg := range args {
				id, err := intArg(cmd, arg, "rule id")
				if err != nil {
					return err
				}
				ids = append(ids, id)
			}
			return runWrite(cmd, []string{fmt.Sprintf("About to reorder OpAMP rules to %v. Later matching rules win.", ids)},
				map[string]any{"action": "reorder", "ids": ids, "message": "Reordered the OpAMP rules."},
				func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
					return nil, c.ReorderSDKConfigRules(ctx, projectID, ids)
				})
		},
	}
}
