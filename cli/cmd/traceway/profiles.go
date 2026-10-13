package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/internal/state"
)

func newProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage stored Traceway profiles",
	}
	cmd.AddCommand(newProfilesListCmd())
	cmd.AddCommand(newProfilesUseCmd())
	return cmd
}

func newProfilesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		RunE:  runProfilesList,
	}
}

type profileSummary struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Current  bool   `json:"current"`
}

type profilesListResponse struct {
	Current string           `json:"current"`
	Data    []profileSummary `json:"data"`
}

func runProfilesList(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := state.Load()
	if err != nil {
		return err
	}
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	// Build a union of names from config and state.
	nameSet := make(map[string]struct{})
	for n := range cfg.Profiles {
		nameSet[n] = struct{}{}
	}
	for n := range st.Profiles {
		nameSet[n] = struct{}{}
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)

	resp := profilesListResponse{Current: st.CurrentProfile}
	for _, n := range names {
		p := cfg.Profiles[n]
		resp.Data = append(resp.Data, profileSummary{
			Name:     n,
			URL:      p.URL,
			Username: p.Username,
			Current:  n == st.CurrentProfile,
		})
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		tw := output.NewTabWriter(cmd.OutOrStdout())
		_, _ = fmt.Fprintln(tw, " \tNAME\tURL\tUSERNAME")
		for _, p := range resp.Data {
			marker := " "
			if p.Current {
				marker = "*"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", marker, p.Name, p.URL, p.Username)
		}
		return tw.Flush()
	}
}

func newProfilesUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "Set the current profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfilesUse,
	}
}

func runProfilesUse(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := state.Load()
	if err != nil {
		return err
	}
	name := args[0]
	_, inCfg := cfg.Profiles[name]
	_, inState := st.Profiles[name]
	if !inCfg && !inState {
		mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
		_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
			Code:     "no_profile",
			Message:  fmt.Sprintf("profile %q does not exist", name),
			Hint:     appdir.Binary + " profiles list",
			ExitCode: exitcode.Auth,
		})
		return newCLIError(exitcode.Auth, "no_profile")
	}
	st.CurrentProfile = name
	if err := st.Save(); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Now using profile %q\n", name)
	return nil
}
