package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/internal/state"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// session bundles everything a query command needs after resolving config,
// state, the active profile, and the project ID. Built by loadSession.
type session struct {
	ProfileName  string
	URL          string
	Username     string
	JWT          string
	RefreshToken string
	Kind         string
	ProjectID    string
}

// Sentinel errors so the caller can map them to the right error envelope.
var (
	errSessionNoProfile = errors.New("session: no profile configured")
	errSessionNoJWT     = errors.New("session: profile has no stored token")
	errSessionNoProject = errors.New("session: no project selected")
)

// loadSession reads config + state, resolves the active profile and project,
// and returns a session. Returns one of the errSession* sentinels on common
// "you need to configure something" failures so callers can render the
// matching error envelope.
func loadSession() (*session, error) {
	return loadSessionOpts(true)
}

// loadSessionOpts is loadSession with the project requirement optional: the
// MCP server is usable without a current project (list_projects works and
// every tool accepts project_id), so it passes requireProject=false.
func loadSessionOpts(requireProject bool) (*session, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	st, err := state.Load()
	if err != nil {
		return nil, fmt.Errorf("loading state: %w", err)
	}

	name := resolveProfileName(st)
	cp, hasCfg := cfg.Profiles[name]
	if !hasCfg {
		return nil, fmt.Errorf("%w: %q", errSessionNoProfile, name)
	}

	sp, hasState := st.Profiles[name]
	if !hasState || sp.JWT == "" {
		return nil, fmt.Errorf("%w: %q", errSessionNoJWT, name)
	}

	projectID := flagProject
	if projectID == "" {
		projectID = sp.CurrentProjectID
	}
	if projectID == "" && requireProject {
		return nil, fmt.Errorf("%w: profile %q has no current project", errSessionNoProject, name)
	}

	return &session{
		ProfileName:  name,
		URL:          cp.URL,
		Username:     cp.Username,
		JWT:          sp.JWT,
		RefreshToken: sp.RefreshToken,
		Kind:         sp.CredentialKind,
		ProjectID:    projectID,
	}, nil
}

// errNoRefresh wraps client.ErrUnauthorized so the transport maps it back to
// the token_expired envelope: for password/PAT credentials a 401 genuinely
// means "log in again", unlike a failed device-flow refresh.
var errNoRefresh = fmt.Errorf("session: no refresh token for profile: %w", client.ErrUnauthorized)

// Client builds an API client for this session that transparently refreshes
// the access token on a 401 (device logins) and persists the rotated tokens.
func (s *session) Client(opts ...client.Option) *client.Client {
	return newRefreshingClient(s.URL, s.ProfileName, s.JWT, s.RefreshToken, s.Kind, opts...)
}

// newRefreshingClient returns a client whose refresher exchanges the stored
// refresh token for a new access token and persists the rotation. Non-device
// credentials (password, PAT) have no refresh token, so the refresher reports
// errNoRefresh and the original 401 surfaces as token_expired.
//
// The refresher is lock-free but concurrency-safe: because refresh tokens are
// single-use and the server revokes a family on genuine replay, two CLI
// processes racing to refresh the same token must not both call the server. So
// before (and after a lost race) it re-reads the on-disk token; if another
// process already rotated it, we adopt that token instead of minting a new one.
// A failed persist is non-fatal for the current command, and a retry within the
// server's reuse-grace window is answered with the same rotated token set; a
// later command re-presenting the stale token is treated as a replay, revoking
// the family, so the warning below tells the user a re-login may be needed.
func newRefreshingClient(url, profileName, jwt, refreshToken, kind string, opts ...client.Option) *client.Client {
	rt := refreshToken
	current := jwt
	refresher := func(ctx context.Context) (string, error) {
		if kind != state.KindDevice || rt == "" {
			return "", errNoRefresh
		}

		// Another process may have already refreshed; prefer its token.
		if disk := loadProfileJWT(profileName); disk != "" && disk != current {
			current = disk
			return disk, nil
		}

		ts, err := client.New(url).Refresh(ctx, rt)
		if err != nil {
			// Lost a refresh race: the winner may have just persisted a token.
			if errors.Is(err, client.ErrInvalidGrant) {
				if disk := loadProfileJWT(profileName); disk != "" && disk != current {
					current = disk
					return disk, nil
				}
			}
			return "", err
		}

		if perr := updateProfileTokens(profileName, ts); perr != nil {
			fmt.Fprintf(os.Stderr, "warning: refreshed access token but could not save it: %v (later commands may require 'traceway login' again)\n", perr)
		}
		if ts.RefreshToken != "" {
			rt = ts.RefreshToken
		}
		current = ts.AccessToken
		return ts.AccessToken, nil
	}
	base := []client.Option{client.WithJWT(jwt), client.WithRefresher(refresher)}
	return client.New(url, append(base, opts...)...)
}

// renderSessionError maps loadSession sentinel errors to envelopes.
func renderSessionError(errOut io.Writer, mode output.Mode, err error) error {
	switch {
	case errors.Is(err, errSessionNoProfile), errors.Is(err, errSessionNoJWT):
		_ = output.RenderError(errOut, mode, output.ErrorEnvelope{
			Code:     "not_authenticated",
			Message:  err.Error(),
			Hint:     "traceway login",
			ExitCode: exitcode.Auth,
		})
		return newCLIError(exitcode.Auth, "not_authenticated")
	case errors.Is(err, errSessionNoProject):
		_ = output.RenderError(errOut, mode, output.ErrorEnvelope{
			Code:     "no_project",
			Message:  err.Error(),
			Hint:     "traceway projects use <project-id> (or pass --project)",
			ExitCode: exitcode.Usage,
		})
		return newCLIError(exitcode.Usage, "no_project")
	}
	_ = output.RenderError(errOut, mode, output.ErrorEnvelope{
		Code: "internal", Message: err.Error(), ExitCode: exitcode.Generic,
	})
	return newCLIError(exitcode.Generic, "internal")
}
