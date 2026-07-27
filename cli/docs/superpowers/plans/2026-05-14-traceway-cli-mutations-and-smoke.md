# traceway-cli Mutations + Smoke Harness + README Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close out v1 of the CLI by adding the only mutating commands the spec calls for (`exceptions archive` / `exceptions unarchive`), plus a confirmation gate that's safe for both humans and LLMs, a build-tag-gated smoke test suite that exercises the full stack against a real Traceway instance, and a README so a new caller (human or agent) can pick the tool up cold.

**Architecture:** Same library-first split established in Plan 1 — mutation methods live on `pkg/client.Client` with no CLI concerns; subcommands in `cmd/traceway/exceptions.go` parse args, call a shared `confirmMutation()` helper for safety, then call the client. Smoke tests live under `test/smoke/` behind a `//go:build smoke` tag so the default `go test ./...` skips them; they read connection/credential info from environment variables and round-trip through the public CLI surface. README is a single top-level `README.md`.

**Tech Stack:** Go (stdlib `net/http`, `os`, `bufio`), Cobra, existing `internal/{config,state,output,exitcode}` packages, existing `pkg/client.Client`/`do()` infrastructure.

---

## Critical context: upstream API shape (verified 2026-05-14)

The brainstorm spec mentioned "archive + resolve" but `tracewayapp/traceway/backend/app/routes.go` actually exposes:

```
POST /api/exception-stack-traces/archive      (RequireProjectAccess + RequireWriteAccess)
POST /api/exception-stack-traces/unarchive    (RequireProjectAccess + RequireWriteAccess)
```

There is **no** `resolve` route upstream. Plan 3 implements `archive` + `unarchive` to match reality. (The ListExceptions request already exposes `IncludeArchived`; toggling that is how callers see archived items.)

Both routes accept the same JSON body shape, with `projectId` as a URL **query param** (not a body field) — same convention all the other endpoints use:

```go
type ArchiveRequest struct {
    Hashes []string `json:"hashes"`
}
```

Response on success: `{"success": true}` (just a status confirmation; we don't expose the struct, we surface the count and the hashes the user passed).

---

## Confirmation gate semantics

These are the only mutating commands in v1, but the gate must work the same way for any mutation we add later, so we implement it as a reusable helper.

**Resolution order** (first match wins):

1. `--yes` flag is set → proceed silently.
2. `TRACEWAY_ASSUME_YES=1` (or `=true`) environment variable → proceed silently. (Lets CI / wrapper scripts opt in once instead of threading `--yes` through every invocation.)
3. stdin is a TTY → print the summary, prompt `Continue? [y/N] `, read one line, accept only `y` or `yes` (case-insensitive). Anything else → `usage_error` envelope, exit 2.
4. stdin is not a TTY and neither opt-in is set → render `usage_error` immediately with hint `pass --yes or set TRACEWAY_ASSUME_YES=1`, exit 2. (LLMs invoking the CLI via Bash get a clear, machine-readable refusal instead of a hung prompt.)

**What the summary looks like:**

```
About to archive 3 exception group(s):
  - 5f8a3b1c9d2e
  - a1b2c3d4e5f6
  - 0123456789ab
Continue? [y/N]
```

The verb (`archive` vs `unarchive`) is the only thing that changes between the two commands. Hashes longer than 12 chars are truncated for the prompt, full hashes still go on the wire.

**Why an env var as well as a flag:** matches `gh`'s pattern for non-interactive defaults; some callers (cron, CI) find an env var cleaner than threading a flag through wrappers. The flag still wins if both are set.

---

## Smoke harness shape

`test/smoke/` is a separate Go package gated behind `//go:build smoke` so:

- `go test ./...` (and `just test`) runs zero smoke tests.
- `go test -tags smoke ./test/smoke/...` (and `just smoke-test`) runs them all.

Each test reads required env vars at the top of the file via a single `requireEnv(t)` helper that calls `t.Skip(...)` (not `t.Fatal`) when any are missing. That way running `just smoke-test` without the env set produces clean SKIP output, not failures.

**Required env vars:**

- `TRACEWAY_SMOKE_URL` — e.g. `https://traceway.stormwind.local`
- `TRACEWAY_SMOKE_USERNAME` — email for login
- `TRACEWAY_SMOKE_PASSWORD` — password for login
- `TRACEWAY_SMOKE_PROJECT_ID` — UUID of the project to query against

Each test gets its own fresh `XDG_CONFIG_HOME` and `XDG_STATE_HOME` (via `t.TempDir()` + `t.Setenv`) so the user's actual credentials are never touched and tests can't leak state into each other. Tests shell out to the locally built binary (`go build -o $TMP/traceway ./cmd/traceway` once at TestMain), invoke it with `--output json`, and assert on the parsed envelope/response.

Coverage is intentionally narrow — one happy-path round trip per command, not exhaustive. The point is "does the wire shape we coded against match what real Traceway returns?", not regression coverage.

---

## File Map

```
pkg/client/
├── exceptions.go              (modify)  — add ArchiveExceptions, UnarchiveExceptions
└── exceptions_test.go         (modify)  — add tests for both

cmd/traceway/
├── querycommon.go             (modify)  — add confirmMutation() helper
├── querycommon_test.go        (create)  — confirmMutation tests
├── exceptions.go              (modify)  — wire up archive + unarchive subcommands
└── exceptions_test.go         (modify)  — add tests for both subcommands

test/smoke/                    (create directory)
├── doc.go                     (create)  — package comment + build tag
├── harness_test.go            (create)  — TestMain (build binary), runCLI helper, requireEnv
├── login_test.go              (create)  — login round trip
├── projects_test.go           (create)  — projects list round trip
├── exceptions_test.go         (create)  — exceptions list + archive/unarchive round trip
├── endpoints_test.go          (create)  — endpoints list round trip
└── logs_test.go               (create)  — logs query round trip

README.md                      (create)  — top-level README
```

**File responsibilities (new):**

- **`pkg/client/exceptions.go` additions** — `ArchiveExceptions(ctx, projectID, hashes)` and `UnarchiveExceptions(ctx, projectID, hashes)`. Both POST to their respective `/api/exception-stack-traces/{archive,unarchive}?projectId=...` paths with `{"hashes":[...]}` body. Return `error` only (no useful response payload to surface).
- **`cmd/traceway/querycommon.go` addition** — `confirmMutation(cmd *cobra.Command, summaryLines []string) error` implementing the resolution order above. Stdin/TTY detection uses the same approach `output.StdoutIsTerminal` uses for stdout (peek at `*os.File` + `term.IsTerminal`). Returns nil on approval, `*cliError(usage)` on refusal/non-interactive without opt-in.
- **`cmd/traceway/exceptions.go` additions** — `newExceptionsArchiveCmd()` and `newExceptionsUnarchiveCmd()` registered onto the existing `exceptions` parent. Each takes `cobra.MinimumNArgs(1)` of hashes. The two share a single `runMutation` helper parameterised by verb + client method to avoid duplicating the boilerplate (session load, summary build, confirmation, client call, success render).
- **`test/smoke/harness_test.go`** — `TestMain` does one `go build` into a temp dir, exposes the binary path via a package-level var. `runCLI(t, args...)` returns `(stdout, stderr, exitCode)`. `requireEnv(t)` skips with a clear message if any var is missing. `freshXDG(t)` returns config + state dirs and sets the env vars with `t.Setenv`.
- **`README.md`** — install (Nix dev shell + `just build`), per-command usage examples, profiles + projects guide, output formats with field projection, error envelope reference table, smoke testing section.

---

## Task 1: pkg/client — ArchiveExceptions

**Files:**
- Modify: `pkg/client/exceptions.go`
- Modify: `pkg/client/exceptions_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `pkg/client/exceptions_test.go`:

```go
func TestArchiveExceptions_postsHashesAndProjectId(t *testing.T) {
	var gotPath string
	var gotProject string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		gotPath = r.URL.Path
		gotProject = r.URL.Query().Get("projectId")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	err := c.ArchiveExceptions(context.Background(), "proj-1", []string{"h1", "h2", "h3"})
	if err != nil {
		t.Fatalf("ArchiveExceptions: %v", err)
	}
	if gotPath != "/api/exception-stack-traces/archive" {
		t.Errorf("path = %q", gotPath)
	}
	if gotProject != "proj-1" {
		t.Errorf("projectId = %q", gotProject)
	}
	hashes, _ := gotBody["hashes"].([]any)
	if len(hashes) != 3 || hashes[0] != "h1" || hashes[2] != "h3" {
		t.Errorf("body hashes = %v", hashes)
	}
}

func TestArchiveExceptions_403_returnsErrForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	err := c.ArchiveExceptions(context.Background(), "proj-1", []string{"h1"})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `nix develop --command go test ./pkg/client/... -run ArchiveExceptions -count=1`
Expected: build failure — `ArchiveExceptions` undefined.

- [ ] **Step 3: Implement ArchiveExceptions**

Append to `pkg/client/exceptions.go`:

```go
// archiveRequest is the body for POST /api/exception-stack-traces/archive
// and .../unarchive. Same shape for both routes.
type archiveRequest struct {
	Hashes []string `json:"hashes"`
}

// ArchiveExceptions marks the given exception hashes as archived for the
// project. The upstream response is just {"success": true}; on success this
// returns nil and the caller can report the count back to the user.
func (c *Client) ArchiveExceptions(ctx context.Context, projectID string, hashes []string) error {
	path := "/api/exception-stack-traces/archive?projectId=" + url.QueryEscape(projectID)
	return c.do(ctx, http.MethodPost, path, archiveRequest{Hashes: hashes}, nil)
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `nix develop --command go test ./pkg/client/... -run ArchiveExceptions -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/client/exceptions.go pkg/client/exceptions_test.go
git commit -m "feat(client): ArchiveExceptions"
```

---

## Task 2: pkg/client — UnarchiveExceptions

**Files:**
- Modify: `pkg/client/exceptions.go`
- Modify: `pkg/client/exceptions_test.go`

- [ ] **Step 1: Write the failing test**

Append to `pkg/client/exceptions_test.go`:

```go
func TestUnarchiveExceptions_postsHashesAndProjectId(t *testing.T) {
	var gotPath string
	var gotProject string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotProject = r.URL.Query().Get("projectId")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	err := c.UnarchiveExceptions(context.Background(), "proj-1", []string{"h1"})
	if err != nil {
		t.Fatalf("UnarchiveExceptions: %v", err)
	}
	if gotPath != "/api/exception-stack-traces/unarchive" {
		t.Errorf("path = %q", gotPath)
	}
	if gotProject != "proj-1" {
		t.Errorf("projectId = %q", gotProject)
	}
	hashes, _ := gotBody["hashes"].([]any)
	if len(hashes) != 1 || hashes[0] != "h1" {
		t.Errorf("body hashes = %v", hashes)
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `nix develop --command go test ./pkg/client/... -run UnarchiveExceptions -count=1`
Expected: build failure — `UnarchiveExceptions` undefined.

- [ ] **Step 3: Implement UnarchiveExceptions**

Append to `pkg/client/exceptions.go`:

```go
// UnarchiveExceptions reverses ArchiveExceptions for the given hashes.
func (c *Client) UnarchiveExceptions(ctx context.Context, projectID string, hashes []string) error {
	path := "/api/exception-stack-traces/unarchive?projectId=" + url.QueryEscape(projectID)
	return c.do(ctx, http.MethodPost, path, archiveRequest{Hashes: hashes}, nil)
}
```

(Reuses the unexported `archiveRequest` type from Task 1.)

- [ ] **Step 4: Run the test, verify it passes**

Run: `nix develop --command go test ./pkg/client/... -run UnarchiveExceptions -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/client/exceptions.go pkg/client/exceptions_test.go
git commit -m "feat(client): UnarchiveExceptions"
```

---

## Task 3: cmd/traceway — confirmMutation helper

**Files:**
- Modify: `cmd/traceway/querycommon.go`
- Create: `cmd/traceway/querycommon_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/traceway/querycommon_test.go`:

```go
package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
)

// newTestCmd returns a minimal cobra command pre-wired with the global flags
// confirmMutation cares about (--yes is the only one).
func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().BoolVar(&flagYes, "yes", false, "")
	return cmd
}

func TestConfirmMutation_yesFlagBypassesPrompt(t *testing.T) {
	t.Cleanup(func() { flagYes = false })
	flagYes = true

	cmd := newTestCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetIn(strings.NewReader("")) // no input — must not be consulted

	if err := confirmMutation(cmd, []string{"about to do thing"}); err != nil {
		t.Fatalf("confirmMutation = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output with --yes, got %q", out.String())
	}
}

func TestConfirmMutation_envVarBypassesPrompt(t *testing.T) {
	t.Setenv("TRACEWAY_ASSUME_YES", "1")
	cmd := newTestCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetIn(strings.NewReader(""))

	if err := confirmMutation(cmd, []string{"summary"}); err != nil {
		t.Fatalf("confirmMutation = %v, want nil", err)
	}
}

func TestConfirmMutation_nonTTYWithoutYes_returnsUsageError(t *testing.T) {
	cmd := newTestCmd()
	stderr := &bytes.Buffer{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(stderr)
	cmd.SetIn(strings.NewReader("")) // io.Reader, not *os.File → not a TTY

	err := confirmMutation(cmd, []string{"summary"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *cliError, got %T", err)
	}
	if ce.code != exitcode.Usage {
		t.Errorf("exit code = %d, want %d", ce.code, exitcode.Usage)
	}
	if !strings.Contains(stderr.String(), "usage_error") {
		t.Errorf("stderr should contain usage_error envelope, got %q", stderr.String())
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `nix develop --command go test ./cmd/traceway/... -run ConfirmMutation -count=1`
Expected: build failure — `confirmMutation` undefined.

- [ ] **Step 3: Implement confirmMutation**

Append to `cmd/traceway/querycommon.go`:

```go
// confirmMutation gates a destructive action behind one of three approvals,
// in priority order:
//
//  1. --yes flag is set
//  2. TRACEWAY_ASSUME_YES env var is set to a truthy value (1, true, yes)
//  3. stdin is a TTY → print summary + prompt, accept y/yes
//
// If none apply (non-TTY caller without an opt-in), a usage_error envelope
// is rendered to errOut and a *cliError(Usage) is returned. Callers should
// return that error directly so main() exits 2.
func confirmMutation(cmd *cobra.Command, summaryLines []string) error {
	if flagYes {
		return nil
	}
	if assume := strings.ToLower(strings.TrimSpace(os.Getenv("TRACEWAY_ASSUME_YES"))); assume == "1" || assume == "true" || assume == "yes" {
		return nil
	}

	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	in := cmd.InOrStdin()
	f, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"refusing to perform mutation without confirmation",
			"pass --yes or set TRACEWAY_ASSUME_YES=1")
	}

	out := cmd.OutOrStdout()
	for _, line := range summaryLines {
		_, _ = fmt.Fprintln(out, line)
	}
	_, _ = fmt.Fprint(out, "Continue? [y/N] ")

	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"failed to read confirmation: "+err.Error(),
			"pass --yes to skip the prompt")
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"confirmation declined", "")
	}
	return nil
}
```

Add the new imports to the import block (currently the file imports just `"io"`, `cobra`, `exitcode`, `output`, `client`):

```go
import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `nix develop --command go test ./cmd/traceway/... -run ConfirmMutation -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full test suite to confirm no regressions**

Run: `nix develop --command go test ./... -count=1`
Expected: PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add cmd/traceway/querycommon.go cmd/traceway/querycommon_test.go
git commit -m "feat(cli): confirmMutation gate (--yes, env, TTY prompt)"
```

---

## Task 4: cmd/traceway — exceptions archive subcommand

**Files:**
- Modify: `cmd/traceway/exceptions.go`
- Modify: `cmd/traceway/exceptions_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `cmd/traceway/exceptions_test.go`:

```go
func TestExceptionsArchive_yes_callsClientAndRendersJSON(t *testing.T) {
	var gotHashes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/exception-stack-traces/archive" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, h := range body["hashes"].([]any) {
			gotHashes = append(gotHashes, h.(string))
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	withTestSession(t, srv.URL, "proj-1")
	stdout, stderr, err := runCmd(t, "exceptions", "archive", "--yes", "--output", "json", "h1", "h2")
	if err != nil {
		t.Fatalf("runCmd: %v\nstderr: %s", err, stderr)
	}
	if len(gotHashes) != 2 || gotHashes[0] != "h1" || gotHashes[1] != "h2" {
		t.Errorf("server got hashes %v", gotHashes)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	if resp["action"] != "archive" {
		t.Errorf("action = %v", resp["action"])
	}
	if resp["count"].(float64) != 2 {
		t.Errorf("count = %v", resp["count"])
	}
}

func TestExceptionsArchive_nonTTYWithoutYes_failsWithUsageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called when confirmation refuses")
	}))
	defer srv.Close()

	withTestSession(t, srv.URL, "proj-1")
	stdout, stderr, err := runCmd(t, "exceptions", "archive", "--output", "json", "h1")
	if err == nil {
		t.Fatalf("expected error, got success: stdout=%s", stdout)
	}
	if !strings.Contains(stderr, "usage_error") {
		t.Errorf("stderr should contain usage_error: %s", stderr)
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitcode.Usage {
		t.Errorf("expected cliError(Usage), got %v", err)
	}
}

func TestExceptionsArchive_requiresAtLeastOneHash(t *testing.T) {
	withTestSession(t, "https://unused", "proj-1")
	_, _, err := runCmd(t, "exceptions", "archive", "--yes")
	if err == nil {
		t.Fatal("expected error from missing args")
	}
}
```

(Helpers `runCmd` and `withTestSession` already exist in `cmd/traceway/testutil_test.go` from Plan 1/2 — confirm before writing this task that they accept the signature shown here. If `withTestSession` does not yet exist, factor it out from existing tests rather than writing a new one inline.)

- [ ] **Step 2: Run the tests, verify they fail**

Run: `nix develop --command go test ./cmd/traceway/... -run ExceptionsArchive -count=1`
Expected: build failure or test failure — `archive` subcommand not registered.

- [ ] **Step 3: Implement the archive subcommand**

Append to `cmd/traceway/exceptions.go`. First, extend `newExceptionsCmd` to register the new subcommands:

```go
func newExceptionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exceptions",
		Short: "Query exception groups and occurrences",
	}
	cmd.AddCommand(newExceptionsListCmd())
	cmd.AddCommand(newExceptionsShowCmd())
	cmd.AddCommand(newExceptionsArchiveCmd())
	cmd.AddCommand(newExceptionsUnarchiveCmd())
	return cmd
}
```

Then add the archive command and the shared mutation runner:

```go
func newExceptionsArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <hash> [<hash>...]",
		Short: "Archive one or more exception groups",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExceptionsMutation(cmd, args, "archive",
				func(c *client.Client, ctx context.Context, projectID string, hashes []string) error {
					return c.ArchiveExceptions(ctx, projectID, hashes)
				})
		},
	}
}

// runExceptionsMutation is the shared body for archive and unarchive. The
// 'verb' parameter controls the prompt wording and the rendered action label;
// 'doIt' is the client method to call after confirmation passes.
func runExceptionsMutation(
	cmd *cobra.Command,
	hashes []string,
	verb string,
	doIt func(c *client.Client, ctx context.Context, projectID string, hashes []string) error,
) error {
	ctx := cmd.Context()
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	sess, err := loadSession()
	if err != nil {
		return renderSessionError(cmd.ErrOrStderr(), mode, err)
	}

	summary := []string{
		fmt.Sprintf("About to %s %d exception group(s):", verb, len(hashes)),
	}
	for _, h := range hashes {
		summary = append(summary, "  - "+truncateHash(h, 12))
	}
	if err := confirmMutation(cmd, summary); err != nil {
		return err
	}

	c := client.New(sess.URL, client.WithJWT(sess.JWT))
	if err := doIt(c, ctx, sess.ProjectID, hashes); err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, false)
	}

	result := map[string]any{
		"action": verb,
		"count":  len(hashes),
		"hashes": hashes,
	}
	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(cmd.OutOrStdout(), result, nil)
	case output.ModeYAML:
		return output.RenderYAML(cmd.OutOrStdout(), result, nil)
	default:
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%sd %d exception group(s).\n", verb, len(hashes))
		return err
	}
}

// truncateHash returns hash, or its first n chars if longer. Used for
// human-readable summaries; the full hash always goes to the API.
func truncateHash(hash string, n int) string {
	if len(hash) <= n {
		return hash
	}
	return hash[:n]
}
```

Add `"context"` to the imports if it isn't already there.

- [ ] **Step 4: Run the tests, verify they pass**

Run: `nix develop --command go test ./cmd/traceway/... -run ExceptionsArchive -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/traceway/exceptions.go cmd/traceway/exceptions_test.go
git commit -m "feat(cli): exceptions archive"
```

---

## Task 5: cmd/traceway — exceptions unarchive subcommand

**Files:**
- Modify: `cmd/traceway/exceptions.go`
- Modify: `cmd/traceway/exceptions_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/traceway/exceptions_test.go`:

```go
func TestExceptionsUnarchive_yes_callsClientAndRendersJSON(t *testing.T) {
	var gotPath string
	var gotHashes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, h := range body["hashes"].([]any) {
			gotHashes = append(gotHashes, h.(string))
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	withTestSession(t, srv.URL, "proj-1")
	stdout, stderr, err := runCmd(t, "exceptions", "unarchive", "--yes", "--output", "json", "h1")
	if err != nil {
		t.Fatalf("runCmd: %v\nstderr: %s", err, stderr)
	}
	if gotPath != "/api/exception-stack-traces/unarchive" {
		t.Errorf("path = %q", gotPath)
	}
	if len(gotHashes) != 1 || gotHashes[0] != "h1" {
		t.Errorf("server got hashes %v", gotHashes)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	if resp["action"] != "unarchive" {
		t.Errorf("action = %v", resp["action"])
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `nix develop --command go test ./cmd/traceway/... -run ExceptionsUnarchive -count=1`
Expected: build failure — `unarchive` subcommand not registered.

- [ ] **Step 3: Implement the unarchive subcommand**

Append to `cmd/traceway/exceptions.go`:

```go
func newExceptionsUnarchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unarchive <hash> [<hash>...]",
		Short: "Unarchive one or more exception groups",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExceptionsMutation(cmd, args, "unarchive",
				func(c *client.Client, ctx context.Context, projectID string, hashes []string) error {
					return c.UnarchiveExceptions(ctx, projectID, hashes)
				})
		},
	}
}
```

(`newExceptionsCmd` was already updated in Task 4 to call `AddCommand(newExceptionsUnarchiveCmd())`.)

- [ ] **Step 4: Run the test, verify it passes**

Run: `nix develop --command go test ./cmd/traceway/... -run ExceptionsUnarchive -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full test suite**

Run: `nix develop --command go test ./... -count=1`
Expected: PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add cmd/traceway/exceptions.go cmd/traceway/exceptions_test.go
git commit -m "feat(cli): exceptions unarchive"
```

---

## Task 6: test/smoke — harness scaffolding + login round trip

**Files:**
- Create: `test/smoke/doc.go`
- Create: `test/smoke/harness_test.go`
- Create: `test/smoke/login_test.go`

- [ ] **Step 1: Create the package doc with the build tag**

Create `test/smoke/doc.go`:

```go
//go:build smoke

// Package smoke contains end-to-end tests that talk to a real Traceway
// instance via the built CLI binary. They are gated behind the "smoke"
// build tag so `go test ./...` skips them entirely.
//
// Run with: go test -tags smoke ./test/smoke/... (or `just smoke-test`).
//
// Required env vars (tests are skipped, not failed, if any are missing):
//
//   TRACEWAY_SMOKE_URL         e.g. https://traceway.stormwind.local
//   TRACEWAY_SMOKE_USERNAME    email used to log in
//   TRACEWAY_SMOKE_PASSWORD    password
//   TRACEWAY_SMOKE_PROJECT_ID  UUID of a project the user can access
package smoke
```

- [ ] **Step 2: Write the harness**

Create `test/smoke/harness_test.go`:

```go
//go:build smoke

package smoke

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "traceway-smoke-bin-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "traceway")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/traceway")
	cmd.Dir = repoRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("smoke harness: go build failed: " + err.Error() + "\n" + string(out))
	}
	os.Exit(m.Run())
}

// repoRoot walks up from the current working dir to find go.mod.
func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for d := wd; d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	panic("smoke harness: go.mod not found above " + wd)
}

// requireEnv reads the four smoke env vars and skips the test cleanly if
// any are missing. Returns (url, username, password, projectID).
func requireEnv(t *testing.T) (string, string, string, string) {
	t.Helper()
	url := os.Getenv("TRACEWAY_SMOKE_URL")
	user := os.Getenv("TRACEWAY_SMOKE_USERNAME")
	pass := os.Getenv("TRACEWAY_SMOKE_PASSWORD")
	proj := os.Getenv("TRACEWAY_SMOKE_PROJECT_ID")
	if url == "" || user == "" || pass == "" || proj == "" {
		t.Skip("smoke env not set (TRACEWAY_SMOKE_URL/USERNAME/PASSWORD/PROJECT_ID)")
	}
	return url, user, pass, proj
}

// freshXDG isolates each test by pointing XDG_CONFIG_HOME and XDG_STATE_HOME
// at fresh temp dirs. Cleanup is handled by t.TempDir.
func freshXDG(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

// runCLI invokes the built binary with the given args. stdinBody is fed
// to the process stdin; pass "" for none. Returns stdout, stderr, exit code.
func runCLI(t *testing.T, stdinBody string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = os.Environ() // inherit XDG_* and any other vars
	if stdinBody != "" {
		cmd.Stdin = strings.NewReader(stdinBody)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("runCLI: failed to run %v: %v", args, err)
		}
	}
	return stdout.String(), stderr.String(), code
}
```

- [ ] **Step 3: Write the login test**

Create `test/smoke/login_test.go`:

```go
//go:build smoke

package smoke

import (
	"strings"
	"testing"
)

// TestSmokeLogin authenticates against the real instance and verifies the
// resulting session works for one follow-up call. This is the prerequisite
// every other smoke test depends on.
func TestSmokeLogin(t *testing.T) {
	url, user, pass, _ := requireEnv(t)
	freshXDG(t)

	stdout, stderr, code := runCLI(t,
		pass+"\n",
		"login", "--url", url, "--username", user, "--password-stdin",
	)
	if code != 0 {
		t.Fatalf("login exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Logged in as "+user) {
		t.Errorf("login stdout = %q", stdout)
	}
}
```

- [ ] **Step 4: Verify the package compiles under the smoke tag**

Run: `nix develop --command go vet -tags smoke ./test/smoke/...`
Expected: no errors. (Tests aren't run yet — `go vet` is enough to catch typos.)

- [ ] **Step 5: Verify the default test run skips smoke tests**

Run: `nix develop --command go test ./... -count=1`
Expected: PASS, no `test/smoke` package reported (tag-gated out).

- [ ] **Step 6: Commit**

```bash
git add test/smoke/doc.go test/smoke/harness_test.go test/smoke/login_test.go
git commit -m "test(smoke): build-tag-gated harness with login round trip"
```

---

## Task 7: test/smoke — projects / exceptions / endpoints / logs round trips

**Files:**
- Create: `test/smoke/projects_test.go`
- Create: `test/smoke/exceptions_test.go`
- Create: `test/smoke/endpoints_test.go`
- Create: `test/smoke/logs_test.go`

Each test reuses the harness from Task 6: `requireEnv`, `freshXDG`, `runCLI`. They share a small pattern: log in, set the project, run the resource command, parse JSON, assert the wrapper shape.

- [ ] **Step 1: Write the projects test**

Create `test/smoke/projects_test.go`:

```go
//go:build smoke

package smoke

import (
	"encoding/json"
	"testing"
)

func TestSmokeProjectsList(t *testing.T) {
	url, user, pass, _ := requireEnv(t)
	freshXDG(t)

	if _, _, code := runCLI(t, pass+"\n", "login", "--url", url, "--username", user, "--password-stdin"); code != 0 {
		t.Fatal("login failed (see TestSmokeLogin)")
	}

	stdout, stderr, code := runCLI(t, "", "projects", "list", "--output", "json")
	if code != 0 {
		t.Fatalf("projects list exit %d\nstderr: %s", code, stderr)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatalf("projects list stdout not a JSON array: %v\n%s", err, stdout)
	}
	if len(arr) == 0 {
		t.Fatal("projects list returned 0 projects; smoke account should have at least one")
	}
	if _, ok := arr[0]["id"]; !ok {
		t.Errorf("first project missing 'id' field: %v", arr[0])
	}
}
```

- [ ] **Step 2: Write the exceptions test**

Create `test/smoke/exceptions_test.go`:

```go
//go:build smoke

package smoke

import (
	"encoding/json"
	"testing"
)

func TestSmokeExceptionsList(t *testing.T) {
	url, user, pass, proj := requireEnv(t)
	freshXDG(t)

	if _, _, code := runCLI(t, pass+"\n", "login", "--url", url, "--username", user, "--password-stdin"); code != 0 {
		t.Fatal("login failed")
	}
	if _, _, code := runCLI(t, "", "projects", "use", proj); code != 0 {
		t.Fatalf("projects use %s failed", proj)
	}

	stdout, stderr, code := runCLI(t, "", "exceptions", "list", "--since", "24h", "--output", "json")
	if code != 0 {
		t.Fatalf("exceptions list exit %d\nstderr: %s", code, stderr)
	}
	var resp struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]any   `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("exceptions list stdout not JSON: %v\n%s", err, stdout)
	}
	if resp.Pagination == nil {
		t.Errorf("exceptions list response missing pagination wrapper: %s", stdout)
	}
}
```

- [ ] **Step 3: Write the endpoints test**

Create `test/smoke/endpoints_test.go`:

```go
//go:build smoke

package smoke

import (
	"encoding/json"
	"testing"
)

func TestSmokeEndpointsList(t *testing.T) {
	url, user, pass, proj := requireEnv(t)
	freshXDG(t)

	if _, _, code := runCLI(t, pass+"\n", "login", "--url", url, "--username", user, "--password-stdin"); code != 0 {
		t.Fatal("login failed")
	}
	if _, _, code := runCLI(t, "", "projects", "use", proj); code != 0 {
		t.Fatalf("projects use %s failed", proj)
	}

	stdout, stderr, code := runCLI(t, "", "endpoints", "list", "--since", "24h", "--output", "json")
	if code != 0 {
		t.Fatalf("endpoints list exit %d\nstderr: %s", code, stderr)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("endpoints list stdout not JSON: %v\n%s", err, stdout)
	}
	if _, ok := resp["data"]; !ok {
		t.Errorf("endpoints list response missing 'data' field: %s", stdout)
	}
}
```

- [ ] **Step 4: Write the logs test**

Create `test/smoke/logs_test.go`:

```go
//go:build smoke

package smoke

import (
	"encoding/json"
	"testing"
)

func TestSmokeLogsQuery(t *testing.T) {
	url, user, pass, proj := requireEnv(t)
	freshXDG(t)

	if _, _, code := runCLI(t, pass+"\n", "login", "--url", url, "--username", user, "--password-stdin"); code != 0 {
		t.Fatal("login failed")
	}
	if _, _, code := runCLI(t, "", "projects", "use", proj); code != 0 {
		t.Fatalf("projects use %s failed", proj)
	}

	stdout, stderr, code := runCLI(t, "", "logs", "query", "--since", "1h", "--page-size", "5", "--output", "json")
	if code != 0 {
		t.Fatalf("logs query exit %d\nstderr: %s", code, stderr)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("logs query stdout not JSON: %v\n%s", err, stdout)
	}
	if _, ok := resp["data"]; !ok {
		t.Errorf("logs query response missing 'data' field: %s", stdout)
	}
}
```

(Intentionally no archive/unarchive smoke test: it would require finding a real exception hash to mutate, then leave server-side state different from before the test. The unit + CLI tests in Tasks 1, 2, 4, 5 already cover wire shape.)

- [ ] **Step 5: Verify the smoke package still compiles**

Run: `nix develop --command go vet -tags smoke ./test/smoke/...`
Expected: no errors.

- [ ] **Step 6: Verify default test run is unaffected**

Run: `nix develop --command go test ./... -count=1`
Expected: PASS, no smoke tests run.

- [ ] **Step 7: Commit**

```bash
git add test/smoke/projects_test.go test/smoke/exceptions_test.go test/smoke/endpoints_test.go test/smoke/logs_test.go
git commit -m "test(smoke): projects/exceptions/endpoints/logs round trips"
```

---

## Task 8: README.md

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write the README**

Create `README.md`:

````markdown
# traceway-cli

A Go CLI for the [Traceway](https://github.com/tracewayapp/traceway) observability platform — exceptions, logs, endpoints, metrics. Designed to be **first-class for both LLMs (invoking via shell tools) and humans** (with `gh`-style ergonomics).

## Install

This repo ships a Nix dev shell with Go 1.26, `just`, `gotestsum`, `golangci-lint`, `govulncheck`, and `gh`:

```bash
nix develop
just build         # produces ./bin/traceway
```

Or vanilla Go:

```bash
go build -o bin/traceway ./cmd/traceway
```

## Quick start

```bash
# 1. log in (creates ~/.config/traceway/config.json + ~/.local/state/traceway/state.json)
traceway login --url https://cloud.tracewayapp.com

# 2. pick a project (one-time; future calls use it implicitly)
traceway projects list
traceway projects use <project-id>

# 3. ask questions
traceway exceptions list --since 24h
traceway logs query --since 1h --search "OutOfMemory"
traceway endpoints list --since 1h
traceway metrics query --name http.server.duration --aggregation avg --since 1h
```

## Commands

| Command | Purpose |
|---|---|
| `traceway login` | Authenticate and store the JWT |
| `traceway logout` | Forget the stored JWT for a profile |
| `traceway profiles {list,use}` | Manage multiple Traceway accounts/instances |
| `traceway projects {list,use}` | List or select the active project |
| `traceway exceptions list` | Recent grouped exceptions |
| `traceway exceptions show <hash>` | A single exception group + occurrences |
| `traceway exceptions archive <hash>...` | Archive one or more groups (mutating; needs `--yes` non-interactively) |
| `traceway exceptions unarchive <hash>...` | Unarchive (mutating; needs `--yes` non-interactively) |
| `traceway logs query` | Query logs with severity / service / search filters |
| `traceway endpoints list` | Per-endpoint p50/p95/p99 stats |
| `traceway metrics query` | Time-series metric queries |

Run `traceway <command> --help` for full per-command flags.

## Profiles

Multiple Traceway instances or accounts coexist via profiles. Configuration (URL, username) lives in `$XDG_CONFIG_HOME/traceway/config.json` so it can be checked in or managed declaratively (e.g. NixOS); credentials and the active project live in `$XDG_STATE_HOME/traceway/state.json`.

```bash
traceway login --url https://traceway.example.com --profile work
traceway profiles list
traceway profiles use work
traceway --profile personal exceptions list   # one-off override
```

## Output formats

The `--output` flag picks the format. The default is `table` on a TTY and `json` otherwise — i.e. piping always gets machine-readable output.

| Format | Use |
|---|---|
| `table` | Human-friendly columns (default on TTY) |
| `json` | Compact JSON, one record per line. Default when stdout isn't a TTY |
| `yaml` | YAML rendering of the same data |

`--fields a,b,c` projects list responses to just those keys (the `pagination` wrapper passes through unchanged):

```bash
traceway exceptions list --output json --fields exceptionHash,count,lastSeen
```

## Errors

Every error writes a stable JSON envelope to stderr (in `json` / `yaml` modes — prose in `table` mode) and exits with one of:

| Exit | Meaning |
|---|---|
| 0 | Success |
| 1 | Generic / API error |
| 2 | Usage error (bad flags, missing confirmation, invalid time range) |
| 3 | Connection failure |
| 4 | Auth failure (`not_authenticated`, `token_expired`, `forbidden`) |
| 5 | Not found |
| 6 | Rate limited |
| 7 | Server (5xx) |

Envelope shape:

```json
{"error":"token_expired","message":"session expired or invalid","hint":"traceway login","exit_code":4}
```

The `error` field is a stable snake_case identifier. LLMs/scripts can branch on it.

## Mutations + confirmation

`exceptions archive` and `exceptions unarchive` require explicit consent because they change server state:

- Pass `--yes` to skip the prompt.
- Or set `TRACEWAY_ASSUME_YES=1` in the environment.
- Or run interactively and answer the `Continue? [y/N]` prompt.

Calling a mutating command from a non-TTY context (script, LLM tool call) without one of the opt-ins fails immediately with `usage_error` (exit 2) — no hung prompts.

## Smoke testing

Unit/CLI tests run by default and never touch a network:

```bash
just test
```

End-to-end tests against a real Traceway instance are gated behind a build tag and read connection info from env vars:

```bash
export TRACEWAY_SMOKE_URL=https://traceway.example.com
export TRACEWAY_SMOKE_USERNAME=you@example.com
export TRACEWAY_SMOKE_PASSWORD=...
export TRACEWAY_SMOKE_PROJECT_ID=...
just smoke-test
```

If any of those vars is missing, the smoke tests skip cleanly rather than fail.

## Contributing

```bash
just check   # lint + test + vulncheck
```

The library at `pkg/client` deliberately has zero CLI dependencies — it's importable directly by other Go programs (e.g. a future MCP server).
````

- [ ] **Step 2: Verify the README renders without obvious issues**

Run: `nix develop --command markdown-it README.md > /dev/null 2>&1 || true` (best-effort; if `markdown-it` isn't in the dev shell, just visually inspect).

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: README"
```

---

## Task 9: Final regression sweep

**Files:**
- None (verification only)

- [ ] **Step 1: Run the full default test suite**

Run: `nix develop --command go test ./... -count=1`
Expected: PASS across all packages. No smoke tests reported.

- [ ] **Step 2: Run the linter**

Run: `nix develop --command just lint`
Expected: clean.

- [ ] **Step 3: Run the vulnerability scanner**

Run: `nix develop --command just vulncheck`
Expected: no findings (or only known-acceptable findings from prior plans, unchanged).

- [ ] **Step 4: Verify the smoke package builds under its tag**

Run: `nix develop --command go vet -tags smoke ./...`
Expected: clean.

- [ ] **Step 5: Sanity-check the binary**

Run:
```bash
nix develop --command just build
./bin/traceway --help
./bin/traceway exceptions --help
./bin/traceway exceptions archive --help
```
Expected: each prints usage with the expected flags. `exceptions archive --help` shows the `--yes` global flag and accepts `<hash> [<hash>...]`.

- [ ] **Step 6: If smoke env is configured, run the smoke suite as a final gate**

Run: `nix develop --command just smoke-test`
Expected: all smoke tests PASS (or SKIP if env is intentionally unset on this machine).

No commit — this task is verification only.
