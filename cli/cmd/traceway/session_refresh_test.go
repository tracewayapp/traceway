package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/state"
)

func TestSession_refreshesExpiredDeviceToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/token":
			_, _ = w.Write([]byte(`{"access_token":"fresh","refresh_token":"rt2","expires_in":900,"email":"d@e.com"}`))
		case "/api/projects":
			if r.Header.Get("Authorization") == "Bearer stale" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Header.Get("Authorization") != "Bearer fresh" {
				t.Errorf("retry auth = %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Profiles: map[string]config.Profile{
		"default": {URL: srv.URL, Username: "d@e.com"},
	}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	st := &state.State{CurrentProfile: "default", Profiles: map[string]state.ProfileState{
		"default": {JWT: "stale", RefreshToken: "rt", CredentialKind: state.KindDevice, CurrentProjectID: "proj-1"},
	}}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runCmd(t, "", "projects", "list"); err != nil {
		t.Fatalf("projects list: %v", err)
	}

	got, _ := state.Load()
	sp := got.Profiles["default"]
	if sp.JWT != "fresh" {
		t.Errorf("access token not refreshed in state: %q", sp.JWT)
	}
	if sp.RefreshToken != "rt2" {
		t.Errorf("refresh token not rotated in state: %q", sp.RefreshToken)
	}
}
