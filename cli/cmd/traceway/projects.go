package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/internal/state"
)

func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List and switch projects",
	}
	cmd.AddCommand(newProjectsListCmd())
	cmd.AddCommand(newProjectsUseCmd())
	return cmd
}

func newProjectsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List projects visible to the authenticated user",
		RunE:  runProjectsList,
	}
}

func runProjectsList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := state.Load()
	if err != nil {
		return err
	}

	profileName := resolveProfileName(st)

	cfgProfile, hasCfg := cfg.Profiles[profileName]
	stateProfile, hasState := st.Profiles[profileName]

	if !hasCfg || cfgProfile.URL == "" {
		_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
			Code:     "not_authenticated",
			Message:  fmt.Sprintf("profile %q not found", profileName),
			Hint:     "traceway login",
			ExitCode: exitcode.Auth,
		})
		return newCLIError(exitcode.Auth, "not_authenticated")
	}
	if !hasState || stateProfile.JWT == "" {
		_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
			Code:     "not_authenticated",
			Message:  fmt.Sprintf("profile %q has no JWT; please login first", profileName),
			Hint:     "traceway login",
			ExitCode: exitcode.Auth,
		})
		return newCLIError(exitcode.Auth, "not_authenticated")
	}

	c := newRefreshingClient(cfgProfile.URL, profileName, stateProfile.JWT, stateProfile.RefreshToken, stateProfile.CredentialKind)
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), projects, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), projects, output.ParseFieldsFlag(flagFields))
	default:
		tw := output.NewTabWriter(cmd.OutOrStdout())
		_, _ = fmt.Fprintln(tw, "ID\tNAME")
		for _, p := range projects {
			_, _ = fmt.Fprintf(tw, "%s\t%s\n", p.ID, p.Name)
		}
		return tw.Flush()
	}
}

func newProjectsUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <project-id>",
		Short: "Set the current project for the active profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectsUse,
	}
}

func runProjectsUse(cmd *cobra.Command, args []string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}

	profileName := resolveProfileName(st)

	if st.Profiles == nil {
		st.Profiles = map[string]state.ProfileState{}
	}
	ps := st.Profiles[profileName]
	ps.CurrentProjectID = args[0]
	st.Profiles[profileName] = ps

	if err := st.Save(); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Now using project %q for profile %q\n", args[0], profileName)
	return nil
}
