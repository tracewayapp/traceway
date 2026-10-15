package main

import (
	"time"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/state"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// credential is the result of any login path: the access token plus, for the
// device flow, a refresh token and access-token expiry.
type credential struct {
	Kind         string
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

// saveProfileCredentials persists profile config (url/username) and runtime
// state (tokens/kind), preserving the current project and setting the
// CurrentProfile pointer on first login. Shared by all login paths.
func saveProfileCredentials(cfg *config.Config, st *state.State, profileName, url, username string, cred credential) error {
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]config.Profile{}
	}
	cfg.Profiles[profileName] = config.Profile{URL: url, Username: username}

	if st.Profiles == nil {
		st.Profiles = map[string]state.ProfileState{}
	}
	currentProject := st.Profiles[profileName].CurrentProjectID
	st.Profiles[profileName] = state.ProfileState{
		JWT:              cred.AccessToken,
		RefreshToken:     cred.RefreshToken,
		TokenExpiresAt:   cred.ExpiresAt,
		CredentialKind:   cred.Kind,
		CurrentProjectID: currentProject,
	}
	if st.CurrentProfile == "" {
		st.CurrentProfile = profileName
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	return st.Save()
}

// updateProfileTokens reloads state and updates only the token fields for the
// profile (preserving project + config), then atomically saves. Used by the
// refresher so a silent refresh doesn't clobber a concurrent project change.
func updateProfileTokens(profileName string, ts *client.TokenSet) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	sp := st.Profiles[profileName]
	sp.JWT = ts.AccessToken
	if ts.RefreshToken != "" {
		sp.RefreshToken = ts.RefreshToken
	}
	sp.TokenExpiresAt = expiresAtUnix(ts.ExpiresIn)
	if sp.CredentialKind == "" {
		sp.CredentialKind = state.KindDevice
	}
	if st.Profiles == nil {
		st.Profiles = map[string]state.ProfileState{}
	}
	st.Profiles[profileName] = sp
	return st.Save()
}

// loadProfileTokens reads the stored access and refresh tokens for a profile
// from disk, returning empty strings if state can't be read. The refresher uses
// them to adopt a rotation another concurrent process already made, avoiding a
// redundant refresh that would trip server-side reuse detection.
func loadProfileTokens(profileName string) (jwt, refreshToken string) {
	st, err := state.Load()
	if err != nil {
		return "", ""
	}
	sp := st.Profiles[profileName]
	return sp.JWT, sp.RefreshToken
}

func expiresAtUnix(expiresInSeconds int) int64 {
	if expiresInSeconds <= 0 {
		return 0
	}
	return time.Now().Add(time.Duration(expiresInSeconds) * time.Second).Unix()
}
