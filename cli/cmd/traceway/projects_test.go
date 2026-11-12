package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/internal/config"
	"github.com/tracewayapp/traceway/cli/v2/internal/state"
)

func seedProfileFor(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"default": {URL: baseURL, Username: "fred@example.com"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	st := &state.State{
		CurrentProfile: "default",
		Profiles: map[string]state.ProfileState{
			"default": {JWT: "tok"},
		},
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectsList_jsonShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q", r.Method)
		}
		_, _ = w.Write([]byte(`[
			{"id":"p1","name":"alpha"},
			{"id":"p2","name":"beta"}
		]`))
	}))
	defer srv.Close()
	seedProfileFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "projects", "list", "--output", "json")
	if err != nil {
		t.Fatalf("projects list: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"alpha"`) || !strings.Contains(out, `"beta"`) {
		t.Errorf("expected both project names, got: %s", out)
	}
}

func TestProjectsList_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"p1","name":"alpha"}]`))
	}))
	defer srv.Close()
	seedProfileFor(t, srv.URL)

	stdout, _, err := runCmd(t, "", "projects", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "p1") || !strings.Contains(out, "alpha") {
		t.Errorf("expected id/name in table, got: %s", out)
	}
}

func TestProjectsList_unauth_writesEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	seedProfileFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "projects", "list", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"token_expired"`) {
		t.Errorf("expected token_expired envelope, got: %s", stderr.String())
	}
}

func TestProjectsList_noProfile_writesEnvelope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	_, stderr, err := runCmd(t, "", "projects", "list", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"not_authenticated"`) {
		t.Errorf("expected not_authenticated envelope, got: %s", stderr.String())
	}
}

func TestProjectsUse_persistsToProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	seedProfileFor(t, srv.URL)

	if _, _, err := runCmd(t, "", "projects", "use", "p1"); err != nil {
		t.Fatal(err)
	}
	st, _ := state.Load()
	if st.Profiles["default"].CurrentProjectID != "p1" {
		t.Errorf("CurrentProjectID = %q", st.Profiles["default"].CurrentProjectID)
	}
}
