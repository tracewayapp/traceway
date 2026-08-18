package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testSetupToken = "tws_test-setup-token"

// newSetupTestServer fakes the three setup endpoints: the plan is approved
// after approveAfterPolls status polls.
func newSetupTestServer(t *testing.T, approveAfterPolls int32) *httptest.Server {
	t.Helper()
	var polls int32
	var submitted atomic.Bool

	backendToken := "tok_backend-secret"
	webToken := "tok_web-secret"

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+testSetupToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/setup/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"organizationId": 1, "organizationName": "Acme", "email": "a@b.c",
				"backendUrl": "https://traceway.example.com", "projects": []any{},
			})
		case "PUT /api/setup/plan":
			submitted.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
		case "GET /api/setup/plan":
			if !submitted.Load() {
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "none"})
				return
			}
			if atomic.AddInt32(&polls, 1) <= approveAfterPolls {
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "approved",
				"projects": []map[string]any{
					{"id": "p-backend", "name": "Product Backend", "framework": "opentelemetry",
						"token": backendToken, "backendUrl": "https://traceway.example.com", "status": "created"},
					{"id": "p-web", "name": "Product Web", "framework": "svelte",
						"token": webToken, "backendUrl": "https://traceway.example.com", "status": "created"},
				},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func writeSetupPlanFile(t *testing.T, dir, url string) string {
	t.Helper()
	plan := map[string]any{
		"url": url,
		"projects": []map[string]any{
			{"name": "Product Backend", "framework": "opentelemetry",
				"envFile": "backend/.env", "envVar": "TRACEWAY_BACKEND_TOKEN"},
			{"name": "Product Web", "framework": "svelte",
				"envFile": "web/.env", "envVar": "PUBLIC_TRACEWAY_CONNECTION_STRING", "envFormat": "connectionString",
				"deployment": map[string]any{"platform": "Vercel", "instructions": "vercel env add PUBLIC_TRACEWAY_CONNECTION_STRING <token>"}},
		},
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	for _, sub := range []string{"backend", "web"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	return path
}

func setupTestEnvIsolation(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TRACEWAY_SETUP_TOKEN", "")

	prevInterval := setupPollInterval
	setupPollInterval = 5 * time.Millisecond
	t.Cleanup(func() { setupPollInterval = prevInterval })
}

func TestSetupApplyHappyPath(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServer(t, 2)
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL)

	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--output", "table")
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}

	combined := stdout.String() + stderr.String()
	if strings.Contains(combined, "tok_backend-secret") || strings.Contains(combined, "tok_web-secret") {
		t.Fatal("token values leaked into output")
	}
	if !strings.Contains(stdout.String(), "Product Backend") || !strings.Contains(stdout.String(), "created") {
		t.Errorf("summary missing rows: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Next steps") || !strings.Contains(stdout.String(), "Vercel") {
		t.Errorf("summary missing next steps: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "review and approve") {
		t.Errorf("stderr missing approval prompt: %s", stderr.String())
	}

	backendEnv, err := os.ReadFile(filepath.Join(dir, "backend", ".env"))
	if err != nil {
		t.Fatalf("read backend env: %v", err)
	}
	if string(backendEnv) != "TRACEWAY_BACKEND_TOKEN=tok_backend-secret\n" {
		t.Errorf("backend env = %q", backendEnv)
	}
	info, err := os.Stat(filepath.Join(dir, "backend", ".env"))
	if err != nil {
		t.Fatalf("stat backend env: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("backend env mode = %v, want 0600", info.Mode().Perm())
	}

	webEnv, err := os.ReadFile(filepath.Join(dir, "web", ".env"))
	if err != nil {
		t.Fatalf("read web env: %v", err)
	}
	want := "PUBLIC_TRACEWAY_CONNECTION_STRING=tok_web-secret@https://traceway.example.com/api/report\n"
	if string(webEnv) != want {
		t.Errorf("web env = %q, want %q", webEnv, want)
	}

	// The ephemeral setup token must leave no trace in CLI state.
	stateDir := os.Getenv("XDG_STATE_HOME")
	if entries, err := os.ReadDir(stateDir); err == nil && len(entries) != 0 {
		t.Errorf("state dir should stay empty, has %v", entries)
	}
}

func TestSetupApplyTokenFromEnv(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServer(t, 0)
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL)
	t.Setenv("TRACEWAY_SETUP_TOKEN", testSetupToken)

	_, stderr, err := runCmd(t, "", "setup", "apply", "--plan", planPath)
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
}

func TestSetupApplyURLFlagOverridesPlan(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServer(t, 0)
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, "https://wrong.example.invalid")

	_, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--url", server.URL)
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
}

func TestSetupApplyRejection(t *testing.T) {
	setupTestEnvIsolation(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/setup/session":
			_ = json.NewEncoder(w).Encode(map[string]any{"organizationId": 1, "organizationName": "Acme", "email": "a@b.c", "backendUrl": "x", "projects": []any{}})
		case "PUT /api/setup/plan":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
		case "GET /api/setup/plan":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "rejected", "reason": "wrong project split"})
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL)

	_, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin")
	if err == nil {
		t.Fatal("expected rejection error")
	}
	var cliErr *cliError
	if !asCLIError(err, &cliErr) || cliErr.code != 1 {
		t.Fatalf("error = %v, want cliError with exit 1", err)
	}
	if !strings.Contains(stderr.String(), "wrong project split") {
		t.Errorf("stderr should carry the rejection reason: %s", stderr.String())
	}
}

func TestSetupApplyInvalidToken(t *testing.T) {
	setupTestEnvIsolation(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL)

	_, stderr, err := runCmd(t, "tws_expired\n", "setup", "apply", "--plan", planPath, "--token-stdin")
	if err == nil {
		t.Fatal("expected auth error")
	}
	var cliErr *cliError
	if !asCLIError(err, &cliErr) || cliErr.code != 4 {
		t.Fatalf("error = %v, want cliError with exit 4", err)
	}
	if !strings.Contains(stderr.String(), "invalid_setup_token") && !strings.Contains(stderr.String(), "invalid or expired") {
		t.Errorf("stderr = %s", stderr.String())
	}
}

func TestSetupApplyApprovalTimeout(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServer(t, 1<<30)
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL)

	_, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--wait-timeout", "30ms")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(stderr.String(), "setup_approval_timeout") && !strings.Contains(stderr.String(), "timed out") {
		t.Errorf("stderr = %s", stderr.String())
	}
}

func TestSetupApplyUsageErrors(t *testing.T) {
	setupTestEnvIsolation(t)
	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, "https://x.example")

	badPlanPath := filepath.Join(dir, "bad-plan.json")
	if err := os.WriteFile(badPlanPath, []byte(`{"projects":[{"name":"A","framework":"react","envFile":"x/.env"}]}`), 0o644); err != nil {
		t.Fatalf("write bad plan: %v", err)
	}

	cases := map[string][]string{
		"no plan":                  {"setup", "apply", "--token", testSetupToken},
		"no token":                 {"setup", "apply", "--plan", planPath},
		"both token flags":         {"setup", "apply", "--plan", planPath, "--token", "x", "--token-stdin"},
		"envFile without envVar":   {"setup", "apply", "--plan", badPlanPath, "--token", testSetupToken},
		"missing plan file":        {"setup", "apply", "--plan", filepath.Join(dir, "nope.json"), "--token", testSetupToken},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := runCmd(t, "", args...)
			var cliErr *cliError
			if !asCLIError(err, &cliErr) || cliErr.code != 2 {
				t.Errorf("error = %v, want usage error (exit 2)", err)
			}
		})
	}
}

func TestSetupApplyIdempotentRerun(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServer(t, 0)
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL)

	if _, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin"); err != nil {
		t.Fatalf("first apply failed: %v\nstderr: %s", err, stderr.String())
	}
	before, err := os.Stat(filepath.Join(dir, "backend", ".env"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin")
	if err != nil {
		t.Fatalf("second apply failed: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "unchanged") {
		t.Errorf("second apply should report unchanged env files: %s", stdout.String())
	}
	after, err := os.Stat(filepath.Join(dir, "backend", ".env"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("unchanged env file must not be rewritten")
	}
}

func TestSetupApplyJSONOutputExcludesTokens(t *testing.T) {
	setupTestEnvIsolation(t)
	server := newSetupTestServer(t, 0)
	defer server.Close()

	dir := t.TempDir()
	planPath := writeSetupPlanFile(t, dir, server.URL)

	stdout, stderr, err := runCmd(t, testSetupToken+"\n", "setup", "apply", "--plan", planPath, "--token-stdin", "--output", "json")
	if err != nil {
		t.Fatalf("setup apply failed: %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "tok_backend-secret") || strings.Contains(stdout.String(), "tok_web-secret") {
		t.Fatal("json output leaked token values")
	}
	var payload struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if len(payload.Projects) != 2 {
		t.Fatalf("projects = %v", payload.Projects)
	}
	for _, p := range payload.Projects {
		if _, exists := p["token"]; exists {
			t.Error("json output must not have a token field")
		}
	}
}

func asCLIError(err error, target **cliError) bool {
	for err != nil {
		if ce, ok := err.(*cliError); ok {
			*target = ce
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}
