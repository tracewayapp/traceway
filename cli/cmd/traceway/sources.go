package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/internal/state"
	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/access/traceway"
)

func newSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage the telemetry sources bound to a profile",
		Long: `A profile always queries its own Traceway instance, listed as the implicit
"traceway" source. Further sources (another Traceway instance today, other
providers as their adapters land) are bound by name with "sources add" and
answer the domains they are given; commands merge every source that answers
a domain unless --source picks one.`,
	}
	cmd.AddCommand(newSourcesListCmd())
	cmd.AddCommand(newSourcesAddCmd())
	cmd.AddCommand(newSourcesRemoveCmd())
	return cmd
}

type sourceSummary struct {
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	Domains  []string `json:"domains"`
	Implicit bool     `json:"implicit"`
}

type sourcesListResponse struct {
	Profile string          `json:"profile"`
	Data    []sourceSummary `json:"data"`
}

func newSourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the sources bound to the current profile",
		RunE:  runSourcesList,
	}
}

func runSourcesList(cmd *cobra.Command, _ []string) error {
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	cfg, profileName, err := loadProfileConfig(cmd, mode)
	if err != nil {
		return err
	}
	resp := sourcesListResponse{Profile: profileName, Data: []sourceSummary{{
		Name:     traceway.DefaultName,
		Provider: traceway.Provider,
		Domains:  domainNames(access.Domains),
		Implicit: true,
	}}}
	for _, src := range cfg.Profiles[profileName].Sources {
		resp.Data = append(resp.Data, sourceSummary{Name: src.Name, Provider: src.Provider, Domains: answeredDomains(src)})
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		tw := output.NewTabWriter(cmd.OutOrStdout())
		_, _ = fmt.Fprintln(tw, "NAME\tPROVIDER\tDOMAINS\tIMPLICIT")
		for _, src := range resp.Data {
			implicit := ""
			if src.Implicit {
				implicit = "yes"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", src.Name, src.Provider, strings.Join(src.Domains, ","), implicit)
		}
		return tw.Flush()
	}
}

func newSourcesAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <provider>",
		Short: "Bind a telemetry source to the current profile",
		Long: `Bind a source of the given provider. --name is how commands refer to it
(--source <name>); --domains limits what it answers, defaulting to every
domain the provider implements. Provider settings, credentials included, are
passed as --set key=value and stored in the profile's config file.

Registered providers: ` + strings.Join(access.Providers(), ", ") + `.

  traceway sources add traceway --name staging --domains logs,exceptions \
    --set url=https://staging.example.com --set token=twp_...`,
		Args: cobra.ExactArgs(1),
		RunE: runSourcesAdd,
	}
	cmd.Flags().String("name", "", "Name of the source (required)")
	cmd.Flags().String("domains", "", "Comma-separated domains the source answers: "+strings.Join(domainNames(access.Domains), ", "))
	cmd.Flags().StringSlice("set", nil, "Provider setting as key=value (repeatable)")
	return cmd
}

func runSourcesAdd(cmd *cobra.Command, args []string) error {
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	provider := args[0]
	if !slices.Contains(access.Providers(), provider) {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			fmt.Sprintf("unknown provider %q", provider),
			"registered providers: "+strings.Join(access.Providers(), ", "))
	}
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "--name is required", "traceway sources add "+provider+" --name <name>")
	}
	if name == traceway.DefaultName {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			fmt.Sprintf("%q is the profile's own instance and always present", name),
			"pick another --name")
	}
	domainsFlag, _ := cmd.Flags().GetString("domains")
	domains, err := parseDomains(domainsFlag)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(),
			"--domains takes a comma-separated subset of: "+strings.Join(domainNames(access.Domains), ", "))
	}
	settingsFlag, _ := cmd.Flags().GetStringSlice("set")
	settings, err := parseKeyValues("--set", settingsFlag)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), "use --set key=value (repeatable)")
	}

	cfg, profileName, err := loadProfileConfig(cmd, mode)
	if err != nil {
		return err
	}
	profile := cfg.Profiles[profileName]
	for _, existing := range profile.Sources {
		if existing.Name == name {
			return renderUsageError(cmd.ErrOrStderr(), mode,
				fmt.Sprintf("source %q already exists on profile %q", name, profileName),
				"traceway sources remove "+name)
		}
	}
	source := config.Source{Name: name, Provider: provider, Domains: domains, Config: settings}
	if _, err := openSources([]config.Source{source}); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), "check --domains and the --set values the provider needs")
	}
	profile.Sources = append(profile.Sources, source)
	cfg.Profiles[profileName] = profile
	if err := cfg.Save(); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Added source %q (%s) to profile %q\n", name, provider, profileName)
	return nil
}

func newSourcesRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Unbind a telemetry source from the current profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runSourcesRemove,
	}
}

func runSourcesRemove(cmd *cobra.Command, args []string) error {
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	name := args[0]
	if name == traceway.DefaultName {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			fmt.Sprintf("%q is the profile's own instance and cannot be removed", name), "")
	}
	cfg, profileName, err := loadProfileConfig(cmd, mode)
	if err != nil {
		return err
	}
	profile := cfg.Profiles[profileName]
	index := slices.IndexFunc(profile.Sources, func(s config.Source) bool { return s.Name == name })
	if index < 0 {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			fmt.Sprintf("profile %q has no source %q", profileName, name), "traceway sources list")
	}
	profile.Sources = slices.Delete(profile.Sources, index, index+1)
	cfg.Profiles[profileName] = profile
	if err := cfg.Save(); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed source %q from profile %q\n", name, profileName)
	return nil
}

// loadProfileConfig loads the config file and resolves the profile the
// sources commands act on, which must exist in the config.
func loadProfileConfig(cmd *cobra.Command, mode output.Mode) (*config.Config, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, "", err
	}
	st, err := state.Load()
	if err != nil {
		return nil, "", err
	}
	profileName := resolveProfileName(st)
	if _, ok := cfg.Profiles[profileName]; !ok {
		_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
			Code:     "no_profile",
			Message:  fmt.Sprintf("profile %q does not exist", profileName),
			Hint:     "traceway login",
			ExitCode: exitcode.Auth,
		})
		return nil, "", newCLIError(exitcode.Auth, "no_profile")
	}
	return cfg, profileName, nil
}

func parseDomains(flag string) ([]string, error) {
	if strings.TrimSpace(flag) == "" {
		return nil, nil
	}
	var domains []string
	for _, raw := range strings.Split(flag, ",") {
		domain := strings.TrimSpace(raw)
		if domain == "" {
			continue
		}
		if !slices.Contains(access.Domains, access.Domain(domain)) {
			return nil, fmt.Errorf("unknown domain %q", domain)
		}
		domains = append(domains, domain)
	}
	return domains, nil
}

func domainNames(domains []access.Domain) []string {
	names := make([]string, 0, len(domains))
	for _, d := range domains {
		names = append(names, string(d))
	}
	return names
}

// answeredDomains is what a bound source answers: its explicit domain list,
// else everything its provider implements. A source that no longer opens
// (a provider removed, settings gone) reports the failure in place.
func answeredDomains(src config.Source) []string {
	if len(src.Domains) > 0 {
		return src.Domains
	}
	bound, err := openSources([]config.Source{src})
	if err != nil {
		return []string{"(cannot open: " + err.Error() + ")"}
	}
	return domainNames(access.ImplementedDomains(bound[0].Source))
}
