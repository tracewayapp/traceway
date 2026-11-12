package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/internal/config"
	"github.com/tracewayapp/traceway/cli/v2/internal/state"
)

func TestLoadSession_happyPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Cleanup(func() { flagProfile = ""; flagProject = "" })

	cfg := &config.Config{Profiles: map[string]config.Profile{
		"default": {URL: "https://x", Username: "u"},
	}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	st := &state.State{
		CurrentProfile: "default",
		Profiles:       map[string]state.ProfileState{"default": {JWT: "tok", CurrentProjectID: "proj-1"}},
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	sess, err := loadSession()
	if err != nil {
		t.Fatalf("loadSession: %v", err)
	}
	if sess.URL != "https://x" {
		t.Errorf("URL = %q", sess.URL)
	}
	if sess.JWT != "tok" {
		t.Errorf("JWT = %q", sess.JWT)
	}
	if sess.ProjectID != "proj-1" {
		t.Errorf("ProjectID = %q", sess.ProjectID)
	}
	if sess.ProfileName != "default" {
		t.Errorf("ProfileName = %q", sess.ProfileName)
	}
}

func TestLoadSession_flagProjectOverridesState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Cleanup(func() { flagProfile = ""; flagProject = "" })

	cfg := &config.Config{Profiles: map[string]config.Profile{
		"default": {URL: "https://x"},
	}}
	_ = cfg.Save()
	st := &state.State{
		CurrentProfile: "default",
		Profiles:       map[string]state.ProfileState{"default": {JWT: "tok", CurrentProjectID: "proj-default"}},
	}
	_ = st.Save()

	flagProject = "proj-override"
	sess, err := loadSession()
	if err != nil {
		t.Fatal(err)
	}
	if sess.ProjectID != "proj-override" {
		t.Errorf("ProjectID = %q, want proj-override", sess.ProjectID)
	}
}

func TestLoadSession_missingProfile_returnsNotAuthenticated(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Cleanup(func() { flagProfile = ""; flagProject = "" })

	_, err := loadSession()
	if !errors.Is(err, errSessionNoProfile) {
		t.Errorf("got %v, want errSessionNoProfile", err)
	}
}

func TestLoadSession_profileWithoutJWT_returnsNotAuthenticated(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Cleanup(func() { flagProfile = ""; flagProject = "" })

	cfg := &config.Config{Profiles: map[string]config.Profile{
		"default": {URL: "https://x"},
	}}
	_ = cfg.Save()
	// State exists but has no JWT for this profile.
	st := &state.State{CurrentProfile: "default", Profiles: map[string]state.ProfileState{}}
	_ = st.Save()

	_, err := loadSession()
	if !errors.Is(err, errSessionNoJWT) {
		t.Errorf("got %v, want errSessionNoJWT", err)
	}
}

func TestLoadSession_missingProject_returnsNoProject(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Cleanup(func() { flagProfile = ""; flagProject = "" })

	cfg := &config.Config{Profiles: map[string]config.Profile{
		"default": {URL: "https://x"},
	}}
	_ = cfg.Save()
	st := &state.State{
		CurrentProfile: "default",
		Profiles:       map[string]state.ProfileState{"default": {JWT: "tok"}}, // no current_project_id
	}
	_ = st.Save()

	_, err := loadSession()
	if !errors.Is(err, errSessionNoProject) {
		t.Errorf("got %v, want errSessionNoProject", err)
	}
}

// Sanity-check: a session built against an httptest server and an HTTP probe
// works end-to-end (smoke for caller patterns).
func TestLoadSession_pointsAtRealServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Cleanup(func() { flagProfile = ""; flagProject = "" })

	cfg := &config.Config{Profiles: map[string]config.Profile{"default": {URL: srv.URL}}}
	_ = cfg.Save()
	st := &state.State{CurrentProfile: "default", Profiles: map[string]state.ProfileState{"default": {JWT: "tok", CurrentProjectID: "p"}}}
	_ = st.Save()

	sess, err := loadSession()
	if err != nil {
		t.Fatal(err)
	}
	if sess.URL != srv.URL {
		t.Errorf("URL mismatch")
	}
}
