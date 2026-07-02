package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/state"
)

func TestLoginToken_success(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer twp_secret" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	if _, _, err := runCmd(t, "", "login", "--url", srv.URL, "--token", "twp_secret"); err != nil {
		t.Fatalf("login token: %v", err)
	}

	st, _ := state.Load()
	sp := st.Profiles["default"]
	if sp.CredentialKind != state.KindPAT {
		t.Errorf("kind = %q, want pat", sp.CredentialKind)
	}
	if sp.JWT != "twp_secret" {
		t.Errorf("jwt = %q", sp.JWT)
	}
	if sp.RefreshToken != "" {
		t.Errorf("unexpected refresh token %q", sp.RefreshToken)
	}
}

func TestLoginToken_stdin_success(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer twp_fromstdin" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	if _, _, err := runCmd(t, "twp_fromstdin\n", "login", "--url", srv.URL, "--token-stdin"); err != nil {
		t.Fatalf("login token-stdin: %v", err)
	}

	st, _ := state.Load()
	if st.Profiles["default"].JWT != "twp_fromstdin" {
		t.Errorf("jwt = %q", st.Profiles["default"].JWT)
	}
}

func TestLoginToken_rejected_doesNotSave(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, _, err := runCmd(t, "", "login", "--url", srv.URL, "--token", "twp_bad")
	if err == nil {
		t.Fatal("expected error for rejected token")
	}
	st, _ := state.Load()
	if _, ok := st.Profiles["default"]; ok {
		t.Error("profile should not be saved when token is rejected")
	}
}
