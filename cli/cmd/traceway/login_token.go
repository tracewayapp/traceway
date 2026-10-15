package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
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
		return renderUsageError(cmd.ErrOrStderr(), mode, "no token provided", appdir.Binary+" login --token <token>")
	}
	if !editionWrite && strings.HasPrefix(token, "tww_") {
		return renderUsageError(cmd.ErrOrStderr(), mode, "this is a write token (tww_), and the read-only traceway CLI does not keep write credentials",
			"use it with 'traceway-write login --token', or create a Read only token (twp_) under Account > Personal Access Tokens")
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

	if editionWrite && strings.HasPrefix(token, "twp_") {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning: this is a read-only token (twp_), so every write will be refused. Create a Read and write token (tww_) under Account > Personal Access Tokens.")
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
