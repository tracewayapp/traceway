package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/state"
)

func TestLogin_passwordStdin_success(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/login" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["email"] != "fred@example.com" {
			t.Errorf("email = %q", req["email"])
		}
		if req["password"] != "hunter2" {
			t.Errorf("password = %q", req["password"])
		}
		_, _ = w.Write([]byte(`{"token":"jwt.value"}`))
	}))
	defer srv.Close()

	_, _, err := runCmd(t, "hunter2\n",
		"login",
		"--url", srv.URL,
		"--username", "fred@example.com",
		"--password-stdin",
	)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	p, ok := cfg.Profiles["default"]
	if !ok {
		t.Fatal("default profile not saved in config")
	}
	if p.URL != srv.URL {
		t.Errorf("URL = %q", p.URL)
	}
	if p.Username != "fred@example.com" {
		t.Errorf("Username = %q", p.Username)
	}

	st, err := state.Load()
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	sp, ok := st.Profiles["default"]
	if !ok {
		t.Fatal("default profile not saved in state")
	}
	if sp.JWT != "jwt.value" {
		t.Errorf("JWT = %q", sp.JWT)
	}
	if st.CurrentProfile != "default" {
		t.Errorf("CurrentProfile = %q, want default (only profile should be auto-set)", st.CurrentProfile)
	}
}

func TestLogin_passwordWithRefreshToken_isRefreshableAndRevocable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"access","refreshToken":"twr_refresh","expiresIn":900}`))
	}))
	defer srv.Close()

	if _, stderr, err := runCmd(t, "hunter2\n", "login", "--url", srv.URL, "--username", "fred@example.com", "--password-stdin"); err != nil {
		t.Fatalf("login failed: %v %s", err, stderr.String())
	}
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	sp := st.Profiles["default"]
	if sp.JWT != "access" || sp.RefreshToken != "twr_refresh" || sp.CredentialKind != state.KindDevice || sp.TokenExpiresAt == 0 {
		t.Errorf("a scoped password login must be stored as a refreshable session, got %+v", sp)
	}
}

func TestLogin_namedProfile_secondLogin_doesNotOverrideCurrent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"token":"tok"}`))
	}))
	defer srv.Close()

	// First login → default profile.
	if _, _, err := runCmd(t, "p\n",
		"login", "--url", srv.URL, "--username", "a@example.com", "--password-stdin",
	); err != nil {
		t.Fatalf("first login: %v", err)
	}
	// Second login → cloud profile. Should NOT change current profile pointer.
	if _, _, err := runCmd(t, "p\n",
		"login", "--profile", "cloud", "--url", srv.URL, "--username", "b@example.com", "--password-stdin",
	); err != nil {
		t.Fatalf("second login: %v", err)
	}

	st, _ := state.Load()
	if st.CurrentProfile != "default" {
		t.Errorf("CurrentProfile = %q, want 'default' (was set on first login, second login must not override)", st.CurrentProfile)
	}
	cfg, _ := config.Load()
	if _, ok := cfg.Profiles["cloud"]; !ok {
		t.Error("cloud profile missing from config")
	}
}

func TestLogin_invalidCredentials_writesEnvelope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, stderr, err := runCmd(t, "wrong\n",
		"login", "--output", "json", "--url", srv.URL, "--username", "x", "--password-stdin",
	)
	if err == nil {
		t.Fatal("expected login to return an error")
	}
	if !strings.Contains(stderr.String(), `"error"`) {
		t.Errorf("expected JSON error envelope on stderr, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), `"not_authenticated"`) {
		t.Errorf("expected error code 'not_authenticated', got: %s", stderr.String())
	}
}

func TestLogin_refreshExistingProfile_keepsURLAndUsername(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_STATE_HOME", stateDir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"token":"newtoken"}`))
	}))
	defer srv.Close()

	// Seed config and state with an existing profile.
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"default": {URL: srv.URL, Username: "fred@example.com"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	st := &state.State{
		CurrentProfile: "default",
		Profiles: map[string]state.ProfileState{
			"default": {JWT: "old"},
		},
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	// Refresh: only --password-stdin, no --url/--username.
	if _, _, err := runCmd(t, "newpw\n", "login", "--password-stdin"); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	gotCfg, _ := config.Load()
	p := gotCfg.Profiles["default"]
	if p.URL != srv.URL {
		t.Errorf("URL changed: %q", p.URL)
	}
	if p.Username != "fred@example.com" {
		t.Errorf("Username changed: %q", p.Username)
	}

	gotState, _ := state.Load()
	if gotState.Profiles["default"].JWT != "newtoken" {
		t.Errorf("JWT not refreshed: %q", gotState.Profiles["default"].JWT)
	}
}
