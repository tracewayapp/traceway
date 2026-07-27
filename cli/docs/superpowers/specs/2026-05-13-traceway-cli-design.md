# traceway-cli — Design

**Date:** 2026-05-13
**Status:** Approved
**Owner:** Fred Drake

## Goal

Build a Go CLI for querying and lightly mutating a [Traceway](https://github.com/tracewayapp/traceway) instance (self-hosted or cloud). Traceway is an OpenTelemetry-native observability platform (logs, metrics, distributed traces, exception tracking, session replay, alerting).

The primary consumer is **LLMs invoking the CLI from a shell tool** (e.g., Claude Code's Bash tool). Humans are a first-class secondary audience: interactive use should feel like `gh` — pretty tables, color when on a TTY, sane defaults — but every choice that benefits LLMs (structured JSON, stable exit codes, explicit error envelopes) wins ties.

A future MCP server is in scope but not in this project. The CLI's `pkg/client` package is structured so a thin MCP server can import it directly later — no shelling, no double-parsing.

## Why CLI first

- **Auth ergonomics.** Traceway uses user JWTs minted via `POST /api/login`. A CLI owns its own config dir and runs `traceway login` once, caching the JWT. An MCP server is spawned without a TTY by Claude Code, so it can't prompt for a password and would need a pre-seeded token in SOPS/keychain.
- **LLM-friendly today.** Claude Code already has a Bash tool; `traceway ... --output json | jq` covers most workflows. MCP's edge (strict tool schemas, streaming) doesn't matter much for query-shaped observability tools.
- **Doesn't block MCP.** Library-first design (`pkg/client`) means the MCP wrapper is a few hundred lines later.

## Why standalone (no upstream Go imports)

- The `tracewayapp/go-client` repo is an instrumentation SDK (sender side — services use it to emit telemetry), not an API query client. Not useful for us.
- Traceway's `backend/app/models` package is internal; they can rename `ExceptionGroup` to `Issue` tomorrow. The actual stable contract is JSON-over-HTTP.
- We define our own types in `pkg/client`, mirroring Traceway's JSON shapes. Hand-rolled once from reading their controllers.

## Audience priority (decided)

LLM-first, humans at home. Specifically:

- `--output` defaults to `json` when stdout is not a TTY, `table` when it is.
- Errors emit a stable JSON envelope on stderr in JSON mode; prose on TTY.
- Exit codes are stable, documented, and emitted in the JSON envelope so LLMs can branch without inspecting `$?`.
- Color when on a TTY; off otherwise.
- Pretty tables via `text/tabwriter`. No `lipgloss`/`charmbracelet` for v1.

## Scope (v1)

**In:**

- Login / logout / profile management
- Project list and "use" pointer (per-profile current project)
- Read endpoints for: exceptions (list, show), logs (query), endpoints (list), metrics (query)
- Mutations on exceptions: archive, resolve. Gated by interactive prompt or `--yes`/`TRACEWAY_ASSUME_YES`.

**Out (v1):**

- Distributed traces, session replay, AI traces. Add later when needed.
- Alerts, dashboards, admin endpoints.
- Anomaly detection logic. We surface raw recent data; the LLM judges anomalies.
- Output trimming via curated default field sets. The `--fields` flag lets the caller project; we don't curate per resource.

## Use cases this CLI is optimized for

1. **"What's broken in prod?"** — `traceway exceptions list --since 1h`, drill into one with `traceway exceptions show <hash>`.
2. **"Why is endpoint X slow?"** — `traceway endpoints list --since 1h` for p50/p95/p99 + error rates.
3. **"What did service Y log around time T?"** — `traceway logs query --service Y --severity error --from ... --to ...`.
4. **"Anomalies in the last hour?"** — broad metric queries; LLM interprets.

## Non-functional decisions

| Concern | Decision |
|---|---|
| CLI framework | Cobra + Viper |
| HTTP client | stdlib `net/http` with thin wrapper |
| Output rendering | `internal/output`: json, yaml, table (`text/tabwriter`) |
| Config storage | `$XDG_CONFIG_HOME/traceway/config.json` (fallback `~/.config/...`), `0700` dir + `0600` file, atomic writes |
| Credential model | Per-profile JWT + URL + username stored on disk. Password never persisted. |
| Token expiry | On 401: re-prompt password if interactive; otherwise hint-and-exit. Retry once. |
| Profiles | Multi-profile from day one. `--profile` defaults to `default`. |
| Projects | Per-profile current project (`traceway projects use <id>`). `--project` flag overrides per call. |
| Time ranges | `--since 1h` *or* `--from`/`--to` (RFC3339), mutually exclusive. Default `--since 1h`. |
| JSON shape | Pass-through Traceway response. `--fields a,b,c` projects to subset. |
| Pagination | `--page-size N` (default 50), `--page N` (default 0). One HTTP call per command. |
| Mutations | Confirmation gate: TTY prompt, or `--yes` / `TRACEWAY_ASSUME_YES=1`. |
| Errors | Stable error envelope (`error`, `message`, `hint`, `exit_code`) on stderr in JSON mode; prose on TTY. |
| Exit codes | 0 success, 1 generic, 2 usage, 3 connection, 4 auth, 5 not-found, 6 rate-limited, 7 server. |
| Logging | None for v1. CLI is short-lived; errors go to stderr; no persistent log file. |

## Architecture & layout

```
traceway-cli/
├── cmd/traceway/                  # main entry, root command
│   ├── main.go                    # main(); panics → exit 1
│   ├── root.go                    # root *cobra.Command, global flags, output mode
│   ├── login.go                   # traceway login [--profile X] [--url ...]
│   ├── logout.go                  # traceway logout [--profile X]
│   ├── profiles.go                # traceway profiles {list,use}
│   ├── projects.go                # traceway projects {list,use}
│   ├── exceptions.go              # exceptions {list,show,archive,resolve}
│   ├── logs.go                    # logs query
│   ├── endpoints.go               # endpoints list
│   └── metrics.go                 # metrics query
│
├── pkg/client/                    # reusable HTTP client + types (MCP-ready)
│   ├── client.go                  # Client struct, constructor, transport, auth header
│   ├── errors.go                  # typed errors
│   ├── pagination.go              # Pagination struct
│   ├── time.go                    # TimeRange{Since, From, To} → RFC3339
│   ├── auth.go                    # Login(email, password) → JWT
│   ├── projects.go                # ListProjects()
│   ├── exceptions.go              # ListExceptions, GetException, ArchiveException, ResolveException
│   ├── logs.go                    # QueryLogs
│   ├── endpoints.go               # ListEndpoints
│   └── metrics.go                 # QueryMetrics
│
├── internal/config/               # config file load/save
│   ├── config.go                  # Config struct + (*Config).Active(name)
│   └── paths.go                   # XDG_CONFIG_HOME resolution
│
├── internal/output/               # rendering
│   ├── format.go                  # Mode enum, TTY detection
│   ├── json.go                    # json.Marshal + --fields projection
│   ├── yaml.go                    # yaml.Marshal + --fields projection
│   ├── table.go                   # text/tabwriter; per-resource columns
│   └── error.go                   # error renderer (matches Mode)
│
├── internal/exitcode/             # stable exit codes
│   └── codes.go
│
├── test/smoke/                    # build-tag-gated, real-Traceway tests
│
├── docs/                          # this directory
├── go.mod
├── flake.nix                      # adds `just` to dev shell
├── justfile                       # task runner — see Tooling
└── .gitignore                     # add .go/ for the dev shell's GOPATH
```

**Boundaries:**

- `pkg/client` knows nothing about Cobra, Viper, terminals, or config files. Pure HTTP-and-types library.
- `internal/config` and `internal/output` are CLI-only — under `internal/` so they cannot leak into a future MCP server.
- `cmd/traceway/*.go` is thin glue: parse flags → load config → call `pkg/client` → hand result to `internal/output`. No business logic.
- One file per resource throughout.

## Components

### `pkg/client.Client`

```go
type Client struct {
    BaseURL    string
    HTTPClient *http.Client  // injectable for tests
    JWT        string        // mutable; refreshed on Login
    UserAgent  string
}

type Option func(*Client)

func New(baseURL string, opts ...Option) *Client { ... }
func WithHTTPClient(c *http.Client) Option       { ... }
func WithJWT(jwt string) Option                  { ... }

// auth
func (c *Client) Login(ctx context.Context, email, password string) (jwt string, err error)

// resources
func (c *Client) ListExceptions(ctx context.Context, projectID string, req ListExceptionsRequest) (*ListExceptionsResponse, error)
func (c *Client) GetException(ctx context.Context, projectID, hash string) (*Exception, error)
func (c *Client) ArchiveException(ctx context.Context, projectID, hash string) error
func (c *Client) ResolveException(ctx context.Context, projectID, hash string) error
func (c *Client) ListProjects(ctx context.Context) ([]Project, error)
func (c *Client) QueryLogs(ctx context.Context, projectID string, req QueryLogsRequest) (*QueryLogsResponse, error)
func (c *Client) ListEndpoints(ctx context.Context, projectID string, req ListEndpointsRequest) (*ListEndpointsResponse, error)
func (c *Client) QueryMetrics(ctx context.Context, projectID string, req QueryMetricsRequest) (*QueryMetricsResponse, error)
```

All HTTP calls funnel through one private `do()` method that:

- Adds `Authorization: Bearer <jwt>` (when JWT is set)
- Sets `Content-Type: application/json`
- JSON-encodes the body
- Decodes the response
- Maps HTTP status to typed errors

Typed errors in `errors.go`:

- `ErrUnauthorized` (401)
- `ErrForbidden` (403)
- `ErrNotFound` (404)
- `ErrRateLimited` (429)
- `*APIError{StatusCode, Body}` for everything else

Use `errors.Is` / `errors.As` from the CLI layer to map to exit codes.

`TimeRange` helper takes a relative `--since` *or* explicit `--from`/`--to`, normalizes to `fromDate`/`toDate` RFC3339 in the request body. Lives in `pkg/client/time.go` so MCP gets it too.

### `internal/config.Config`

```go
type Config struct {
    CurrentProfile string             `json:"current_profile"`
    Profiles       map[string]Profile `json:"profiles"`
}

type Profile struct {
    URL              string `json:"url"`
    Username         string `json:"username"`
    JWT              string `json:"jwt"`
    CurrentProjectID string `json:"current_project_id,omitempty"`
}

func Load() (*Config, error)
func (c *Config) Save() error
func (c *Config) Active(name string) (*Profile, error)  // resolves --profile > CurrentProfile > "default"
```

- Path: `$XDG_CONFIG_HOME/traceway/config.json`, fallback `~/.config/traceway/config.json`. Created `0700` dir + `0600` file.
- On load: warn (don't fail) if perms are loose.
- On save: atomic write (tempfile + rename in same dir).
- Viper is used only for env-var binding (`TRACEWAY_PROFILE`, `TRACEWAY_PROJECT`, `TRACEWAY_ASSUME_YES`). The credentials file does not round-trip through Viper.

### `cmd/traceway` flow per command (representative)

```go
// cmd/traceway/exceptions.go — list subcommand
func runListExceptions(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()

    profile, err := resolveProfile(cmd)
    if err != nil { return err }
    project, err := resolveProject(cmd, profile)
    if err != nil { return err }
    timeRange, err := resolveTimeRange(cmd)
    if err != nil { return err }

    c := client.New(profile.URL, client.WithJWT(profile.JWT))
    resp, err := c.ListExceptions(ctx, project, client.ListExceptionsRequest{
        TimeRange:  timeRange,
        Pagination: client.Pagination{Page: page, PageSize: pageSize},
        Search:     search,
    })
    if err != nil {
        return handleAPIError(err, profile)  // 401 → re-prompt if TTY, else hint
    }

    return output.Render(cmd.OutOrStdout(), outputMode, fields, resp)
}
```

Each command is ~30–40 lines of glue.

## Data flow

### Flow A — `traceway login [--profile X] [--url ...]`

1. Resolve profile name (`--profile`, default `default`).
2. Load config; if profile exists, prompt with saved URL and username as defaults; otherwise prompt fresh (URL defaults to `--url` if set, else `https://cloud.tracewayapp.com`).
3. Read password (no echo) via `golang.org/x/term.ReadPassword`. `--password-stdin` reads from stdin instead (LLM-friendly).
4. Call `client.Login(URL, email, password)` → JWT.
5. Save profile `{URL, Username, JWT, CurrentProjectID=""}`. If this is the only profile, set `CurrentProfile` to its name.
6. Print `Logged in as <user> on <url> (profile: <name>)`. Exit 0.

`--url` is honored on profile creation. On refresh of an existing profile, the saved URL wins unless `--url` is explicitly passed (then update + save).

### Flow B — Query (e.g., `traceway exceptions list --since 1h`)

1. Parse flags → resolve profile → resolve project → build `TimeRange`.
2. `client.ListExceptions(ctx, projectID, req)`.
3. On 200: render to stdout in selected output mode, applying `--fields` projection if set. Exit 0.
4. On non-2xx: typed error → `handleAPIError` → stderr envelope + exit code. stdout stays empty.

One HTTP call per command. The `pagination` block in the response tells the caller whether more pages exist.

### Flow C — Mutation (e.g., `traceway exceptions archive <hash>`)

1. Parse flags → resolve profile, project.
2. Confirmation gate:
   - TTY on stdin AND `--yes` not set AND `TRACEWAY_ASSUME_YES` not set → prompt `Archive exception <hash>? [y/N]`.
   - Non-interactive AND no `--yes` → exit 2 + `usage_error`. Fail fast; don't stall.
3. `client.ArchiveException(ctx, projectID, hash)`.
4. On success: print `Archived exception <hash>` (or `{"archived": "<hash>"}` in JSON mode). Exit 0.

LLMs always pass `--yes`; humans get the prompt.

### Flow D — Token expiry (401 from any call)

1. Any client call returns `ErrUnauthorized`.
2. `handleAPIError(err, profile)`:
   - TTY on stdin AND not `--output json` AND not `--no-prompt` → `Session expired. Re-login as <user>? [Y/n]`. Prompt password → `client.Login()` → save JWT → retry the original call **once**.
   - Otherwise → stderr envelope `{error: "token_expired", hint: "traceway login --profile X"}`, exit 4.
3. If retry fails: exit 4 with the same error. No infinite loops.

`--no-prompt` global flag lets humans opt into LLM-style behavior in a TTY.

## Error handling

### Exit codes (stable contract)

| Code | Meaning | Examples |
|------|---------|----------|
| 0 | Success | Command ran, returned data (even if empty `[]`) |
| 1 | Generic error | Bug, panic recovered, unknown failure |
| 2 | Usage error | Missing required flag, mutually exclusive flags, bad time format, mutation without `--yes` in non-TTY |
| 3 | Connection error | DNS, refused, TLS, timeout — anything before HTTP status |
| 4 | Auth error | 401, 403, expired token, missing profile, missing JWT |
| 5 | Not found | 404 |
| 6 | Rate limited | 429 |
| 7 | Server error | 5xx from Traceway |

Defined in `internal/exitcode/codes.go`. Documented in README. Emitted in the JSON error envelope.

### Error envelope (JSON mode)

```jsonc
// stderr, on failure
{
  "error": "token_expired",
  "message": "JWT expired or invalid",
  "hint": "traceway login --profile stormwind",
  "exit_code": 4
}
```

- stdout stays empty on failure. `| jq` pipelines never see partial garbage.
- `error` is a stable enum the LLM can branch on: `token_expired`, `not_authenticated`, `forbidden`, `no_profile`, `no_project`, `invalid_time_range`, `not_found`, `rate_limited`, `connection_failed`, `server_error`, `usage_error`, `api_error`, `internal`.
- `hint` is best-effort; absent for errors with no actionable next step.

### Error envelope (TTY / table mode)

```
Error: session expired
  Hint: traceway login --profile stormwind
```

`Error:` prefix in red when stderr is a TTY.

### `pkg/client` → CLI mapping

| `pkg/client` error | Exit | JSON `error` |
|---|---|---|
| `ErrUnauthorized` | 4 | `token_expired` (or `not_authenticated` if no JWT was sent) |
| `ErrForbidden` | 4 | `forbidden` |
| `ErrNotFound` | 5 | `not_found` |
| `ErrRateLimited` | 6 | `rate_limited` |
| `*APIError` 5xx | 7 | `server_error` |
| `*APIError` other | 1 | `api_error` |
| `*url.Error` (network) | 3 | `connection_failed` |
| Cobra usage errors | 2 | `usage_error` |
| Config load failure | 4 | `not_authenticated` |
| Mutation w/o `--yes` non-TTY | 2 | `usage_error` |

### Explicitly NOT done

- No retry/backoff inside the CLI for 5xx or rate-limiting. Caller decides.
- No partial-success accumulation. One command = one HTTP call = one outcome.

## Testing

### Layer 1 — Unit tests (`pkg/client/*_test.go`)

Cover: request encoding, response decoding, error mapping, time-range conversion, pagination shape.

Pattern: `httptest.NewServer` per test. Test server returns canned JSON or specific status codes. Verify request body, headers, and decoded response shape.

```go
func TestListExceptions_decodesResponse(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // assert path, method, auth header, body
        json.NewEncoder(w).Encode(canned)
    }))
    defer srv.Close()

    c := client.New(srv.URL, client.WithJWT("test-token"))
    resp, err := c.ListExceptions(ctx, "proj-1", req)
    // assert resp shape
}
```

Coverage target: ~80% on `pkg/client`.

### Layer 2 — CLI behavior tests (`cmd/traceway/*_test.go`)

Cover: flag parsing, profile/project resolution, output format selection, error rendering, exit codes, mutation gates.

Pattern: same `httptest` setup, but invoke commands via `cobra.Command.Execute()` with captured stdout/stderr/stdin. `t.TempDir()` as `XDG_CONFIG_HOME`.

Key cases:

- `--profile X` overrides `current_profile`
- `--project X` overrides profile's `current_project_id`
- Missing profile → exit 4 + `not_authenticated`
- Mutation in non-TTY without `--yes` → exit 2 + `usage_error`
- 401 in non-TTY → exit 4 + `token_expired`, no retry
- `--output json` produces valid JSON; `--fields a,b` projects correctly
- `--since 1h` and `--from`/`--to` both produce correct RFC3339
- `--since` + `--from` together → exit 2 + `usage_error`

### Layer 3 — Smoke tests against real Traceway (`test/smoke/*_test.go`)

Cover: that response shapes still match what stormwind actually returns. Early warning for upstream API drift.

Build-tag-gated (`//go:build smoke`). Reads `TRACEWAY_SMOKE_URL`, `TRACEWAY_SMOKE_USERNAME`, `TRACEWAY_SMOKE_PASSWORD`, `TRACEWAY_SMOKE_PROJECT_ID` from env. Skipped if any are absent. Hits each endpoint, decodes into our types, asserts no decode errors.

Run on demand or weekly cron in CI. Not blocking.

### Non-goals for v1

- No mock generators. `httptest` is enough.
- No fuzzing. Inputs are well-typed.
- No shell-level e2e tests (`bats`). Cobra-level tests cover the same ground.

## Tooling

`flake.nix` adds `just` to the dev shell. A `justfile` at repo root provides task aliases:

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

# convenience: run everything that should pass before a commit
check: lint test vulncheck
```

`gotestsum` is preferred over bare `go test` for nicer output (clean per-package summary, end-of-run failure list, JUnit support if needed). It's already in the dev shell.

## Open questions to resolve during implementation

1. **Personal access tokens in Traceway UI.** Research said no, but it's source-only. If stormwind's UI exposes "API tokens," we should support paste-a-token as an alternative auth path. Check before implementing `login`.
2. **JWT TTL.** Read `backend/app/middleware/use_app_auth.middleware.go` upstream to learn the lifetime. Drives nothing in v1 (we always re-login on 401), but worth knowing.
3. **Session replay handling.** Out of scope for v1, but if added later: return the recording URL/ID, don't stream the (large) events.

## Build order

1. `go mod init github.com/tracewayapp/traceway/cli`. Add `.gitignore` for `.go/`.
2. `pkg/client`: constructor, `Login`, error types, time helper, `httptest` unit tests.
3. `internal/config`: load/save, `Active()`, tests.
4. `internal/output`: JSON renderer, error envelope, TTY detection.
5. `cmd/traceway`: root, `login`, `logout`, `profiles`. Smoke against stormwind.
6. `internal/output/table.go`. `cmd/traceway/projects.go`.
7. `cmd/traceway/exceptions.go` (`list`, `show`) + matching `pkg/client/exceptions.go` reads.
8. Add mutations: `exceptions {archive,resolve}`. Confirmation gate.
9. `cmd/traceway/logs.go`, `endpoints.go`, `metrics.go` — one slice at a time.
10. `test/smoke/`. `justfile`. README.

Get one full vertical slice working (`pkg/client` → `cmd/traceway` → real HTTP call → decoded result against stormwind) before broadening. The patterns from that first slice repeat 6–8 times.

## Reference — Traceway API surface

Self-hosted Traceway: single Go binary, Gin web framework, default port 80, all routes under `/api/...`. PostgreSQL (metadata) + ClickHouse (telemetry). No GraphQL, no OpenAPI spec.

| CLI verb | Endpoint | Notes |
|---|---|---|
| `traceway projects list` | `GET /api/projects` | Bootstrap; bare array, no pagination wrapper |
| `traceway exceptions list` | `POST /api/exception-stack-traces` | Grouped by hash |
| `traceway exceptions show <hash>` | `POST /api/exception-stack-traces/:hash` | Full stack + occurrences |
| `traceway exceptions archive <hash>` | (TBD — read controller) | v1 mutation |
| `traceway exceptions resolve <hash>` | (TBD — read controller) | v1 mutation |
| `traceway logs query` | `POST /api/logs` | Filter by service/severity/trace-id/attrs; >24h needs a selector |
| `traceway endpoints list` | `POST /api/endpoints` | p50/p95/p99, error rates per HTTP endpoint |
| `traceway metrics query` | `POST /api/metrics/query` | name + aggregation + tag filters + groupBy |

All request bodies JSON; all responses paginated `{data: [...], pagination: {...}}`. Time filters use `fromDate`/`toDate` as RFC3339. **Exception:** `GET /api/projects` returns a bare JSON array with no pagination wrapper.

Authoritative response types live in `backend/app/models/*.model.go` upstream — read them, don't import them. Use `gh api repos/tracewayapp/traceway/contents/<path>` to fetch files without cloning.

Key files to read when implementing each subcommand:

- Routes: `backend/app/controllers/routes.go`
- Auth: `backend/app/controllers/auth.controller.go`, `backend/app/middleware/use_app_auth.middleware.go`
- Exceptions: `backend/app/controllers/exception_stack_trace.controller.go` + `backend/app/models/exception_stack_trace.model.go`
- Logs: `backend/app/controllers/log.controller.go` + `backend/app/models/log_record.model.go`
- Endpoints: `backend/app/controllers/endpoint.controller.go` + `backend/app/models/endpoint.model.go`
- Metrics: `backend/app/controllers/metric_query.controller.go` + `backend/app/models/metric_record.model.go`
- Projects: `backend/app/controllers/project.controller.go` + `backend/app/models/project.model.go`
- Config: `backend/app/config/config.go`
- Entry: `backend/cmd/run.go`
