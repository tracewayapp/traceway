package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tracewayapp/traceway/cli/internal/state"
)

// Config is the on-disk declarative configuration file. It contains only
// stable fields suitable for dotfile management (url, username per profile).
// Runtime fields (JWT, current project, active profile pointer) live in State.
type Config struct {
	Profiles map[string]Profile `json:"profiles"`
}

// Profile holds connection parameters for a single Traceway instance.
type Profile struct {
	URL      string `json:"url"`
	Username string `json:"username"`
}

// legacyProfile is used only for migration detection.
type legacyProfile struct {
	URL              string `json:"url"`
	Username         string `json:"username"`
	JWT              string `json:"jwt"`
	CurrentProjectID string `json:"current_project_id,omitempty"`
}

// legacyConfig is used only for migration detection.
type legacyConfig struct {
	CurrentProfile string                   `json:"current_profile"`
	Profiles       map[string]legacyProfile `json:"profiles"`
}

// Load reads the config file from disk. A missing file yields an empty Config
// (not an error) — the caller treats absence of credentials as an auth error
// only when an actual command needs them.
//
// If the on-disk file is in the old shape (has current_profile at top level or
// any profile has jwt / current_project_id), Load migrates those fields into
// the state file automatically and rewrites config.json without them.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	// Parse into the legacy shape first to detect migration need.
	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if legacy.Profiles == nil {
		legacy.Profiles = map[string]legacyProfile{}
	}

	needsMigration := legacy.CurrentProfile != ""
	if !needsMigration {
		for _, lp := range legacy.Profiles {
			if lp.JWT != "" || lp.CurrentProjectID != "" {
				needsMigration = true
				break
			}
		}
	}

	if needsMigration {
		// Build a State from the legacy fields.
		st := &state.State{
			CurrentProfile: legacy.CurrentProfile,
			Profiles:       map[string]state.ProfileState{},
		}
		for name, lp := range legacy.Profiles {
			if lp.JWT != "" || lp.CurrentProjectID != "" {
				st.Profiles[name] = state.ProfileState{
					JWT:              lp.JWT,
					CurrentProjectID: lp.CurrentProjectID,
				}
			}
		}
		if err := st.Save(); err != nil {
			return nil, fmt.Errorf("migrating state: %w", err)
		}

		// Build a clean Config with only stable fields.
		cfg := &Config{Profiles: map[string]Profile{}}
		for name, lp := range legacy.Profiles {
			cfg.Profiles[name] = Profile{URL: lp.URL, Username: lp.Username}
		}
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("saving migrated config: %w", err)
		}
		return cfg, nil
	}

	// No migration needed — parse normally into the clean shape.
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return &cfg, nil
}

// Save atomically writes the config to disk. Creates parent dirs (0700) and the
// file (0600). Atomicity is achieved by writing to a tempfile in the same
// directory and renaming over the destination.
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".config.json.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
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
