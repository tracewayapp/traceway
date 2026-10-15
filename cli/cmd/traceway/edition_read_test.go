//go:build !write

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadEdition_hasNoWriteCommands(t *testing.T) {
	seedSessionFor(t, "http://127.0.0.1:1")
	for _, args := range [][]string{
		{"exceptions", "archive", "--yes", "abc"},
		{"pages", "ack", "1"},
		{"alerts", "snooze", "1"},
		{"monitors", "run", "1"},
		{"post-mortems", "create", "--title", "x"},
		{"endpoints", "mark-slow", "GET /"},
	} {
		_, stderr, err := runCmd(t, "", append(args, "--output", "json")...)
		if err == nil || !strings.Contains(stderr.String(), "write_required") || !strings.Contains(stderr.String(), "traceway-write") {
			t.Errorf("%v in the read-only build: err=%v stderr=%s", args, err, stderr.String())
		}
	}
}

func TestReadEdition_passwordLoginAsksForReadScope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	}))
	defer srv.Close()

	if _, stderr, err := runCmd(t, "pw\n", "login", "--url", srv.URL, "--username", "a@example.com", "--password-stdin"); err != nil {
		t.Fatalf("login: %v %s", err, stderr.String())
	}
	if body["scope"] != "traceway:read" {
		t.Errorf("password login must ask for a read-scoped token, body = %v", body)
	}
	if !strings.HasPrefix(newRootCmd().Use, "traceway") || newRootCmd().Use != "traceway" {
		t.Errorf("read build binary name = %q", newRootCmd().Use)
	}
}

func TestReadEdition_refusesWriteTokens(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)

	_, stderr, err := runCmd(t, "", "login", "--url", srv.URL, "--token", "tww_abc", "--output", "json")
	if err == nil || !strings.Contains(stderr.String(), "write token") || called {
		t.Errorf("login --token with a tww_ token must be refused before any request, got err=%v called=%v stderr=%s", err, called, stderr.String())
	}

	t.Setenv("TRACEWAY_URL", srv.URL)
	t.Setenv("TRACEWAY_TOKEN", "tww_abc")
	if _, err := resolveMcpConfig(&strings.Builder{}); err == nil || !strings.Contains(err.Error(), "write token") {
		t.Errorf("mcp with a tww_ TRACEWAY_TOKEN must be refused, got %v", err)
	}
}
