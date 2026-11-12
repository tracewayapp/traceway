package main

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/internal/config"
	"github.com/tracewayapp/traceway/cli/v2/internal/state"
)

func seedTwoProfiles(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"default": {URL: "https://a", Username: "fred@a"},
			"cloud":   {URL: "https://b", Username: "fred@b"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	st := &state.State{
		CurrentProfile: "default",
		Profiles: map[string]state.ProfileState{
			"default": {JWT: "tok1"},
			"cloud":   {JWT: "tok2"},
		},
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestProfilesList_table(t *testing.T) {
	seedTwoProfiles(t)
	stdout, _, err := runCmd(t, "", "profiles", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "default") {
		t.Errorf("missing 'default' in output: %s", out)
	}
	if !strings.Contains(out, "cloud") {
		t.Errorf("missing 'cloud' in output: %s", out)
	}
	// Current profile marked somehow (we use a "*" prefix).
	if !strings.Contains(out, "*") {
		t.Errorf("expected current-profile marker '*': %s", out)
	}
}

func TestProfilesList_json(t *testing.T) {
	seedTwoProfiles(t)
	stdout, _, err := runCmd(t, "", "profiles", "list", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"default"`) || !strings.Contains(out, `"cloud"`) {
		t.Errorf("expected both profiles in JSON, got: %s", out)
	}
	if !strings.Contains(out, `"current"`) {
		t.Errorf("expected 'current' field in JSON, got: %s", out)
	}
}

func TestProfilesUse_setsCurrent(t *testing.T) {
	seedTwoProfiles(t)
	if _, _, err := runCmd(t, "", "profiles", "use", "cloud"); err != nil {
		t.Fatal(err)
	}
	st, _ := state.Load()
	if st.CurrentProfile != "cloud" {
		t.Errorf("CurrentProfile = %q, want cloud", st.CurrentProfile)
	}
}

func TestProfilesUse_unknown_returnsAuthError(t *testing.T) {
	seedTwoProfiles(t)
	_, stderr, err := runCmd(t, "", "profiles", "use", "ghost", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"no_profile"`) {
		t.Errorf("expected 'no_profile' code in stderr, got: %s", stderr.String())
	}
}
