package main

import (
	"github.com/spf13/cobra"
)

// Global flag values, populated by Cobra at flag-parse time.
var (
	flagProfile string
	flagProject string
	flagOutput  string
	flagFields  string
	flagYes     bool
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "traceway",
		Short:         "CLI for the Traceway observability platform",
		Version:       version,
		SilenceUsage:  true, // we render our own error envelopes
		SilenceErrors: true,
	}

	pf := cmd.PersistentFlags()
	pf.StringVar(&flagProfile, "profile", "", "Profile name (default: current profile, then \"default\")")
	pf.StringVar(&flagProject, "project", "", "Project ID (default: profile's current project)")
	pf.StringVarP(&flagOutput, "output", "o", "", "Output format: json, yaml, or table (default: table on TTY, json otherwise)")
	pf.StringVar(&flagFields, "fields", "", "Comma-separated field projection (e.g. id,name)")
	pf.BoolVar(&flagYes, "yes", false, "Skip confirmation for mutating commands")

	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newProfilesCmd())
	cmd.AddCommand(newProjectsCmd())
	cmd.AddCommand(newExceptionsCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newEndpointsCmd())
	cmd.AddCommand(newTasksCmd())
	cmd.AddCommand(newAiTracesCmd())
	cmd.AddCommand(newSessionsCmd())
	cmd.AddCommand(newTracesCmd())
	cmd.AddCommand(newMetricsCmd())
	cmd.AddCommand(newMcpCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}
