//go:build write

package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func newTokensCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Create and list your read-only personal access tokens",
	}
	cmd.AddCommand(newTokensCreateCmd(), newTokensListCmd())
	return cmd
}

func newTokensCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a read-only personal access token (twp_) for your account",
		Long: "Create a read-only personal access token (twp_) for the signed-in account.\n\n" +
			"The token reads everything you can read in Traceway and issues dashboard\n" +
			"links, for example for a host application that embeds a dashboard. It\n" +
			"never expires unless --expires-in-days is given, and only the dashboard\n" +
			"(Account > Personal Access Tokens) revokes it. The CLI cannot create write\n" +
			"tokens. The token is printed once: hand it straight to the secret store\n" +
			"that needs it and do not log it.\n\n" +
			"Creating a token is an agent write: the selected project's organization\n" +
			"must allow agent writes, and the creation appears in its agent activity log.",
		Example: "  traceway-write tokens create --name \"Acme admin embed\" --yes --output json\n" +
			"  traceway-write tokens create --name \"CI read\" --expires-in-days 30 --yes",
		Args: cobra.NoArgs,
		RunE: runTokensCreate,
	}
	cmd.Flags().String("name", "", "Name shown in Account > Personal Access Tokens (required, at most 100 characters)")
	cmd.Flags().Int("expires-in-days", 0, fmt.Sprintf("Lifetime in whole days, 1 to %d (default: never expires)", client.PersonalAccessTokenMaxDays))
	return cmd
}

func runTokensCreate(cmd *cobra.Command, _ []string) error {
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	name, _ := cmd.Flags().GetString("name")
	days, _ := cmd.Flags().GetInt("expires-in-days")
	if cmd.Flags().Changed("expires-in-days") && days < 1 {
		return renderUsageError(cmd.ErrOrStderr(), mode, fmt.Sprintf("--expires-in-days must be between 1 and %d", client.PersonalAccessTokenMaxDays), cmd.CommandPath()+" --help")
	}
	if err := client.ValidatePersonalAccessToken(name, days); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), cmd.CommandPath()+" --help")
	}
	validity := "It never expires"
	if days > 0 {
		validity = fmt.Sprintf("It expires in %d days", days)
	}
	summary := fmt.Sprintf("About to create the read-only personal access token %q for your account. %s; only the dashboard can revoke it.", name, validity)

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}
	if err := confirmMutation(cmd, []string{summary}); err != nil {
		return err
	}
	raw, err := sess.Client().CreateReadToken(cmd.Context(), sess.ProjectID, name, days)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}
	return renderRaw(cmd, mode, raw, func(w io.Writer, v any) error {
		token, _ := v.(map[string]any)
		expires := "never"
		if at, ok := token["expiresAt"].(string); ok {
			expires = at
		}
		_, err := fmt.Fprintf(w, "%v\nName %v, expires %s. It is shown only once.\n", token["token"], token["name"], expires)
		return err
	})
}

func newTokensListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List your personal access tokens (names, prefixes, scopes; never the tokens)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
			sess, err := loadSession()
			if err != nil {
				return renderSessionError(cmd.ErrOrStderr(), mode, err)
			}
			raw, err := sess.Client().ListPersonalAccessTokens(cmd.Context())
			if err != nil {
				return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
			}
			return renderRaw(cmd, mode, raw, func(w io.Writer, v any) error {
				tokens, _ := v.([]any)
				if len(tokens) == 0 {
					_, err := fmt.Fprintln(w, "No personal access tokens.")
					return err
				}
				for _, item := range tokens {
					t, _ := item.(map[string]any)
					expires := "never"
					if at, ok := t["expiresAt"].(string); ok {
						expires = at
					}
					if _, err := fmt.Fprintf(w, "%v…\t%v\t%v\texpires %s\n", t["prefix"], t["name"], t["scope"], expires); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
}
