# traceway-cli Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundation of traceway-cli — module bootstrap, HTTP client skeleton, config storage, output renderers, and a working `login` → `projects list` vertical slice. End state: a CLI that authenticates against a Traceway instance and lists projects in JSON, YAML, or table format, with proper error handling and profile management.

**Architecture:** Library-first split — `pkg/client` is a pure HTTP+types library (importable by a future MCP server), `internal/config` and `internal/output` are CLI-only. `cmd/traceway/*.go` files are thin glue: parse flags → load config → call client → render output.

**Tech Stack:** Go (stdlib `net/http`), Cobra (CLI framework), Viper (env-var binding only), `golang.org/x/term` (password reading + TTY detection), `gopkg.in/yaml.v3` (YAML output), `text/tabwriter` (tables), `httptest` (tests).

---

## File Map

This plan creates the following files. Each task lists exactly which files it touches.

```
.gitignore                          (modify — add .go/)
flake.nix                           (modify — add `just` to dev shell)
justfile                            (create)
go.mod                              (create — via go mod init)

internal/exitcode/codes.go          (create)
internal/exitcode/codes_test.go     (create)

internal/config/paths.go            (create)
internal/config/paths_test.go       (create)
internal/config/config.go           (create)
internal/config/config_test.go      (create)

pkg/client/errors.go                (create)
pkg/client/errors_test.go           (create)
pkg/client/client.go                (create)
pkg/client/client_test.go           (create)
pkg/client/auth.go                  (create)
pkg/client/auth_test.go             (create)
pkg/client/projects.go              (create)
pkg/client/projects_test.go         (create)

internal/output/format.go           (create)
internal/output/format_test.go      (create)
internal/output/json.go             (create)
internal/output/json_test.go        (create)
internal/output/yaml.go             (create)
internal/output/yaml_test.go        (create)
internal/output/table.go            (create)
internal/output/error.go            (create)
internal/output/error_test.go       (create)

cmd/traceway/main.go                (create)
cmd/traceway/root.go                (create)
cmd/traceway/testutil_test.go       (create — shared test helpers)
cmd/traceway/login.go               (create)
cmd/traceway/login_test.go          (create)
cmd/traceway/logout.go              (create)
cmd/traceway/logout_test.go         (create)
cmd/traceway/profiles.go            (create)
cmd/traceway/profiles_test.go       (create)
cmd/traceway/projects.go            (create)
cmd/traceway/projects_test.go       (create)
cmd/traceway/errors.go              (create — handleAPIError helper)
cmd/traceway/errors_test.go         (create)
```

**File responsibilities:**

- **`internal/exitcode`** — single source of truth for stable exit codes. No deps.
- **`internal/config`** — credential and profile storage (`config.json`). No CLI deps.
- **`pkg/client`** — HTTP client and typed responses. No CLI/config/output deps. The reusable surface.
- **`internal/output`** — rendering JSON/YAML/table and the error envelope. Depends on `exitcode` only.
- **`cmd/traceway`** — Cobra subcommands. Glue — parse flags, call client, call output.

---

## Task 1: Bootstrap module and tooling

**Files:**
- Create: `go.mod` (via `go mod init`)
- Modify: `.gitignore` (currently contains `.go/` already per dev shell — verify)
- Modify: `flake.nix` (add `just` to dev shell)
- Create: `justfile`

- [ ] **Step 1: Initialize the Go module**

Run: `go mod init github.com/tracewayapp/traceway/cli`
Expected: creates `go.mod` with `module github.com/tracewayapp/traceway/cli` and the current Go version.

- [ ] **Step 2: Verify `.gitignore` contains `.go/`**

Read `.gitignore`. If it doesn't already contain `.go/`, add it. The dev shell pins `GOPATH`/`GOCACHE`/`GOBIN` inside `.go/` and we don't want those committed.

Expected `.gitignore` contents (at minimum):
```
.go/
```

- [ ] **Step 3: Add `just` to the Nix dev shell**

Read `flake.nix`. Find the `buildInputs` (or `packages`) list inside `mkShell`. Add `just` to it.

For example, if you see:
```nix
buildInputs = with pkgs; [ go gopls gotools delve gofumpt gomodifytags impl gotestsum golangci-lint govulncheck gh git jq ];
```
Change to:
```nix
buildInputs = with pkgs; [ go gopls gotools delve gofumpt gomodifytags impl gotestsum golangci-lint govulncheck gh git jq just ];
```

- [ ] **Step 4: Create the justfile**

Create `justfile` (no extension) at the repo root:

```just
default: test

test:
    gotestsum --format pkgname -- ./...

smoke-test:
    gotestsum --format pkgname -- -tags smoke ./test/smoke/...

lint:
    golangci-lint run

vulncheck:
    govulncheck ./...

# Run everything that should pass before a commit
check: lint test vulncheck
```

- [ ] **Step 5: Verify the toolchain works**

Run: `nix develop --command which just`
Expected: a path under `/nix/store/...`.

Run: `nix develop --command go mod download`
Expected: no output (no deps yet) and exit 0.

Run: `nix develop --command just --list`
Expected: lists `default`, `test`, `smoke-test`, `lint`, `vulncheck`, `check`.

Note: Do **not** run `just test` yet — there are no tests.

- [ ] **Step 6: Commit**

```bash
git add go.mod .gitignore flake.nix justfile
git commit -m "chore: bootstrap go module, justfile, just in dev shell"
```

---

## Task 2: Exit code constants

**Files:**
- Create: `internal/exitcode/codes.go`
- Test: `internal/exitcode/codes_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/exitcode/codes_test.go`:

```go
package exitcode

import "testing"

func TestCodes_areStable(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"Success", Success, 0},
		{"Generic", Generic, 1},
		{"Usage", Usage, 2},
		{"Connection", Connection, 3},
		{"Auth", Auth, 4},
		{"NotFound", NotFound, 5},
		{"RateLimited", RateLimited, 6},
		{"Server", Server, 7},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `go test ./internal/exitcode/...`
Expected: build failure — `undefined: Success` (etc.).

- [ ] **Step 3: Implement the constants**

Create `internal/exitcode/codes.go`:

```go
// Package exitcode defines the stable exit codes emitted by the traceway CLI.
// LLMs and scripts may branch on these values; do not renumber.
package exitcode

const (
	Success     = 0
	Generic     = 1
	Usage       = 2
	Connection  = 3
	Auth        = 4
	NotFound    = 5
	RateLimited = 6
	Server      = 7
)
```

- [ ] **Step 4: Run the test, verify it passes**

Run: `go test ./internal/exitcode/...`
Expected: `ok  github.com/tracewayapp/traceway/cli/internal/exitcode`.

- [ ] **Step 5: Commit**

```bash
git add internal/exitcode/
git commit -m "feat(exitcode): define stable exit code constants"
```

---

## Task 3: Config — XDG path resolution

**Files:**
- Create: `internal/config/paths.go`
- Test: `internal/config/paths_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/paths_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestConfigPath_xdgSet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	t.Setenv("HOME", "/tmp/home")

	got, err := configPath()
	if err != nil {
		t.Fatalf("configPath() error: %v", err)
	}
	want := filepath.Join("/tmp/xdg", "traceway", "config.json")
	if got != want {
		t.Errorf("configPath() = %q, want %q", got, want)
	}
}

func TestConfigPath_xdgUnset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/home")

	got, err := configPath()
	if err != nil {
		t.Fatalf("configPath() error: %v", err)
	}
	want := filepath.Join("/tmp/home", ".config", "traceway", "config.json")
	if got != want {
		t.Errorf("configPath() = %q, want %q", got, want)
	}
}

func TestConfigPath_neither(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	if _, err := configPath(); err == nil {
		t.Fatal("expected error when both XDG_CONFIG_HOME and HOME are empty")
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `go test ./internal/config/...`
Expected: build failure — `undefined: configPath`.

- [ ] **Step 3: Implement `configPath`**

Create `internal/config/paths.go`:

```go
package config

import (
	"errors"
	"os"
	"path/filepath"
)

// configPath returns the path to the config file, resolving XDG_CONFIG_HOME
// or falling back to $HOME/.config. It does not create the file.
func configPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "traceway", "config.json"), nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("neither XDG_CONFIG_HOME nor HOME is set")
	}
	return filepath.Join(home, ".config", "traceway", "config.json"), nil
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): resolve XDG-aware config path"
```

---

## Task 4: Config — types and Load

**Files:**
- Create: `internal/config/config.go`
- Modify: `internal/config/config_test.go` (create)

- [ ] **Step 1: Write the failing tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_missingFile_returnsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil cfg")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected empty Profiles, got %v", cfg.Profiles)
	}
	if cfg.CurrentProfile != "" {
		t.Errorf("expected empty CurrentProfile, got %q", cfg.CurrentProfile)
	}
}

func TestLoad_existingFile_readsJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "traceway"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{
		"current_profile": "stormwind",
		"profiles": {
			"stormwind": {
				"url": "https://traceway.stormwind.local",
				"username": "fred@example.com",
				"jwt": "abc.def.ghi",
				"current_project_id": "proj-1"
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "traceway", "config.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.CurrentProfile != "stormwind" {
		t.Errorf("CurrentProfile = %q, want %q", cfg.CurrentProfile, "stormwind")
	}
	p, ok := cfg.Profiles["stormwind"]
	if !ok {
		t.Fatal("stormwind profile not loaded")
	}
	if p.URL != "https://traceway.stormwind.local" {
		t.Errorf("URL = %q", p.URL)
	}
	if p.JWT != "abc.def.ghi" {
		t.Errorf("JWT = %q", p.JWT)
	}
	if p.CurrentProjectID != "proj-1" {
		t.Errorf("CurrentProjectID = %q", p.CurrentProjectID)
	}
}

func TestLoad_corruptFile_returnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "traceway"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "traceway", "config.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("expected Load() to fail on corrupt JSON")
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/config/...`
Expected: build failure — `undefined: Load`, `undefined: Config`.

- [ ] **Step 3: Implement `Config` and `Load`**

Create `internal/config/config.go`:

```go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Config is the on-disk credential and preference file.
type Config struct {
	CurrentProfile string             `json:"current_profile"`
	Profiles       map[string]Profile `json:"profiles"`
}

// Profile holds credentials and preferences for a single Traceway instance.
type Profile struct {
	URL              string `json:"url"`
	Username         string `json:"username"`
	JWT              string `json:"jwt"`
	CurrentProjectID string `json:"current_project_id,omitempty"`
}

// Load reads the config file from disk. A missing file yields an empty Config
// (not an error) — the caller treats absence of credentials as an auth error
// only when an actual command needs them.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return &cfg, nil
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): define Config/Profile types and Load"
```

---

## Task 5: Config — atomic Save with strict perms

**Files:**
- Modify: `internal/config/config.go` (add `Save` method)
- Modify: `internal/config/config_test.go` (add tests for `Save`)

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/config_test.go`:

```go
func TestSave_writesAtomicallyWith0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := &Config{
		CurrentProfile: "default",
		Profiles: map[string]Profile{
			"default": {
				URL:      "https://cloud.tracewayapp.com",
				Username: "fred@example.com",
				JWT:      "tok",
			},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	path := filepath.Join(dir, "traceway", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 0600", perm)
	}

	dirInfo, err := os.Stat(filepath.Join(dir, "traceway"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 0700", perm)
	}
}

func TestSave_overwritesExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	first := &Config{
		Profiles: map[string]Profile{"a": {URL: "https://a"}},
	}
	if err := first.Save(); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	second := &Config{
		Profiles: map[string]Profile{"b": {URL: "https://b"}},
	}
	if err := second.Save(); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := loaded.Profiles["a"]; ok {
		t.Error("profile 'a' should have been overwritten")
	}
	if _, ok := loaded.Profiles["b"]; !ok {
		t.Error("profile 'b' should exist after second Save")
	}
}

func TestSave_thenLoad_roundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := &Config{
		CurrentProfile: "stormwind",
		Profiles: map[string]Profile{
			"stormwind": {
				URL:              "https://traceway.stormwind.local",
				Username:         "fred@example.com",
				JWT:              "tok",
				CurrentProjectID: "proj-1",
			},
		},
	}
	if err := want.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentProfile != want.CurrentProfile {
		t.Errorf("CurrentProfile mismatch")
	}
	if got.Profiles["stormwind"] != want.Profiles["stormwind"] {
		t.Errorf("Profile mismatch: got %+v want %+v", got.Profiles["stormwind"], want.Profiles["stormwind"])
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/config/...`
Expected: build failure — `cfg.Save undefined`.

- [ ] **Step 3: Implement `Save`**

Append to `internal/config/config.go`:

```go
import (
	"path/filepath"
)

// Save atomically writes the config to disk. Creates parent dirs (0700) and the
// file (0600). Atomicity is achieved by writing to a tempfile in the same
// directory and renaming over the destination.
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".config.json.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming into place: %w", err)
	}
	return nil
}
```

Make sure the imports at the top of the file include `path/filepath`. Combine with the existing import block.

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): atomic Save with strict 0600/0700 perms"
```

---

## Task 6: Config — Active profile resolution

**Files:**
- Modify: `internal/config/config.go` (add `Active` method)
- Modify: `internal/config/config_test.go` (add tests)

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/config_test.go`:

```go
func TestActive_explicitName(t *testing.T) {
	cfg := &Config{
		CurrentProfile: "stormwind",
		Profiles: map[string]Profile{
			"stormwind": {URL: "https://a"},
			"cloud":     {URL: "https://b"},
		},
	}
	p, err := cfg.Active("cloud")
	if err != nil {
		t.Fatal(err)
	}
	if p.URL != "https://b" {
		t.Errorf("got %q, want https://b", p.URL)
	}
}

func TestActive_emptyName_usesCurrentProfile(t *testing.T) {
	cfg := &Config{
		CurrentProfile: "stormwind",
		Profiles: map[string]Profile{
			"stormwind": {URL: "https://a"},
		},
	}
	p, err := cfg.Active("")
	if err != nil {
		t.Fatal(err)
	}
	if p.URL != "https://a" {
		t.Errorf("got %q, want https://a", p.URL)
	}
}

func TestActive_emptyName_emptyCurrent_usesDefault(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"default": {URL: "https://d"},
		},
	}
	p, err := cfg.Active("")
	if err != nil {
		t.Fatal(err)
	}
	if p.URL != "https://d" {
		t.Errorf("got %q, want https://d", p.URL)
	}
}

func TestActive_unknownName_returnsError(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{"default": {URL: "https://d"}},
	}
	if _, err := cfg.Active("nonexistent"); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestActive_emptyConfig_returnsError(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	if _, err := cfg.Active(""); err == nil {
		t.Fatal("expected error when no profiles exist")
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/config/...`
Expected: build failure — `cfg.Active undefined`.

- [ ] **Step 3: Implement `Active`**

Append to `internal/config/config.go`:

```go
// ErrProfileNotFound is returned by Active when the requested profile does not
// exist in the config.
var ErrProfileNotFound = errors.New("profile not found")

// Active resolves the effective profile by precedence:
//
//	explicit name (e.g. from --profile) > c.CurrentProfile > "default"
//
// Returns ErrProfileNotFound if the resolved name has no profile in the config.
func (c *Config) Active(name string) (*Profile, error) {
	resolved := name
	if resolved == "" {
		resolved = c.CurrentProfile
	}
	if resolved == "" {
		resolved = "default"
	}
	p, ok := c.Profiles[resolved]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrProfileNotFound, resolved)
	}
	return &p, nil
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): Active profile resolution with --profile precedence"
```

---

## Task 7: pkg/client — typed errors

**Files:**
- Create: `pkg/client/errors.go`
- Test: `pkg/client/errors_test.go`

- [ ] **Step 1: Write the failing tests**

Create `pkg/client/errors_test.go`:

```go
package client

import (
	"errors"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	e := &APIError{StatusCode: 500, Body: "boom"}
	got := e.Error()
	if got == "" {
		t.Fatal("Error() returned empty string")
	}
}

func TestSentinelErrors_areDistinct(t *testing.T) {
	all := []error{ErrUnauthorized, ErrForbidden, ErrNotFound, ErrRateLimited}
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("errors.Is(%v, %v) = true; expected distinct", a, b)
			}
		}
	}
}

func TestAPIError_doesNotMatchSentinels(t *testing.T) {
	apiErr := &APIError{StatusCode: 418}
	if errors.Is(apiErr, ErrUnauthorized) {
		t.Error("APIError(418) should not match ErrUnauthorized")
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./pkg/client/...`
Expected: build failure — undefined identifiers.

- [ ] **Step 3: Implement the error types**

Create `pkg/client/errors.go`:

```go
// Package client is the HTTP client and types for talking to a Traceway instance.
//
// This package has no dependencies on Cobra, Viper, or any CLI machinery so
// that a future MCP server can import it directly.
package client

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by client methods. Use errors.Is to test.
var (
	ErrUnauthorized = errors.New("unauthorized (401)")
	ErrForbidden    = errors.New("forbidden (403)")
	ErrNotFound     = errors.New("not found (404)")
	ErrRateLimited  = errors.New("rate limited (429)")
)

// APIError is returned for any non-2xx response that isn't covered by a sentinel.
// Inspect StatusCode and Body for diagnostics.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("traceway API error: status %d", e.StatusCode)
	}
	return fmt.Sprintf("traceway API error: status %d: %s", e.StatusCode, e.Body)
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./pkg/client/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/client/
git commit -m "feat(client): define typed errors and APIError"
```

---

## Task 8: pkg/client — Client struct and `do()` helper

**Files:**
- Create: `pkg/client/client.go`
- Test: `pkg/client/client_test.go`

- [ ] **Step 1: Write the failing tests**

Create `pkg/client/client_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_setsDefaults(t *testing.T) {
	c := New("https://example.com")
	if c.BaseURL != "https://example.com" {
		t.Errorf("BaseURL = %q", c.BaseURL)
	}
	if c.HTTPClient == nil {
		t.Error("HTTPClient should default to a non-nil client")
	}
	if c.UserAgent == "" {
		t.Error("UserAgent should have a default value")
	}
}

func TestWithJWT_setsJWT(t *testing.T) {
	c := New("https://example.com", WithJWT("tok"))
	if c.JWT != "tok" {
		t.Errorf("JWT = %q", c.JWT)
	}
}

func TestDo_setsHeadersAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "traceway-cli") {
			t.Errorf("User-Agent = %q", got)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["foo"] != "bar" {
			t.Errorf("body.foo = %q", body["foo"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	var resp struct {
		OK bool `json:"ok"`
	}
	err := c.do(context.Background(), http.MethodPost, "/api/test", map[string]string{"foo": "bar"}, &resp)
	if err != nil {
		t.Fatalf("do(): %v", err)
	}
	if !resp.OK {
		t.Error("expected resp.OK = true")
	}
}

func TestDo_omitsAuthHeaderWhenJWTEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("expected no Authorization header, got %q", got)
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	var resp struct{}
	if err := c.do(context.Background(), http.MethodPost, "/api/x", nil, &resp); err != nil {
		t.Fatal(err)
	}
}

func TestDo_mapsStatusCodes(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{401, ErrUnauthorized},
		{403, ErrForbidden},
		{404, ErrNotFound},
		{429, ErrRateLimited},
	}
	for _, c := range cases {
		c := c
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.status)
			}))
			defer srv.Close()

			cli := New(srv.URL)
			var resp struct{}
			err := cli.do(context.Background(), http.MethodPost, "/api/x", nil, &resp)
			if !errors.Is(err, c.want) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, c.want)
			}
		})
	}
}

func TestDo_returnsAPIErrorForOtherStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	cli := New(srv.URL)
	var resp struct{}
	err := cli.do(context.Background(), http.MethodPost, "/api/x", nil, &resp)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if apiErr.Body != "boom" {
		t.Errorf("Body = %q", apiErr.Body)
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./pkg/client/...`
Expected: build failure — `undefined: New`, `undefined: WithJWT`, `cli.do undefined`.

- [ ] **Step 3: Implement the Client**

Create `pkg/client/client.go`:

```go
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to a Traceway HTTP API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	JWT        string
	UserAgent  string
}

// Option mutates a Client during construction.
type Option func(*Client)

// New returns a Client with sane defaults. The baseURL is normalized by
// stripping trailing slashes; do() prepends "/api/..." paths.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  "traceway-cli/0.1",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithHTTPClient injects a custom *http.Client (useful for tests).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.HTTPClient = h }
}

// WithJWT sets the bearer token to send on every request.
func WithJWT(jwt string) Option {
	return func(c *Client) { c.JWT = jwt }
}

// do is the internal HTTP transport. It JSON-encodes body (if non-nil),
// JSON-decodes the response into out (if non-nil), and maps non-2xx status
// codes to typed errors.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.JWT != "" {
		req.Header.Set("Authorization", "Bearer "+c.JWT)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil {
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
		return nil
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	}
	respBody, _ := io.ReadAll(resp.Body)
	return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./pkg/client/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/client/
git commit -m "feat(client): Client struct, options, do() with typed error mapping"
```

---

## Task 9: pkg/client — `Login`

**Files:**
- Create: `pkg/client/auth.go`
- Test: `pkg/client/auth_test.go`

**Note on the endpoint shape:** Per the spec, login is `POST /api/login` with `{email, password}`. The response shape needs to be confirmed against `tracewayapp/traceway/backend/app/controllers/auth.controller.go`. The most common shape is `{"token": "..."}` or `{"jwt": "..."}`. Before writing the test, fetch the controller and confirm:

```bash
gh api repos/tracewayapp/traceway/contents/backend/app/controllers/auth.controller.go --jq .content | base64 -d
```

If the response field is named differently (e.g. `accessToken`), adjust the `loginResponse` struct's JSON tag accordingly. The test below uses `token` — change it if upstream uses something else.

- [ ] **Step 1: Confirm the upstream response field name**

Fetch the controller (command above) and read it. Look for the JSON response shape returned by the login handler. Note the field name. Default assumption used in this task: `token`.

- [ ] **Step 2: Write the failing tests**

Create `pkg/client/auth_test.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogin_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/login" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["email"] != "fred@example.com" {
			t.Errorf("email = %q", req["email"])
		}
		if req["password"] != "hunter2" {
			t.Errorf("password = %q", req["password"])
		}
		_, _ = w.Write([]byte(`{"token": "jwt.value.here"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	jwt, err := c.Login(context.Background(), "fred@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if jwt != "jwt.value.here" {
		t.Errorf("jwt = %q", jwt)
	}
}

func TestLogin_invalidCredentials_returnsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Login(context.Background(), "x", "y")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("got %v, want ErrUnauthorized", err)
	}
}
```

- [ ] **Step 3: Run the tests, verify they fail**

Run: `go test ./pkg/client/...`
Expected: build failure — `c.Login undefined`.

- [ ] **Step 4: Implement `Login`**

Create `pkg/client/auth.go`:

```go
package client

import (
	"context"
	"errors"
	"net/http"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	// If upstream uses "accessToken" or "jwt" instead of "token", change this tag.
	Token string `json:"token"`
}

// Login exchanges an email + password for a JWT. The returned token should be
// stored by the caller and passed to subsequent Client constructions via
// WithJWT.
func (c *Client) Login(ctx context.Context, email, password string) (string, error) {
	var resp loginResponse
	if err := c.do(ctx, http.MethodPost, "/api/login", loginRequest{Email: email, Password: password}, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", errors.New("login response did not include a token")
	}
	return resp.Token, nil
}
```

- [ ] **Step 5: Run the tests, verify they pass**

Run: `go test ./pkg/client/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/client/
git commit -m "feat(client): Login(email, password) → JWT"
```

---

## Task 10: pkg/client — `ListProjects`

**Files:**
- Create: `pkg/client/projects.go`
- Test: `pkg/client/projects_test.go`

**Note on shape:** `POST /api/projects` lists projects. Confirm the response shape against `tracewayapp/traceway/backend/app/controllers/project.controller.go` and the model. Most likely it's `{"data": [{"id": "...", "name": "..."}, ...], "pagination": {...}}`. Adjust struct tags if upstream uses different field names.

- [ ] **Step 1: Confirm upstream response shape**

```bash
gh api repos/tracewayapp/traceway/contents/backend/app/controllers/project.controller.go --jq .content | base64 -d
gh api repos/tracewayapp/traceway/contents/backend/app/models/project.model.go --jq .content | base64 -d
```

Note the Project model field names. Default assumption used in this task: `id`, `name`. Add other fields you observe (e.g. `createdAt`) — but only if you'll use them; YAGNI.

- [ ] **Step 2: Write the failing tests**

Create `pkg/client/projects_test.go`:

```go
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListProjects_decodesData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "p1", "name": "stormwind-prod"},
				{"id": "p2", "name": "stormwind-staging"}
			],
			"pagination": {"page": 0, "pageSize": 50, "total": 2}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("got %d projects, want 2", len(resp.Data))
	}
	if resp.Data[0].ID != "p1" {
		t.Errorf("Data[0].ID = %q", resp.Data[0].ID)
	}
	if resp.Data[1].Name != "stormwind-staging" {
		t.Errorf("Data[1].Name = %q", resp.Data[1].Name)
	}
	if resp.Pagination.Total != 2 {
		t.Errorf("Pagination.Total = %d", resp.Pagination.Total)
	}
}
```

- [ ] **Step 3: Run the tests, verify they fail**

Run: `go test ./pkg/client/...`
Expected: build failure.

- [ ] **Step 4: Implement `Pagination`, `Project`, and `ListProjects`**

Create `pkg/client/projects.go`:

```go
package client

import (
	"context"
	"net/http"
)

// Pagination matches Traceway's pagination block on every list response.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

// Project is the minimal project shape we need today. Add fields as commands
// require them; do not pre-emptively mirror the entire upstream model.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListProjectsResponse is the shape of POST /api/projects.
type ListProjectsResponse struct {
	Data       []Project  `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// ListProjects returns all projects visible to the authenticated user.
func (c *Client) ListProjects(ctx context.Context) (*ListProjectsResponse, error) {
	var resp ListProjectsResponse
	// Empty body — Traceway accepts {} or no body for the default listing.
	if err := c.do(ctx, http.MethodPost, "/api/projects", map[string]any{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
```

- [ ] **Step 5: Run the tests, verify they pass**

Run: `go test ./pkg/client/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/client/
git commit -m "feat(client): Pagination, Project, ListProjects"
```

---

## Task 11: output — Mode enum and TTY detection

**Files:**
- Create: `internal/output/format.go`
- Test: `internal/output/format_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/output/format_test.go`:

```go
package output

import (
	"testing"
)

func TestParseMode_validValues(t *testing.T) {
	cases := map[string]Mode{
		"json":  ModeJSON,
		"JSON":  ModeJSON,
		"yaml":  ModeYAML,
		"table": ModeTable,
	}
	for in, want := range cases {
		got, err := ParseMode(in)
		if err != nil {
			t.Errorf("ParseMode(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseMode(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseMode_invalid(t *testing.T) {
	if _, err := ParseMode("xml"); err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestResolveMode_explicitWins(t *testing.T) {
	got := ResolveMode("json", false)
	if got != ModeJSON {
		t.Errorf("got %v", got)
	}
	got = ResolveMode("table", false)
	if got != ModeTable {
		t.Errorf("got %v", got)
	}
}

func TestResolveMode_emptyDefaultsByTTY(t *testing.T) {
	if got := ResolveMode("", true); got != ModeTable {
		t.Errorf("TTY default = %v, want ModeTable", got)
	}
	if got := ResolveMode("", false); got != ModeJSON {
		t.Errorf("non-TTY default = %v, want ModeJSON", got)
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/output/...`
Expected: build failure.

- [ ] **Step 3: Implement the `Mode` type**

Create `internal/output/format.go`:

```go
// Package output renders command results as JSON, YAML, or tables, plus the
// stable error envelope.
package output

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Mode controls how Render formats its output.
type Mode int

const (
	ModeJSON Mode = iota
	ModeYAML
	ModeTable
)

func (m Mode) String() string {
	switch m {
	case ModeJSON:
		return "json"
	case ModeYAML:
		return "yaml"
	case ModeTable:
		return "table"
	default:
		return "unknown"
	}
}

// ParseMode converts the user-facing flag value to a Mode.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(s) {
	case "json":
		return ModeJSON, nil
	case "yaml":
		return ModeYAML, nil
	case "table":
		return ModeTable, nil
	default:
		return 0, fmt.Errorf("invalid output mode %q (valid: json, yaml, table)", s)
	}
}

// ResolveMode picks an effective Mode. An explicit user value wins; otherwise
// default to table on TTY and json on non-TTY.
//
// The error returned by ParseMode for invalid explicit values is intentionally
// suppressed here — the caller should validate via ParseMode at flag-parse time.
func ResolveMode(explicit string, isTTY bool) Mode {
	if explicit != "" {
		if m, err := ParseMode(explicit); err == nil {
			return m
		}
	}
	if isTTY {
		return ModeTable
	}
	return ModeJSON
}

// IsTerminal reports whether the given file descriptor is a terminal.
// Wraps golang.org/x/term so callers don't import it directly.
func IsTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// StdoutIsTerminal is a convenience shortcut.
func StdoutIsTerminal() bool { return IsTerminal(os.Stdout.Fd()) }

// StderrIsTerminal is a convenience shortcut.
func StderrIsTerminal() bool { return IsTerminal(os.Stderr.Fd()) }
```

- [ ] **Step 4: Add the term dependency**

Run: `go get golang.org/x/term`
Expected: `go.sum` and `go.mod` updated.

- [ ] **Step 5: Run the tests, verify they pass**

Run: `go test ./internal/output/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/output/ go.mod go.sum
git commit -m "feat(output): Mode enum, ParseMode, ResolveMode, TTY detection"
```

---

## Task 12: output — JSON renderer with `--fields` projection

**Files:**
- Create: `internal/output/json.go`
- Test: `internal/output/json_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/output/json_test.go`:

```go
package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type wrappedResp struct {
	Data       []project `json:"data"`
	Pagination struct {
		Total int `json:"total"`
	} `json:"pagination"`
}

func TestRenderJSON_passThroughWhenNoFields(t *testing.T) {
	in := wrappedResp{
		Data: []project{{ID: "p1", Name: "alpha", URL: "https://a"}},
	}
	in.Pagination.Total = 1

	var buf bytes.Buffer
	if err := RenderJSON(&buf, in, nil); err != nil {
		t.Fatal(err)
	}

	var got wrappedResp
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}
	if got.Data[0].URL != "https://a" {
		t.Errorf("URL was stripped; pass-through expected")
	}
}

func TestRenderJSON_projectsFieldsInWrappedData(t *testing.T) {
	in := wrappedResp{
		Data: []project{
			{ID: "p1", Name: "alpha", URL: "https://a"},
			{ID: "p2", Name: "beta", URL: "https://b"},
		},
	}

	var buf bytes.Buffer
	if err := RenderJSON(&buf, in, []string{"id", "name"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, `"id"`) || !strings.Contains(out, `"name"`) {
		t.Errorf("expected id and name in projection, got: %s", out)
	}
	if strings.Contains(out, `"url"`) {
		t.Errorf("url should have been projected away, got: %s", out)
	}
	// Pagination stays at the top level even when fields are projected.
	if !strings.Contains(out, `"pagination"`) {
		t.Errorf("pagination should pass through, got: %s", out)
	}
}

func TestRenderJSON_projectsTopLevelObject(t *testing.T) {
	in := project{ID: "p1", Name: "alpha", URL: "https://a"}

	var buf bytes.Buffer
	if err := RenderJSON(&buf, in, []string{"id"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"id"`) {
		t.Errorf("expected id, got: %s", out)
	}
	if strings.Contains(out, `"name"`) {
		t.Errorf("name should have been projected away, got: %s", out)
	}
}

func TestParseFieldsFlag(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ", []string{"a", "b"}},
	}
	for _, c := range cases {
		got := ParseFieldsFlag(c.in)
		if len(got) != len(c.want) {
			t.Errorf("ParseFieldsFlag(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParseFieldsFlag(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/output/...`
Expected: build failure.

- [ ] **Step 3: Implement the JSON renderer + projection**

Create `internal/output/json.go`:

```go
package output

import (
	"encoding/json"
	"io"
	"strings"
)

// ParseFieldsFlag splits "a, b, c" → []string{"a", "b", "c"}. Empty input → nil.
func ParseFieldsFlag(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// RenderJSON marshals v as indented JSON to w. If fields is non-nil, projects
// each item of a top-level "data" array (Traceway's wrapper shape) — or the
// top-level object itself if it has no "data" key — to just the named fields.
// "pagination" and other top-level wrapper keys pass through unchanged.
func RenderJSON(w io.Writer, v any, fields []string) error {
	if fields == nil {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}

	// Round-trip through JSON to a generic any so we can project by string keys.
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	projected := projectFields(generic, fields)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(projected)
}

// projectFields keeps only the named keys, with awareness of Traceway's
// {data: [...], pagination: {...}} wrapper. See RenderJSON for the contract.
func projectFields(v any, fields []string) any {
	set := make(map[string]bool, len(fields))
	for _, f := range fields {
		set[f] = true
	}

	if m, ok := v.(map[string]any); ok {
		if data, hasData := m["data"]; hasData {
			if arr, ok := data.([]any); ok {
				out := make([]any, len(arr))
				for i, item := range arr {
					out[i] = projectMap(item, set)
				}
				m["data"] = out
				return m
			}
		}
		return projectMap(v, set)
	}
	if arr, ok := v.([]any); ok {
		out := make([]any, len(arr))
		for i, item := range arr {
			out[i] = projectMap(item, set)
		}
		return out
	}
	return v
}

func projectMap(v any, fields map[string]bool) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(fields))
	for k, val := range m {
		if fields[k] {
			out[k] = val
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./internal/output/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/
git commit -m "feat(output): JSON renderer with --fields projection"
```

---

## Task 13: output — YAML renderer

**Files:**
- Create: `internal/output/yaml.go`
- Test: `internal/output/yaml_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/output/yaml_test.go`:

```go
package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderYAML_passThrough(t *testing.T) {
	in := project{ID: "p1", Name: "alpha", URL: "https://a"}

	var buf bytes.Buffer
	if err := RenderYAML(&buf, in, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "id: p1") {
		t.Errorf("expected 'id: p1' in YAML, got:\n%s", out)
	}
	if !strings.Contains(out, "name: alpha") {
		t.Errorf("expected 'name: alpha' in YAML, got:\n%s", out)
	}
	if !strings.Contains(out, "url: https://a") {
		t.Errorf("expected 'url: https://a' in YAML, got:\n%s", out)
	}
}

func TestRenderYAML_projectsFields(t *testing.T) {
	in := project{ID: "p1", Name: "alpha", URL: "https://a"}

	var buf bytes.Buffer
	if err := RenderYAML(&buf, in, []string{"id"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "id: p1") {
		t.Errorf("expected id in projection, got:\n%s", out)
	}
	if strings.Contains(out, "name:") {
		t.Errorf("name should be projected away, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/output/...`
Expected: build failure.

- [ ] **Step 3: Add the yaml dependency and implement**

Run: `go get gopkg.in/yaml.v3`
Expected: `go.mod`/`go.sum` updated.

Create `internal/output/yaml.go`:

```go
package output

import (
	"encoding/json"
	"io"

	"gopkg.in/yaml.v3"
)

// RenderYAML marshals v as YAML to w, applying the same field projection as
// RenderJSON when fields is non-nil. Internally we round-trip through JSON so
// that callers' types only need json struct tags (no need for parallel yaml
// tags on every struct in pkg/client).
func RenderYAML(w io.Writer, v any, fields []string) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	if fields != nil {
		generic = projectFields(generic, fields)
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(generic)
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./internal/output/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/ go.mod go.sum
git commit -m "feat(output): YAML renderer (JSON-backed) with --fields projection"
```

---

## Task 14: output — table helper

**Files:**
- Create: `internal/output/table.go`
- (No dedicated test file — table.go is just a tabwriter constructor; per-resource table rendering is tested in the cmd layer.)

- [ ] **Step 1: Implement the table helper**

Per the spec, table rendering is per-resource (each `cmd/traceway/<resource>.go` writes its own columns). This file just provides a configured `text/tabwriter` so all tables look consistent.

Create `internal/output/table.go`:

```go
package output

import (
	"io"
	"text/tabwriter"
)

// NewTabWriter returns a *text/tabwriter.Writer configured for traceway's
// table output style: left-aligned columns separated by two spaces. Callers
// must call Flush() after writing all rows.
//
//	tw := output.NewTabWriter(w)
//	fmt.Fprintln(tw, "ID\tNAME")
//	fmt.Fprintf(tw, "%s\t%s\n", id, name)
//	tw.Flush()
func NewTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}
```

- [ ] **Step 2: Verify the package still builds**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/output/
git commit -m "feat(output): NewTabWriter helper for per-resource tables"
```

---

## Task 15: output — error envelope renderer

**Files:**
- Create: `internal/output/error.go`
- Test: `internal/output/error_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/output/error_test.go`:

```go
package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderError_jsonShape(t *testing.T) {
	var buf bytes.Buffer
	err := RenderError(&buf, ModeJSON, ErrorEnvelope{
		Code:     "token_expired",
		Message:  "JWT expired or invalid",
		Hint:     "traceway login --profile stormwind",
		ExitCode: 4,
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, buf.String())
	}
	if got["error"] != "token_expired" {
		t.Errorf("error = %v", got["error"])
	}
	if got["message"] != "JWT expired or invalid" {
		t.Errorf("message = %v", got["message"])
	}
	if got["hint"] != "traceway login --profile stormwind" {
		t.Errorf("hint = %v", got["hint"])
	}
	if int(got["exit_code"].(float64)) != 4 {
		t.Errorf("exit_code = %v", got["exit_code"])
	}
}

func TestRenderError_jsonOmitsEmptyHint(t *testing.T) {
	var buf bytes.Buffer
	_ = RenderError(&buf, ModeJSON, ErrorEnvelope{
		Code:     "internal",
		Message:  "boom",
		ExitCode: 1,
	})
	var got map[string]any
	_ = json.Unmarshal(buf.Bytes(), &got)
	if _, ok := got["hint"]; ok {
		t.Errorf("hint should be omitted when empty, got: %v", got)
	}
}

func TestRenderError_proseHasErrorPrefix(t *testing.T) {
	var buf bytes.Buffer
	_ = RenderError(&buf, ModeTable, ErrorEnvelope{
		Code:     "token_expired",
		Message:  "session expired",
		Hint:     "traceway login --profile stormwind",
		ExitCode: 4,
	})
	out := buf.String()
	if !strings.HasPrefix(out, "Error:") {
		t.Errorf("prose form should start with 'Error:', got:\n%s", out)
	}
	if !strings.Contains(out, "session expired") {
		t.Errorf("missing message, got:\n%s", out)
	}
	if !strings.Contains(out, "Hint:") {
		t.Errorf("missing hint line, got:\n%s", out)
	}
}

func TestRenderError_proseOmitsHintLineWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	_ = RenderError(&buf, ModeTable, ErrorEnvelope{
		Code:     "internal",
		Message:  "boom",
		ExitCode: 1,
	})
	out := buf.String()
	if strings.Contains(out, "Hint:") {
		t.Errorf("Hint line should be omitted, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/output/...`
Expected: build failure.

- [ ] **Step 3: Implement the error renderer**

Create `internal/output/error.go`:

```go
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// ErrorEnvelope is the stable error contract written to stderr on any failure.
// Code is a snake_case stable identifier; LLMs may branch on it.
type ErrorEnvelope struct {
	Code     string `json:"error"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// RenderError writes the envelope to w. JSON for ModeJSON/ModeYAML; prose for
// ModeTable. (YAML mode uses JSON for errors — easier for callers to parse and
// matches gh's behavior for machine-formatted errors.)
func RenderError(w io.Writer, mode Mode, env ErrorEnvelope) error {
	if mode == ModeJSON || mode == ModeYAML {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(env)
	}
	if _, err := fmt.Fprintf(w, "Error: %s\n", env.Message); err != nil {
		return err
	}
	if env.Hint != "" {
		if _, err := fmt.Fprintf(w, "  Hint: %s\n", env.Hint); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./internal/output/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/
git commit -m "feat(output): error envelope renderer (JSON + prose)"
```

---

## Task 16: cmd/traceway — main, root command, global flags

**Files:**
- Create: `cmd/traceway/main.go`
- Create: `cmd/traceway/root.go`

- [ ] **Step 1: Add the Cobra dependency**

Run: `go get github.com/spf13/cobra`
Expected: `go.mod`/`go.sum` updated.

- [ ] **Step 2: Implement `main.go`**

Create `cmd/traceway/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := newRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		// All command runners are expected to write the user-facing error
		// message themselves (via internal/output). This branch is the last
		// resort for errors Cobra returned without a message — most often
		// usage errors, which Cobra has already printed to stderr.
		if msg := err.Error(); msg != "" && os.Getenv("TRACEWAY_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "debug:", msg)
		}
		os.Exit(exitcode.Generic)
	}
}
```

- [ ] **Step 3: Implement `root.go`**

Create `cmd/traceway/root.go`:

```go
package main

import (
	"github.com/spf13/cobra"
)

// Global flag values, populated by Cobra at flag-parse time.
var (
	flagProfile      string
	flagProject      string
	flagOutput       string
	flagFields       string
	flagYes          bool
	flagNoPrompt     bool
	flagPasswordFile string
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "traceway",
		Short:         "CLI for the Traceway observability platform",
		SilenceUsage:  true, // we render our own error envelopes
		SilenceErrors: true,
	}

	pf := cmd.PersistentFlags()
	pf.StringVar(&flagProfile, "profile", "", "Profile name (default: current profile, then \"default\")")
	pf.StringVar(&flagProject, "project", "", "Project ID (default: profile's current project)")
	pf.StringVarP(&flagOutput, "output", "o", "", "Output format: json, yaml, or table (default: table on TTY, json otherwise)")
	pf.StringVar(&flagFields, "fields", "", "Comma-separated field projection (e.g. id,name)")
	pf.BoolVar(&flagYes, "yes", false, "Skip confirmation for mutating commands")
	pf.BoolVar(&flagNoPrompt, "no-prompt", false, "Never prompt interactively, even on a TTY")

	// Subcommand wiring will be added by individual files in their own init() blocks
	// or by appending to this function as we implement them.
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newProfilesCmd())
	cmd.AddCommand(newProjectsCmd())

	return cmd
}
```

This file references `newLoginCmd`, `newLogoutCmd`, `newProfilesCmd`, `newProjectsCmd` — all created in later tasks. The build will fail until they exist; we wire them in now and they get filled in incrementally.

- [ ] **Step 4: Add temporary stub functions so Task 16 compiles in isolation**

Create temporary stubs to make the build pass; they'll be replaced in Tasks 17–20.

Create `cmd/traceway/login.go`:

```go
package main

import "github.com/spf13/cobra"

func newLoginCmd() *cobra.Command { return &cobra.Command{Use: "login", Hidden: true} }
```

Create `cmd/traceway/logout.go`:

```go
package main

import "github.com/spf13/cobra"

func newLogoutCmd() *cobra.Command { return &cobra.Command{Use: "logout", Hidden: true} }
```

Create `cmd/traceway/profiles.go`:

```go
package main

import "github.com/spf13/cobra"

func newProfilesCmd() *cobra.Command { return &cobra.Command{Use: "profiles", Hidden: true} }
```

Create `cmd/traceway/projects.go`:

```go
package main

import "github.com/spf13/cobra"

func newProjectsCmd() *cobra.Command { return &cobra.Command{Use: "projects", Hidden: true} }
```

- [ ] **Step 5: Verify the build**

Run: `go build ./...`
Expected: no errors.

Run: `go run ./cmd/traceway --help`
Expected: prints usage with "CLI for the Traceway observability platform" and lists `login`, `logout`, `profiles`, `projects` (even though they're hidden, `--help` shows them) — actually, `Hidden: true` keeps them out of `--help`. That's fine for now; you won't see them until they're implemented for real.

Run: `go run ./cmd/traceway --output xml`
Expected: exit nonzero (we don't validate `--output` here yet — that's wired into individual commands in Task 17+). Acceptable.

- [ ] **Step 6: Commit**

```bash
git add cmd/traceway/ go.mod go.sum
git commit -m "feat(cmd): root command, global flags, stub subcommands"
```

---

## Task 17: cmd/traceway — login command

**Files:**
- Create: `cmd/traceway/testutil_test.go` (shared helpers — used by all subsequent command tests)
- Create: `cmd/traceway/errors.go` (handleAPIError stub for login's auth error)
- Modify: `cmd/traceway/login.go` (replace stub with real impl)
- Create: `cmd/traceway/login_test.go`

- [ ] **Step 1: Write the shared test helpers**

Create `cmd/traceway/testutil_test.go`:

```go
package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// runCmd executes the given args against a fresh root command, with stdin/out/err
// captured into buffers. It returns the buffers and the error from Execute().
//
// Each test should also call t.Setenv("XDG_CONFIG_HOME", t.TempDir()) so that
// config writes are isolated.
func runCmd(t *testing.T, stdin string, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}

	cmd := newRootCmd()
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

// readAll consumes a bytes.Buffer entirely; useful when a test needs the
// trailing bytes after Execute returned.
func readAll(t *testing.T, r io.Reader) string {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	return string(b)
}
```

- [ ] **Step 2: Write the failing tests for login**

Create `cmd/traceway/login_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/config"
)

func TestLogin_passwordStdin_success(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

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
		t.Fatal("default profile not saved")
	}
	if p.URL != srv.URL {
		t.Errorf("URL = %q", p.URL)
	}
	if p.Username != "fred@example.com" {
		t.Errorf("Username = %q", p.Username)
	}
	if p.JWT != "jwt.value" {
		t.Errorf("JWT = %q", p.JWT)
	}
	if cfg.CurrentProfile != "default" {
		t.Errorf("CurrentProfile = %q, want default (only profile should be auto-set)", cfg.CurrentProfile)
	}
}

func TestLogin_namedProfile_secondLogin_doesNotOverrideCurrent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

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

	cfg, _ := config.Load()
	if cfg.CurrentProfile != "default" {
		t.Errorf("CurrentProfile = %q, want 'default' (was set on first login, second login must not override)", cfg.CurrentProfile)
	}
	if _, ok := cfg.Profiles["cloud"]; !ok {
		t.Error("cloud profile missing")
	}
}

func TestLogin_invalidCredentials_writesEnvelope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"token":"newtoken"}`))
	}))
	defer srv.Close()

	// Seed config with an existing profile.
	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {URL: srv.URL, Username: "fred@example.com", JWT: "old"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	// Refresh: only --password-stdin, no --url/--username.
	if _, _, err := runCmd(t, "newpw\n", "login", "--password-stdin"); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	got, _ := config.Load()
	p := got.Profiles["default"]
	if p.URL != srv.URL {
		t.Errorf("URL changed: %q", p.URL)
	}
	if p.Username != "fred@example.com" {
		t.Errorf("Username changed: %q", p.Username)
	}
	if p.JWT != "newtoken" {
		t.Errorf("JWT not refreshed: %q", p.JWT)
	}
}
```

- [ ] **Step 3: Run the tests, verify they fail**

Run: `go test ./cmd/traceway/...`
Expected: build failure (login is still a stub).

- [ ] **Step 4: Create the shared `errors.go` helper**

Create `cmd/traceway/errors.go`:

```go
package main

import (
	"errors"
	"io"
	"net/url"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// renderAPIError writes the appropriate envelope to errOut and returns a
// sentinel error so the cobra runner sees a non-nil result. The actual exit
// code is communicated via the envelope's ExitCode field; main() resolves it.
//
// loginContext = true means we're in the login command itself; an Unauthorized
// from there means "wrong username/password", not "session expired".
func renderAPIError(errOut io.Writer, mode output.Mode, err error, loginContext bool) error {
	env := classifyError(err, loginContext)
	_ = output.RenderError(errOut, mode, env)
	return errors.New(env.Code) // sentinel; main() calls os.Exit with env.ExitCode
}

func classifyError(err error, loginContext bool) output.ErrorEnvelope {
	switch {
	case errors.Is(err, client.ErrUnauthorized):
		if loginContext {
			return output.ErrorEnvelope{
				Code: "not_authenticated", Message: "invalid email or password",
				ExitCode: exitcode.Auth,
			}
		}
		return output.ErrorEnvelope{
			Code: "token_expired", Message: "session expired or invalid",
			Hint:     "traceway login --profile " + flagProfile,
			ExitCode: exitcode.Auth,
		}
	case errors.Is(err, client.ErrForbidden):
		return output.ErrorEnvelope{
			Code: "forbidden", Message: "permission denied",
			ExitCode: exitcode.Auth,
		}
	case errors.Is(err, client.ErrNotFound):
		return output.ErrorEnvelope{
			Code: "not_found", Message: "resource not found",
			ExitCode: exitcode.NotFound,
		}
	case errors.Is(err, client.ErrRateLimited):
		return output.ErrorEnvelope{
			Code: "rate_limited", Message: "rate limit exceeded — slow down or retry later",
			ExitCode: exitcode.RateLimited,
		}
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode >= 500 {
			return output.ErrorEnvelope{
				Code: "server_error", Message: apiErr.Error(),
				ExitCode: exitcode.Server,
			}
		}
		return output.ErrorEnvelope{
			Code: "api_error", Message: apiErr.Error(),
			ExitCode: exitcode.Generic,
		}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return output.ErrorEnvelope{
			Code: "connection_failed", Message: urlErr.Error(),
			Hint:     "check that the Traceway URL is reachable and the network is up",
			ExitCode: exitcode.Connection,
		}
	}
	return output.ErrorEnvelope{
		Code: "internal", Message: err.Error(),
		ExitCode: exitcode.Generic,
	}
}
```

- [ ] **Step 5: Wire `main.go` to honor the envelope's exit code**

We need `main()` to know the exit code. The simplest pattern: stash the envelope in a package-level variable and read it after `Execute`.

Modify `cmd/traceway/main.go` (replace the existing body):

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
)

// lastExitCode is set by command runners that want to control os.Exit. Defaults
// to exitcode.Success on a clean run; updated by renderAPIError via the
// classified envelope.
var lastExitCode int

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := newRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		// If a runner already classified the error, lastExitCode is set.
		if lastExitCode != 0 {
			os.Exit(lastExitCode)
		}
		// Otherwise it was likely a Cobra usage error — Cobra has already
		// printed it to stderr.
		if os.Getenv("TRACEWAY_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "debug:", err)
		}
		os.Exit(exitcode.Usage)
	}
	os.Exit(exitcode.Success)
}
```

Modify `cmd/traceway/errors.go` to set `lastExitCode`:

```go
// In renderAPIError, after computing env:
func renderAPIError(errOut io.Writer, mode output.Mode, err error, loginContext bool) error {
	env := classifyError(err, loginContext)
	_ = output.RenderError(errOut, mode, env)
	lastExitCode = env.ExitCode
	return errors.New(env.Code)
}
```

- [ ] **Step 6: Implement `login.go`**

Replace `cmd/traceway/login.go` with the real implementation:

```go
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

const defaultURL = "https://cloud.tracewayapp.com"

// login-specific flag values
var (
	loginURL          string
	loginUsername     string
	loginPasswordFile bool
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against a Traceway instance and store the JWT",
		RunE:  runLogin,
	}
	cmd.Flags().StringVar(&loginURL, "url", "", "Traceway base URL (default: existing or "+defaultURL+")")
	cmd.Flags().StringVar(&loginUsername, "username", "", "Email address (default: existing or interactive prompt)")
	cmd.Flags().BoolVar(&loginPasswordFile, "password-stdin", false, "Read password from stdin instead of prompting")
	return cmd
}

func runLogin(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	profileName := flagProfile
	if profileName == "" {
		profileName = "default"
	}

	existing, hasExisting := cfg.Profiles[profileName]

	url := loginURL
	if url == "" {
		if hasExisting {
			url = existing.URL
		} else {
			url = defaultURL
		}
	}

	username := loginUsername
	if username == "" {
		if hasExisting {
			username = existing.Username
		}
		if username == "" {
			username, err = promptUsername(cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
		}
	}

	password, err := readPassword(cmd.InOrStdin(), cmd.OutOrStdout(), loginPasswordFile)
	if err != nil {
		return err
	}

	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	c := client.New(url)
	jwt, err := c.Login(ctx, username, password)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, true)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]config.Profile{}
	}
	currentProject := ""
	if hasExisting {
		currentProject = existing.CurrentProjectID
	}
	cfg.Profiles[profileName] = config.Profile{
		URL:              url,
		Username:         username,
		JWT:              jwt,
		CurrentProjectID: currentProject,
	}
	// First profile ever → set CurrentProfile pointer. Don't override on subsequent logins.
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = profileName
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s on %s (profile: %s)\n", username, url, profileName)
	return nil
}

func promptUsername(in io.Reader, out io.Writer) (string, error) {
	fmt.Fprint(out, "Username: ")
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readPassword(in io.Reader, out io.Writer, fromStdin bool) (string, error) {
	if fromStdin {
		r := bufio.NewReader(in)
		line, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	// Interactive: read with no echo if stdin is a real terminal.
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(out, "Password: ")
		bytes, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(out)
		return string(bytes), err
	}
	// Fallback: line-based read (covers test injection).
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
```

- [ ] **Step 7: Run the tests, verify they pass**

Run: `go test ./cmd/traceway/...`
Expected: PASS for all four login tests.

- [ ] **Step 8: Commit**

```bash
git add cmd/traceway/
git commit -m "feat(cmd): login command with --password-stdin and profile-aware refresh"
```

---

## Task 18: cmd/traceway — logout command

**Files:**
- Modify: `cmd/traceway/logout.go` (replace stub)
- Create: `cmd/traceway/logout_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/traceway/logout_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/config"
)

func TestLogout_removesProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {URL: "https://x", Username: "u", JWT: "tok"},
			"cloud":   {URL: "https://y", Username: "v", JWT: "tok2"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runCmd(t, "", "logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	got, _ := config.Load()
	if _, ok := got.Profiles["default"]; ok {
		t.Error("default profile should be removed")
	}
	if _, ok := got.Profiles["cloud"]; !ok {
		t.Error("cloud profile should be untouched")
	}
}

func TestLogout_resetsCurrentProfileWhenRemovingIt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {URL: "https://x"},
			"cloud":   {URL: "https://y"},
		},
	}
	_ = cfg.Save()

	_, _, err := runCmd(t, "", "logout")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := config.Load()
	if got.CurrentProfile == "default" {
		t.Errorf("CurrentProfile should not still be 'default', got %q", got.CurrentProfile)
	}
}

func TestLogout_unknownProfile_returnsAuthError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, stderr, err := runCmd(t, "", "logout", "--profile", "ghost", "--output", "json")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(stderr.String(), `"error"`) {
		t.Errorf("expected JSON envelope, got: %s", stderr.String())
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cmd/traceway/...`
Expected: tests fail (logout is still a stub that does nothing).

- [ ] **Step 3: Implement logout**

Replace `cmd/traceway/logout.go`:

```go
package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
)

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove a profile's stored credentials",
		RunE:  runLogout,
	}
}

func runLogout(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	name := flagProfile
	if name == "" {
		name = cfg.CurrentProfile
	}
	if name == "" {
		name = "default"
	}

	if _, ok := cfg.Profiles[name]; !ok {
		mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
		_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
			Code:     "no_profile",
			Message:  fmt.Sprintf("profile %q does not exist", name),
			ExitCode: exitcode.Auth,
		})
		lastExitCode = exitcode.Auth
		return errors.New("no_profile")
	}

	delete(cfg.Profiles, name)
	if cfg.CurrentProfile == name {
		// Pick any remaining profile as the new current; otherwise blank.
		cfg.CurrentProfile = ""
		for k := range cfg.Profiles {
			cfg.CurrentProfile = k
			break
		}
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Logged out of profile %q\n", name)
	return nil
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cmd/traceway/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/traceway/
git commit -m "feat(cmd): logout command — remove profile and reset current pointer"
```

---

## Task 19: cmd/traceway — `profiles list` and `profiles use`

**Files:**
- Modify: `cmd/traceway/profiles.go` (replace stub)
- Create: `cmd/traceway/profiles_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/traceway/profiles_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/config"
)

func seedTwoProfiles(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {URL: "https://a", Username: "fred@a"},
			"cloud":   {URL: "https://b", Username: "fred@b"},
		},
	}
	if err := cfg.Save(); err != nil {
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
	cfg, _ := config.Load()
	if cfg.CurrentProfile != "cloud" {
		t.Errorf("CurrentProfile = %q, want cloud", cfg.CurrentProfile)
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
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cmd/traceway/...`
Expected: failures (profiles is a stub).

- [ ] **Step 3: Implement `profiles list` and `profiles use`**

Replace `cmd/traceway/profiles.go`:

```go
package main

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
)

func newProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage stored Traceway profiles",
	}
	cmd.AddCommand(newProfilesListCmd())
	cmd.AddCommand(newProfilesUseCmd())
	return cmd
}

func newProfilesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		RunE:  runProfilesList,
	}
}

type profileSummary struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Current  bool   `json:"current"`
}

type profilesListResponse struct {
	Current string           `json:"current"`
	Data    []profileSummary `json:"data"`
}

func runProfilesList(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	names := make([]string, 0, len(cfg.Profiles))
	for n := range cfg.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)

	resp := profilesListResponse{Current: cfg.CurrentProfile}
	for _, n := range names {
		p := cfg.Profiles[n]
		resp.Data = append(resp.Data, profileSummary{
			Name:     n,
			URL:      p.URL,
			Username: p.Username,
			Current:  n == cfg.CurrentProfile,
		})
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		tw := output.NewTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(tw, " \tNAME\tURL\tUSERNAME")
		for _, p := range resp.Data {
			marker := " "
			if p.Current {
				marker = "*"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", marker, p.Name, p.URL, p.Username)
		}
		return tw.Flush()
	}
}

func newProfilesUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "Set the current profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfilesUse,
	}
}

func runProfilesUse(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	name := args[0]
	if _, ok := cfg.Profiles[name]; !ok {
		mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
		_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
			Code:     "no_profile",
			Message:  fmt.Sprintf("profile %q does not exist", name),
			Hint:     "traceway profiles list",
			ExitCode: exitcode.Auth,
		})
		lastExitCode = exitcode.Auth
		return errors.New("no_profile")
	}
	cfg.CurrentProfile = name
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Now using profile %q\n", name)
	return nil
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cmd/traceway/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/traceway/
git commit -m "feat(cmd): profiles list and profiles use"
```

---

## Task 20: cmd/traceway — `projects list` and `projects use`

**Files:**
- Modify: `cmd/traceway/projects.go` (replace stub)
- Create: `cmd/traceway/projects_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/traceway/projects_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/config"
)

func seedProfileFor(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {URL: baseURL, Username: "fred@example.com", JWT: "tok"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectsList_jsonPassThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"p1","name":"alpha"},
				{"id":"p2","name":"beta"}
			],
			"pagination":{"page":0,"pageSize":50,"total":2}
		}`))
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
	if !strings.Contains(out, `"pagination"`) {
		t.Errorf("expected pagination passthrough, got: %s", out)
	}
}

func TestProjectsList_table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"p1","name":"alpha"}],"pagination":{"total":1}}`))
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

	_, stderr, err := runCmd(t, "", "projects", "list", "--output", "json", "--no-prompt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), `"token_expired"`) {
		t.Errorf("expected token_expired envelope, got: %s", stderr.String())
	}
}

func TestProjectsList_noProfile_writesEnvelope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

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
	cfg, _ := config.Load()
	if cfg.Profiles["default"].CurrentProjectID != "p1" {
		t.Errorf("CurrentProjectID = %q", cfg.Profiles["default"].CurrentProjectID)
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cmd/traceway/...`
Expected: failures.

- [ ] **Step 3: Implement projects**

Replace `cmd/traceway/projects.go`:

```go
package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List and switch projects",
	}
	cmd.AddCommand(newProjectsListCmd())
	cmd.AddCommand(newProjectsUseCmd())
	return cmd
}

func newProjectsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List projects visible to the authenticated user",
		RunE:  runProjectsList,
	}
}

func runProjectsList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	profile, err := cfg.Active(flagProfile)
	if err != nil {
		_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
			Code:     "not_authenticated",
			Message:  err.Error(),
			Hint:     "traceway login",
			ExitCode: exitcode.Auth,
		})
		lastExitCode = exitcode.Auth
		return errors.New("not_authenticated")
	}

	c := client.New(profile.URL, client.WithJWT(profile.JWT))
	resp, err := c.ListProjects(ctx)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}

	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), resp, output.ParseFieldsFlag(flagFields))
	default:
		tw := output.NewTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(tw, "ID\tNAME")
		for _, p := range resp.Data {
			fmt.Fprintf(tw, "%s\t%s\n", p.ID, p.Name)
		}
		return tw.Flush()
	}
}

func newProjectsUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <project-id>",
		Short: "Set the current project for the active profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectsUse,
	}
}

func runProjectsUse(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	profileName := flagProfile
	if profileName == "" {
		profileName = cfg.CurrentProfile
	}
	if profileName == "" {
		profileName = "default"
	}
	p, ok := cfg.Profiles[profileName]
	if !ok {
		mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
		_ = output.RenderError(cmd.ErrOrStderr(), mode, output.ErrorEnvelope{
			Code:     "no_profile",
			Message:  fmt.Sprintf("profile %q does not exist", profileName),
			Hint:     "traceway login",
			ExitCode: exitcode.Auth,
		})
		lastExitCode = exitcode.Auth
		return errors.New("no_profile")
	}
	p.CurrentProjectID = args[0]
	cfg.Profiles[profileName] = p
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Now using project %q for profile %q\n", args[0], profileName)
	return nil
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cmd/traceway/...`
Expected: PASS.

- [ ] **Step 5: End-to-end smoke against stormwind (optional but recommended)**

Run against your real Traceway instance — replace placeholders with your stormwind URL and credentials:

```bash
go run ./cmd/traceway login --url https://traceway.stormwind.local --username fred@example.com
# (enter password at prompt)

go run ./cmd/traceway projects list
go run ./cmd/traceway projects list --output json
go run ./cmd/traceway projects list --output yaml
go run ./cmd/traceway projects list --output json --fields id,name

go run ./cmd/traceway profiles list
```

Expected: each command exits 0 and shows the projects from your stormwind. If `login` fails with a different field name in the response, fix `loginResponse.Token`'s json tag in `pkg/client/auth.go` (see Task 9).

- [ ] **Step 6: Commit**

```bash
git add cmd/traceway/
git commit -m "feat(cmd): projects list and projects use"
```

---

## Self-Review (already performed during writing)

**Spec coverage check:**

| Spec section | Implementing tasks |
|---|---|
| Audience priority (LLM-first, humans at home) | Task 11 (auto-detect TTY), Task 15 (envelope shapes) |
| Profile model + `--profile default` | Tasks 4–6, 17–19 |
| Project model + `--project` resolution | Task 6 (resolution helpers), Task 20 (per-profile current) |
| `--output json/yaml/table` + auto-detect | Tasks 11–14 |
| `--fields` projection | Task 12 |
| Pagination (one HTTP call per command) | Task 10 (Pagination type) |
| Mutations confirmation gate | **Deferred to Plan 3** (no mutations in Plan 1) |
| Error envelope (JSON + prose) | Task 15, Task 17 (error helper) |
| Exit codes | Task 2, Task 17 (lastExitCode wiring) |
| Token expiry retry | **Partially in Plan 1** (envelope-only path); interactive retry deferred to Plan 2 first-use |
| Atomic config writes, 0600/0700 | Task 5 |
| Library-first (`pkg/client`) | Tasks 7–10 (no CLI imports) |
| Smoke harness | **Deferred to Plan 3** |
| `justfile` + `just` in dev shell | Task 1 |

Two intentional Plan-1 deferrals:
- **Mutations + confirmation gate** — no mutations exist yet (no `--yes` workflow to test).
- **Interactive token-expiry re-login retry** — implemented as envelope-only in Plan 1; the interactive prompt path lands when the first read-resource needs it (Plan 2).

**Placeholder scan:** No "TBD"/"TODO" in steps. The two endpoint-shape "confirm against upstream" notes (Tasks 9, 10) are explicit research items, not placeholders.

**Type consistency:** Spot-checked across tasks. `Config`, `Profile`, `Client`, `Project`, `Pagination`, `ListProjectsResponse`, `Mode`, `ErrorEnvelope`, `lastExitCode` — all named consistently across the tasks that reference them. `flagProfile`/`flagProject`/`flagOutput`/`flagFields`/`flagYes`/`flagNoPrompt` declared once in Task 16, used by name in Tasks 17–20.

---

## End-of-plan checkpoint

After Task 20, you should be able to:

```bash
just test          # all tests pass
just lint          # no lint findings
just vulncheck     # no vulns
go run ./cmd/traceway login --url <your URL> --username <you>
go run ./cmd/traceway projects list
go run ./cmd/traceway profiles list
```

End state: working CLI with auth, profiles, project switching, and three output formats. All architectural patterns (client, config, output, errors, profiles, exit codes) are in place. Plan 2 builds on this for the read resources (exceptions, logs, endpoints, metrics).
