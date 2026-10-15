package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// A long-running process (traceway mcp) that adopts another process's rotated
// access token must adopt its refresh token too. Otherwise, once that access
// token expires, it presents its own already-spent refresh token, which the
// server treats as a replay and answers by revoking the whole login.
func TestSession_adoptedRotationCarriesTheRefreshToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var mu sync.Mutex
	valid := map[string]bool{"jwt2": true}
	var presented []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api/auth/token":
			var body struct {
				RefreshToken string `json:"refresh_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			presented = append(presented, body.RefreshToken)
			if body.RefreshToken != "rt2" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			valid["jwt3"] = true
			_, _ = w.Write([]byte(`{"access_token":"jwt3","refresh_token":"rt3","expires_in":900}`))
		case "/api/projects":
			if !valid[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	// This process started with jwt1/rt1; another process has since rotated
	// them to jwt2/rt2 and saved those.
	st := &state.State{CurrentProfile: "default", Profiles: map[string]state.ProfileState{
		"default": {JWT: "jwt2", RefreshToken: "rt2", CredentialKind: state.KindDevice},
	}}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	c := newRefreshingClient(srv.URL, "default", "jwt1", "rt1", state.KindDevice)

	if _, err := c.ListProjects(context.Background()); err != nil {
		t.Fatalf("first call (adopts jwt2): %v", err)
	}
	mu.Lock()
	if len(presented) != 0 {
		t.Fatalf("adopting a saved rotation must not refresh, presented %v", presented)
	}
	delete(valid, "jwt2") // the adopted access token expires
	mu.Unlock()

	if _, err := c.ListProjects(context.Background()); err != nil {
		t.Fatalf("second call (refreshes): %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(presented) != 1 || presented[0] != "rt2" {
		t.Fatalf("refresh must present the adopted rt2, presented %v", presented)
	}
	if got, _ := state.Load(); got.Profiles["default"].RefreshToken != "rt3" {
		t.Errorf("rotated refresh token not saved: %q", got.Profiles["default"].RefreshToken)
	}
}
