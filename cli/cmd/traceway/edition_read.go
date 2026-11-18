//go:build !write

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/v2/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

const (
	editionWrite   = false
	editionShort   = "CLI for the Traceway observability platform (read-only)"
	deviceClientID = "traceway-cli"
	loginScope     = client.ScopeRead
)

// writeOnlyCommands are the write build's subcommands. The read build answers
// them with a pointer to traceway-write instead of printing the parent's help,
// which would read as if the command half-worked.
var writeOnlyCommands = map[string][]string{
	"exceptions":   {"archive", "unarchive"},
	"pages":        {"ack", "resolve"},
	"alerts":       {"enable", "disable", "snooze", "create", "update", "delete"},
	"monitors":     {"run"},
	"post-mortems": {"create", "update"},
	"endpoints":    {"mark-slow"},
	"opamp rules":  {"create", "update", "delete", "reorder"},
}

func init() {
	for _, op := range client.AlertingOperations() {
		if op.ReadOnly {
			continue
		}
		found := false
		for _, name := range writeOnlyCommands[op.Resource] {
			if name == op.Action {
				found = true
				break
			}
		}
		if !found {
			writeOnlyCommands[op.Resource] = append(writeOnlyCommands[op.Resource], op.Action)
		}
	}
}

func addWriteCommands(root *cobra.Command) {
	addAlertingConfigurationCommands(root, false)
	for path, names := range writeOnlyCommands {
		sub, _, err := root.Find(strings.Fields(path))
		if err != nil || sub == root {
			continue
		}
		for _, name := range names {
			sub.AddCommand(&cobra.Command{
				Use:                name,
				Hidden:             true,
				DisableFlagParsing: true,
				RunE: func(cmd *cobra.Command, _ []string) error {
					mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
					_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
						Code:     "write_required",
						Message:  "'" + path + " " + name + "' changes data, and this traceway CLI is read-only",
						Hint:     "install the separate traceway-write CLI (install.sh --write), log in with it, and ask an organization admin to turn on Settings > Agent Access",
						ExitCode: exitcode.Usage,
					})
					return newCLIError(exitcode.Usage, "write_required")
				},
			})
		}
	}
}

func runAlertingMutation(_ *cobra.Command, _ []string, _ map[string]any, _ func(context.Context, *client.Client, string) (json.RawMessage, error)) error {
	return fmt.Errorf("mutations require traceway-write")
}
