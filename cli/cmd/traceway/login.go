package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/internal/state"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

const defaultURL = "https://cloud.traceway.com"

// login-specific flag values
var (
	loginURL          string
	loginUsername     string
	loginPasswordFile bool
	loginPassword     bool
	loginToken        string
	loginTokenStdin   bool
	loginNoBrowser    bool
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against a Traceway instance and store the token",
		Long: "Authenticate against a Traceway instance and store the token.\n\n" +
			"By default this starts a browser device-login flow: the CLI prints a URL\n" +
			"and a short code, you approve in your browser, and the CLI receives a token.\n" +
			"Use --password for email/password login, or --token to store a personal\n" +
			"access token.",
		RunE: runLogin,
	}
	cmd.Flags().StringVar(&loginURL, "url", "", "Traceway base URL (default: existing or "+defaultURL+")")
	cmd.Flags().StringVar(&loginUsername, "username", "", "Email address for password login, implies --password (default: existing or interactive prompt)")
	cmd.Flags().BoolVar(&loginPasswordFile, "password-stdin", false, "Read password from stdin (implies --password)")
	cmd.Flags().BoolVar(&loginPassword, "password", false, "Authenticate with email + password instead of the browser device flow")
	cmd.Flags().StringVar(&loginToken, "token", "", "Authenticate with a personal access token")
	cmd.Flags().BoolVar(&loginTokenStdin, "token-stdin", false, "Read a personal access token from stdin")
	cmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "Do not attempt to open a browser during the device flow")
	return cmd
}

func runLogin(cmd *cobra.Command, _ []string) error {
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	tokenMode := loginToken != "" || loginTokenStdin
	// --username selected the password flow before the device flow became the
	// default; keep it doing so instead of silently ignoring it for auth.
	passwordMode := loginPassword || loginPasswordFile || loginUsername != ""

	if tokenMode && passwordMode {
		return renderUsageError(cmd.ErrOrStderr(), mode, "choose one of --token or --password/--username, not both", "traceway login --help")
	}
	if loginToken != "" && loginTokenStdin {
		return renderUsageError(cmd.ErrOrStderr(), mode, "choose one of --token or --token-stdin, not both", "traceway login --help")
	}

	switch {
	case tokenMode:
		return runLoginToken(cmd)
	case passwordMode:
		return runLoginPassword(cmd)
	default:
		return runLoginWeb(cmd)
	}
}

func runLoginPassword(cmd *cobra.Command) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	cfg, st, profileName, url, err := loadLoginContext()
	if err != nil {
		return err
	}

	username := loginUsername
	if username == "" {
		if existing, ok := cfg.Profiles[profileName]; ok {
			username = existing.Username
		}
		if username == "" {
			username, err = promptUsername(cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
		}
	}

	password, err := readPassword(cmd.InOrStdin(), cmd.OutOrStdout(), loginPasswordFile)
	if err != nil {
		return err
	}

	jwt, err := client.New(url).Login(ctx, username, password)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, true)
	}

	if err := saveProfileCredentials(cfg, st, profileName, url, username, credential{Kind: state.KindPassword, AccessToken: jwt}); err != nil {
		return err
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s on %s (profile: %s)\n", username, url, profileName)
	return err
}

// loadLoginContext loads config + state and resolves the active profile name
// and base URL, shared by all login paths.
func loadLoginContext() (*config.Config, *state.State, string, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, "", "", err
	}
	st, err := state.Load()
	if err != nil {
		return nil, nil, "", "", err
	}

	profileName := resolveProfileName(st)

	url := loginURL
	if url == "" {
		if existing, ok := cfg.Profiles[profileName]; ok && existing.URL != "" {
			url = existing.URL
		} else {
			url = defaultURL
		}
	}
	return cfg, st, profileName, url, nil
}

func promptUsername(in io.Reader, out io.Writer) (string, error) {
	_, _ = fmt.Fprint(out, "Username: ")
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readPassword(in io.Reader, out io.Writer, fromStdin bool) (string, error) {
	if fromStdin {
		r := bufio.NewReader(in)
		line, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	// Interactive: read with no echo if stdin is a real terminal.
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		_, _ = fmt.Fprint(out, "Password: ")
		bytes, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(out)
		return string(bytes), err
	}
	// Fallback: line-based read (covers test injection).
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
