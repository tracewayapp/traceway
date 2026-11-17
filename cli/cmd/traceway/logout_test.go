package main

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/internal/config"
	"github.com/tracewayapp/traceway/cli/v2/internal/state"
)

func TestLogout_removesProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"default": {URL: "https://x", Username: "u"},
			"cloud":   {URL: "https://y", Username: "v"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	st := &state.State{
		CurrentProfile: "default",
		Profiles: map[string]state.ProfileState{
			"default": {JWT: "tok"},
			"cloud":   {JWT: "tok2"},
		},
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runCmd(t, "", "logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	gotCfg, _ := config.Load()
	if _, ok := gotCfg.Profiles["default"]; ok {
		t.Error("default profile should be removed from config")
	}
	if _, ok := gotCfg.Profiles["cloud"]; !ok {
		t.Error("cloud profile should be untouched in config")
	}
	gotState, _ := state.Load()
	if _, ok := gotState.Profiles["default"]; ok {
		t.Error("default profile should be removed from state")
	}
}

func TestLogout_resetsCurrentProfileWhenRemovingIt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"default": {URL: "https://x"},
			"cloud":   {URL: "https://y"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	st := &state.State{
		CurrentProfile: "default",
		Profiles: map[string]state.ProfileState{
			"default": {JWT: "tok"},
			"cloud":   {JWT: "tok2"},
		},
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCmd(t, "", "logout")
	if err != nil {
		t.Fatal(err)
	}
	gotState, _ := state.Load()
	if gotState.CurrentProfile == "default" {
		t.Errorf("CurrentProfile should not still be 'default', got %q", gotState.CurrentProfile)
	}
}

func TestLogout_unknownProfile_returnsAuthError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	_, stderr, err := runCmd(t, "", "logout", "--profile", "ghost", "--output", "json")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(stderr.String(), `"error"`) {
		t.Errorf("expected JSON envelope, got: %s", stderr.String())
	}
}
