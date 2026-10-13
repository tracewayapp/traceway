package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
	"github.com/tracewayapp/traceway/cli/internal/output"
)

var version = "dev"

type versionInfo struct {
	Version string `json:"version"`
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the traceway CLI version",
		Args:  cobra.NoArgs,
		RunE:  runVersion,
	}
}

func runVersion(cmd *cobra.Command, _ []string) error {
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	info := versionInfo{Version: version}
	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), info, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), info, output.ParseFieldsFlag(flagFields))
	default:
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s version %s\n", appdir.Binary, version)
		return err
	}
}
