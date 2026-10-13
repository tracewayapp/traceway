//go:build !write

package main

import (
	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
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
	"alerts":       {"enable", "disable", "snooze"},
	"monitors":     {"run"},
	"post-mortems": {"create", "update"},
	"endpoints":    {"mark-slow"},
}

func addWriteCommands(root *cobra.Command) {
	for _, sub := range root.Commands() {
		for _, name := range writeOnlyCommands[sub.Name()] {
			sub.AddCommand(&cobra.Command{
				Use:                name,
				Hidden:             true,
				DisableFlagParsing: true,
				RunE: func(cmd *cobra.Command, _ []string) error {
					mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
					_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
						Code:     "write_required",
						Message:  "'" + cmd.Parent().Name() + " " + name + "' changes data, and this traceway CLI is read-only",
						Hint:     "install the separate traceway-write CLI (install.sh --write), log in with it, and ask an organization admin to turn on Settings > Agent Access",
						ExitCode: exitcode.Usage,
					})
					return newCLIError(exitcode.Usage, "write_required")
				},
			})
		}
	}
}
