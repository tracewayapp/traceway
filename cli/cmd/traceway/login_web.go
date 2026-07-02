package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/internal/state"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func runLoginWeb(cmd *cobra.Command) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	out := cmd.OutOrStdout()

	// The device flow needs a human to approve the code in a browser. In a
	// non-interactive context (CI, scripts) nobody will, so it would otherwise
	// poll uselessly until the ~10-minute deadline — fail fast and point at the
	// non-interactive login modes instead. --no-browser is the explicit opt-in
	// for driving the flow manually (e.g. a headless box where the code is
	// approved from another device), so it suppresses the guard.
	if !loginNoBrowser && !output.StdoutIsTerminal() {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"the browser device-login flow requires an interactive terminal",
			"use 'traceway login --token <token>' or 'traceway login --password' for non-interactive login")
	}

	cfg, st, profileName, url, err := loadLoginContext()
	if err != nil {
		return err
	}

	c := client.New(url)
	da, err := c.DeviceAuthorize(ctx, "")
	if err != nil {
		// A backend that predates the device flow 404s the authorize endpoint,
		// or serves the SPA's index.html (a JSON syntax error). Point at the
		// password flow instead of surfacing an opaque error.
		var syntaxErr *json.SyntaxError
		if errors.Is(err, client.ErrNotFound) || errors.As(err, &syntaxErr) {
			_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
				Code:     "device_login_unsupported",
				Message:  "this server does not support the browser device-login flow (it may be running an older Traceway version)",
				Hint:     "traceway login --password",
				ExitCode: exitcode.Auth,
			})
			return newCLIError(exitcode.Auth, "device_login_unsupported")
		}
		return renderAPIError(cmd.ErrOrStderr(), mode, err, true)
	}

	openURL := da.VerificationURIComplete
	if openURL == "" {
		openURL = da.VerificationURI
	}

	_, _ = fmt.Fprintf(out, "\nTo sign in, open this URL in your browser:\n\n    %s\n\n", da.VerificationURI)
	_, _ = fmt.Fprintf(out, "And enter the code:  %s\n\n", da.UserCode)

	if !loginNoBrowser && output.StdoutIsTerminal() {
		_ = openBrowser(openURL)
	}
	_, _ = fmt.Fprintln(out, "Waiting for authorization...")

	interval := da.Interval
	if interval < 1 {
		interval = 5
	}
	deadline := time.Now().Add(10 * time.Minute)
	if da.ExpiresIn > 0 {
		deadline = time.Now().Add(time.Duration(da.ExpiresIn) * time.Second)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(interval) * time.Second):
		}

		if time.Now().After(deadline) {
			return renderDeviceError(cmd.ErrOrStderr(), mode, "device_code_expired", "the login request expired before it was approved")
		}

		ts, err := c.PollDeviceToken(ctx, da.DeviceCode)
		if err != nil {
			switch {
			case errors.Is(err, client.ErrAuthorizationPending):
				continue
			case errors.Is(err, client.ErrSlowDown):
				interval += 5
				continue
			case errors.Is(err, client.ErrAccessDenied):
				return renderDeviceError(cmd.ErrOrStderr(), mode, "access_denied", "the login request was denied")
			case errors.Is(err, client.ErrExpiredToken):
				return renderDeviceError(cmd.ErrOrStderr(), mode, "device_code_expired", "the login request expired before it was approved")
			default:
				return renderAPIError(cmd.ErrOrStderr(), mode, err, true)
			}
		}

		username := ts.Email
		if username == "" {
			username = loginUsername
		}
		cred := credential{
			Kind:         state.KindDevice,
			AccessToken:  ts.AccessToken,
			RefreshToken: ts.RefreshToken,
			ExpiresAt:    expiresAtUnix(ts.ExpiresIn),
		}
		if err := saveProfileCredentials(cfg, st, profileName, url, username, cred); err != nil {
			return err
		}

		who := username
		if who == "" {
			who = "you"
		}
		_, err = fmt.Fprintf(out, "\nLogged in as %s on %s (profile: %s)\n", who, url, profileName)
		return err
	}
}

func renderDeviceError(errOut io.Writer, mode output.Mode, code, msg string) error {
	_ = output.RenderError(errOut, mode, output.ErrorEnvelope{
		Code:     code,
		Message:  msg,
		Hint:     "traceway login",
		ExitCode: exitcode.Auth,
	})
	return newCLIError(exitcode.Auth, code)
}

func openBrowser(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	return exec.Command(name, args...).Start()
}
