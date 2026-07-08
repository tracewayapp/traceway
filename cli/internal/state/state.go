// Package state manages runtime data that changes frequently and must not be
// managed declaratively: JWT tokens, current project selection, and the active
// profile pointer. It mirrors the XDG Base Directory Specification by storing
// data under XDG_STATE_HOME (default: $HOME/.local/state).
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// State is the on-disk runtime state file.
type State struct {
	CurrentProfile string                  `json:"current_profile"`
	Profiles       map[string]ProfileState `json:"profiles"`
}

// Credential kinds stored on ProfileState.CredentialKind. An empty value is
// treated as a legacy password login (non-refreshable).
const (
	KindPassword = "password"
	KindDevice   = "device"
	KindPAT      = "pat"
)

// ProfileState holds runtime state for a single Traceway profile.
type ProfileState struct {
	JWT              string `json:"jwt"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	TokenExpiresAt   int64  `json:"token_expires_at,omitempty"`
	CredentialKind   string `json:"credential_kind,omitempty"`
	CurrentProjectID string `json:"current_project_id,omitempty"`
}

// Load reads the state file from disk. A missing file yields an empty State
// (not an error) — the caller treats absence of credentials as an auth error
// only when an actual command needs them.
func Load() (*State, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{Profiles: map[string]ProfileState{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing state %s: %w", path, err)
	}
	if st.Profiles == nil {
		st.Profiles = map[string]ProfileState{}
	}
	return &st, nil
}

// Save atomically writes the state to disk. Creates parent dirs (0700) and the
// file (0600). Atomicity is achieved by writing to a tempfile in the same
// directory and renaming over the destination.
func (s *State) Save() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".state.json.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming into place: %w", err)
	}
	return nil
}

// Active resolves the effective profile name by precedence:
//
//	explicit name > s.CurrentProfile > "default"
//
// It returns the resolved name only. Callers should index into s.Profiles
// themselves; the profile may not exist in state yet (e.g., before first login).
func (s *State) Active(name string) string {
	if name != "" {
		return name
	}
	if s.CurrentProfile != "" {
		return s.CurrentProfile
	}
	return "default"
}
