package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/internal/state"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func runLoginToken(cmd *cobra.Command) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	cfg, st, profileName, url, err := loadLoginContext()
	if err != nil {
		return err
	}

	token := strings.TrimSpace(loginToken)
	if loginTokenStdin {
		token, err = readTokenStdin(cmd.InOrStdin())
		if err != nil {
			return err
		}
	}
	if token == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "no token provided", "traceway login --token <token>")
	}

	username := loginUsername
	if username == "" {
		if existing, ok := cfg.Profiles[profileName]; ok {
			username = existing.Username
		}
	}

	if _, err := client.New(url, client.WithJWT(token)).ListProjects(ctx); err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}

	if err := saveProfileCredentials(cfg, st, profileName, url, username, credential{Kind: state.KindPAT, AccessToken: token}); err != nil {
		return err
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Stored personal access token for %s (profile: %s)\n", url, profileName)
	return err
}

func readTokenStdin(in io.Reader) (string, error) {
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
