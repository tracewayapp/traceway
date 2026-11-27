# AGENTS.md - Traceway Project

This file provides guidance to coding agents (Claude Code, Codex) when working with code in this repository. It describes the system as it is now. Three areas are split out into `dev-docs/` and read on demand: `environment.md` (every backend variable), `ci.md` (CI, releases, skills mirror) and `telemetry-internals.md` (trace storage, client metrics, metric rollup repair).

## Project Overview

Traceway is an error tracking and monitoring platform consisting of:
- **Frontend**: SvelteKit 2 dashboard application with Svelte 5
- **Backend**: Go/Gin API server with ClickHouse database
- **CLI**: Go/Cobra command-line client for the backend HTTP API (`/cli`), built twice from one source: read-only `traceway`, and `traceway-write` (`-tags write`, own config dir and OAuth client, write commands in `cmd/traceway/edition_write.go`). The skills in `skills/` and `plugins/` are generated from `cli/pkg/mcpserver/knowledge` by `just gen-skills` (`skills/traceway-setup` is written by hand). The skill and MCP evals live in `cli/test/evals/` (own Go module), never under `skills/`: anything in a skill directory ships to users and to the agent under test. Skill layout, the `tracewayapp/skills` mirror and its release: `dev-docs/ci.md`

Traceway Cloud, the cloud-hosted version, is a separate repository: `../traceway-cloud`.

---

## Code Style

- **No pointless comments**: Do not add comments that simply describe what the code does. The code should be self-explanatory. Only add comments when explaining non-obvious "why" decisions.
- **No `py-4` in dialog form content**: Do not add `py-4` on the content wrapper inside `AlertDialog` or `Dialog` components — it creates too much blank space between the form and the action buttons.
- **Dialog button labels & toasts**: For form dialogs, use descriptive button labels with icons instead of generic "Create"/"Update". The `{Entity}` is always a platform entity, capitalized (Project, Widget, Dashboard, Channel, Rule, Token, Invitation, ...). Create actions: `<Plus icon> New {Entity}` with `variant="success"`. Update actions: `<Check icon> Update {Entity}` with the default (primary) variant. Delete/revoke/remove confirm buttons: `<Trash2 icon> Delete {Entity}` (or `Revoke {Entity}` / `Remove {Entity}`) with `variant="destructive"`. After success, show `toast.success('Successfully created the {Entity}', { position: 'top-center' })` for creates and `'Successfully updated the {Entity}'` for updates. The button should only be `disabled` during the loading state — never disable it to enforce validation; let the backend return 422 and show the error in the dialog instead.

---

## Quick Start

### Development Commands
| Component | Command | Description |
|-----------|---------|-------------|
| Frontend | `cd frontend && npm run dev` | Dev server (port 5173) |
| Frontend | `npm run build` | Production build |
| Frontend | `npm run check` | TypeScript checking |
| Frontend | `npm run lint` | `prettier --check . && eslint .` (CI gate; `npm run format` fixes the prettier half) |
| Backend | `cd backend && go run ./cmd/traceway` | API server (port 8082) |
| Backend | `cd backend && govulncheck ./...` | Vulnerability scan (default tags only); CI also scans the other storage build-tag combos (`.github/workflows/backend-vulncheck.yml`) |
| CLI | `cd cli && just build` | Builds `bin/traceway` |
| CLI | `cd cli && just test` | Runs unit tests |
| CLI | `cd cli && just check` | Lint + test + vulncheck + skill drift + contract tests (pre-commit gate; CI enforces it for `cli/`) |
| CLI | `cd cli && just smoke-test` | Live E2E (needs `TRACEWAY_SMOKE_*` env vars) |
| CLI | `cd cli && just eval-harness` | Eval harness tests plus every case's oracle trajectory, fake model only ($0) |
| CLI | `cd cli && just eval-plan --preset smoke --max-cost-usd 30` | Eval matrix and live-price cost estimate ($0) |
| CLI | `cd cli && just eval-dispatch -f preset=smoke -f max_cost_usd=30` | Starts a paid eval run in the CLI Evals workflow; it still waits for approval at the `evals` environment gate |
| CLI | `cd cli && just eval-local ...` | Paid local eval run, for harness development only (`yes` typed on a terminal, refused without one; required `--max-cost-usd`) |
| Any | `nix develop` (or direnv via `.envrc`) | Dev shell with the Go and Node versions read from `backend/go.mod` and `.nvmrc`, plus `just`, `golangci-lint`, `govulncheck` (`flake.nix`; shells `default`, `backend`, `frontend`, `oxc`). Put `JWT_SECRET` in the gitignored `.envrc.local` |
| Any | `./scripts/check-node-pins.sh` | Asserts every Dockerfile's `ARG NODE_VERSION` default matches `.nvmrc`; run by `release-traceway.yml` before it builds |

**Node's version lives in `.nvmrc`** and nothing else derives it independently. To move Node, edit `.nvmrc` and the `ARG NODE_VERSION` default of each Dockerfile, then run `./scripts/check-node-pins.sh`.

Set `JWT_SECRET` (min 32 characters) before running the backend, including the All-in-One image. Former public signing keys are rejected. The All-in-One shell script leaves `.env` loading to the backend so Docker-provided values win. Embedded `Run(opts...)` never reads `JWT_SECRET`, since it runs inside the host app's process where that name usually belongs to the host: it takes `WithJWTSecret`, else `TRACEWAY_JWT_SECRET`, else a random key per start; set a stable secret to preserve dashboard sessions across embedded restarts. The All-in-One image's supervisord sends the backend's output to the container's stdout/stderr, so `docker logs` shows a startup rejection. `SQLITE_PATH` sets the database location, defaulting to `./traceway.db` in the working directory. `BOOTSTRAP_FILE` points the standalone binary and the server images at a file that creates the organization, users and projects (with operator-chosen tokens) on the first start, while the main DB is still empty (`cmd/bootstrap.go`, docs `docs/pages/learn/preconfigure.mdx`).

CI on pull requests is opt-in: a PR runs nothing until a maintainer applies the `ci` label, and each application validates the PR as it is at that moment (remove and re-add the label to validate later pushes). `cli-evals.yml` (the paid skill and MCP evals) is started by hand only (`workflow_dispatch`, approval on the `evals` environment, required `max_cost_usd`) and must never get another trigger or caller; `cli/test/evals/internal/guard/triggers_test.go` fails if that changes. Dispatching a `release-*.yml` workflow publishes for real, it is not a dry run. The workflow list, Dependabot, action pins and the skills mirror: `dev-docs/ci.md`.

### Tech Stack
- **Frontend**: SvelteKit 2.49, Svelte 5.45, Tailwind CSS v4, shadcn-svelte, Vite 7
- **Backend**: Go 1.26, Gin 1.12, ClickHouse, PostgreSQL (in `backend/go.mod` the `go` line is the floor importers inherit and the `toolchain` directive is what builds it — keep the two distinct)
- **CLI**: Go 1.26, Cobra 1.10, separate Go module (`github.com/tracewayapp/traceway/cli/v2`); justfile entrypoints, covered by the root `flake.nix` dev shell

### lit Library (SQL mapper)
- **Import**: `github.com/tracewayapp/lit/v2` (currently v2.0.5)
- **Purpose**: Lightweight generic CRUD operations on top of `database/sql`. Supports PostgreSQL, MySQL, SQLite, and DuckDB drivers (`lit.PostgreSQL`, `lit.MySQL`, `lit.SQLite`, `lit.DuckDB`). Traceway registers models against `db.Driver` (SQLite or PostgreSQL depending on build tags); the DuckDB telemetry backend deliberately keeps `lit.SQLite` for reads since DuckDB accepts `?` placeholders.
- **Docs for AI use**: the lit repo ships a skill at `skills/lit/SKILL.md` plus `llms.txt` with the full API contract and pitfall checklist.

#### Model Registration (required before use)
All models are registered centrally in `models/models.go` via `models.Init(driver lit.Driver)`, called at boot with `db.Driver`:
```go
func Init(driver lit.Driver) {
    lit.RegisterModel[User](driver)
    lit.RegisterModel[Project](driver)
    // ...all models registered here
}
```
Repository-local result models (e.g., aggregate structs only used in one repo) can use file-level `init()` instead. Irregular table names use `lit.RegisterModelWithNaming` with a struct embedding `lit.DefaultDbNamingStrategy` (see `escalationPolicyNaming` in `models.go`).

#### Naming Conventions
- Fields: CamelCase → snake_case (`FirstName` → `first_name`)
- Consecutive uppercase: stay together (`HTTPCode` → `http_code`)
- Tables: pluralize + snake_case (`User` → `users`)
- Override via struct tag: `lit:"custom_name"`

#### Core CRUD Operations
All lit functions take a `lit.Executor` as the first argument. Both `*sql.Tx` and `*sql.DB` qualify: main-DB repositories pass the `*sql.Tx` from the transaction middleware, telemetry repositories pass `db.TelemetryDB` directly.

| Function | Description |
|----------|-------------|
| `lit.Insert[T](tx, &entity)` | Insert, returns auto-generated int ID |
| `lit.InsertUuid[T](tx, &entity)` | Insert with auto-generated UUID |
| `lit.InsertExistingUuid[T](tx, &entity)` | Insert with pre-set UUID |
| `lit.Select[T](tx, query, args...)` | Retrieve multiple records (returns `[]*T`) |
| `lit.SelectSingle[T](tx, query, args...)` | Retrieve one record (returns `*T`) |
| `lit.Update[T](tx, &entity, "id = $1", id)` | Update (auto-prepends WHERE) |
| `lit.UpdateNative(tx, "UPDATE table SET col = $1 WHERE ...", args...)` | Raw SQL update for partial/single-field changes |
| `lit.Delete(tx, "DELETE FROM table WHERE id = $1", id)` | Delete records (any exec statement) |
| `lit.SelectNamed[T]` / `lit.SelectSingleNamed[T]` / `lit.UpdateNamed[T]` | `:name` placeholder variants taking a `lit.P{...}` map; render per-driver, used for dialect-neutral queries |
| `lit.DeleteNamed(driver, tx, query, lit.P{...})` | Named delete/exec. Gotcha: takes the driver as the FIRST argument |
| `lit.ParseNamedQuery(driver, query, lit.P{...})` | Renders `:name` to positional; escape hatch for `QueryContext`/`RowsAffected` via `database/sql` |

#### Transaction Helper (`db.ExecuteTransaction`)
Main-DB work outside a transactional route uses `db.ExecuteTransaction` for automatic commit/rollback:

```go
// ExecuteTransaction[T] wraps a function in a transaction
// - Commits on success, rolls back on error or panic
// - Returns (T, error) directly - no pointer wrapping

project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
    // All repository calls receive the transaction
    return transactional.ProjectRepository.FindById(tx, id)
})
```

#### Transactional Middleware (`middleware.Transactional`)
For auth flows and routes requiring transaction context throughout the request lifecycle, use the `Transactional` middleware:

```go
// In routes.go - wrap routes that need transaction context
api.POST("/register", middleware.Transactional, authController.Register)
api.POST("/login", middleware.Transactional, authController.Login)

// In controller - retrieve transaction from Gin context
func (c *AuthController) Register(ctx *gin.Context) {
    tx := db.GetTx(ctx)  // Get transaction from context (db package, not middleware)

    // Use tx for all repository calls
    user, err := transactional.UserRepository.FindByEmail(tx, email)
    if err != nil {
        ctx.JSON(500, gin.H{"error": err.Error()})
        return  // Transaction auto-rolls back on non-success status
    }

    ctx.JSON(201, user)  // Transaction auto-commits on any 2xx/3xx status
}
```

**Auto-commit/rollback behavior:**
- Commits on any 2xx or 3xx status (`200 <= status < 400`)
- Rolls back on all other status codes or panics

**Response buffering:** `Transactional` holds the handler's response in memory (`responseBuffer` in `buffer_response.go`) and only releases it after `Commit()` succeeds; a failed commit answers 500 with an empty body instead of the 2xx the handler rendered, and commit hooks run before the release (each hook is isolated by `runCommitHook`, so a panicking hook is recorded on `c.Errors` without cancelling the others or the response). Handlers on transactional routes therefore must not stream, flush or hijack.

**Body buffering:** `Transactional` reads the request body into memory (cap `maxTransactionalBodyBytes`, 8MB) before it calls `db.DB.BeginTx`, answering 413/408/400 without a transaction when that fails; the handler's later bind is served from memory. This is what keeps a slow-drip body from holding the single SQLite main-DB connection, and it covers every transactional route without each one remembering a special middleware. Routes that want a tighter cap put `middleware.BufferAuthBody` (64KB) in front of `Transactional`, which then skips the body it finds already buffered; the anonymous auth endpoints do this. The self-transacting OAuth routes (`/auth/device/*`, `/auth/token`, `/auth/logout`) use `BufferAuthBody` on its own for the same cap. `UseAppAuth` caps every dashboard route's body at the same 8MB with `MaxBytesReader`, so the non-transactional telemetry reads are bounded too. Every plain JSON bind site in the controllers answers through `middleware.RejectBindError(c, err, fallback)`, which maps `MaxBytesError` to 413 and the body guard's timeout to 408 with fixed messages and otherwise returns 400 with the fallback (the ingest routes use `middleware.RejectIngestBindError`, the same mapping except that the timeout becomes 503 + `Retry-After`, because OTLP exporters retry 503 but treat 408 as permanent; the Traceway SDKs re-queue a failed batch whatever the status); the guard itself swaps the raw deadline error (a `*net.OpError` naming the listener address) for `middleware.ErrBodyTimedOut` before it leaves `Read`, so no handler can echo the address.

**Preference:** For CRUD controller methods, always prefer using `middleware.Transactional` in the route + `db.GetTx(ctx)` in the controller over `db.ExecuteTransaction`. The middleware approach keeps controllers flat, avoids nested closures, and follows the established pattern. (The tx getter lives in the `db` package: `db.GetTx(ctx)`, not `middleware.GetTx`.)

#### Repository Pattern
Repositories accept `*sql.Tx` to participate in transactions:
```go
func (p *projectRepository) FindById(tx *sql.Tx, id uuid.UUID) (*models.Project, error) {
    return lit.SelectSingle[models.Project](
        tx,
        "SELECT id, name, token, framework, created_at FROM projects WHERE id = $1",
        id,
    )
}
```

#### PostgreSQL Specifics
- Uses `$1, $2, $3` placeholders (not `?`)
- Tables must have an `id` column
- Always pass `*sql.Tx` from `ExecuteTransaction` to lit functions

#### Common Pitfalls

**Always initialize all struct fields with defaults:**
When using `lit.Insert`, all struct fields are included in the INSERT statement, overriding database defaults. Always set fields like `CreatedAt` explicitly:

```go
// CORRECT - set CreatedAt explicitly
user := &models.User{
    Email:     email,
    Name:      name,
    CreatedAt: time.Now().UTC(),
}

// WRONG - CreatedAt remains zero value (0001-01-01)
user := &models.User{
    Email: email,
    Name:  name,
}
```

**lit.Update WHERE clause:**
The `lit.Update` function automatically includes `WHERE` in the generated SQL. Do not add `WHERE` yourself:

```go
// CORRECT - just the condition
lit.Update(tx, &user, "id = $1", user.Id)

// WRONG - results in "WHERE WHERE id = $1"
lit.Update(tx, &user, "WHERE id = $1", user.Id)
```

#### Custom Result Models for Aggregates
For queries that return aggregated or computed values (not direct table rows), create a custom result model:

```go
// Define a result model for the query output
type CountResult struct {
    Count int `lit:"count"`
}

// Register in models.Init() (or file-level init() if repo-local)
lit.RegisterModel[CountResult](lit.PostgreSQL)

// Use in repository
func (r *userRepository) CountByOrganization(tx *sql.Tx, orgID uuid.UUID) (int, error) {
    result, err := lit.SelectSingle[CountResult](
        tx,
        "SELECT COUNT(*) as count FROM users WHERE organization_id = $1",
        orgID,
    )
    if err != nil {
        return 0, err
    }
    if result == nil {
        return 0, nil
    }
    return result.Count, nil
}
```

#### Handling "Not Found" Cases
**IMPORTANT:** When using lit/PostgreSQL, do NOT check for `sql.ErrNoRows`. The lit library returns `nil` when no record is found, not an error. Always check for `nil` instead:

```go
// CORRECT - check for nil
user, err := transactional.UserRepository.FindByEmail(tx, email)
if err != nil {
    return nil, err  // actual database error
}
if user == nil {
    // record not found - handle accordingly
    return nil, errors.New("user not found")
}

// WRONG - do not use sql.ErrNoRows with lit
user, err := transactional.UserRepository.FindByEmail(tx, email)
if err == sql.ErrNoRows {  // This won't work with lit!
    // ...
}
```

#### Handling "Not Found" Cases (ClickHouse)
**IMPORTANT:** ClickHouse queries behave differently from lit/PostgreSQL. ClickHouse returns `sql.ErrNoRows` when no record is found. Always use `errors.Is()` to check:

```go
// CORRECT - ClickHouse returns sql.ErrNoRows
exception, err := exceptionRepo.GetByHash(projectID, hash)
if errors.Is(err, sql.ErrNoRows) {
    // record not found - handle accordingly
    return nil, errors.New("exception not found")
}
if err != nil {
    return nil, err  // actual database error
}

// Summary of error handling:
// - lit/PostgreSQL: check `if result == nil` (no error returned)
// - ClickHouse: check `errors.Is(err, sql.ErrNoRows)`
```

### Environment Variables (Backend)

The full reference, with the default, limits and sizing notes of every variable, is `dev-docs/environment.md`. Document a new variable there. What a local run needs:

```
JWT_SECRET=<min 32 char secret>       # required
DB_TYPE=                              # sqlite | postgres; sqlite unless built with -tags transactional_pg
SQLITE_PATH=                          # main DB path, default ./traceway.db; the telemetry DB derives from it
PORTS=80,8082                         # HTTP listen ports; a port that fails to bind only logs a warning
CLICKHOUSE_SERVER=localhost:9000      # plus CLICKHOUSE_DATABASE, _USERNAME, _PASSWORD, _TLS
POSTGRES_HOST=localhost               # plus POSTGRES_PORT, _DATABASE, _USERNAME, _PASSWORD, _SSLMODE
MONITORING_TRACEWAY_URL=              # <project_token>@<instance URL>, turns self-monitoring on
```

---

## Architecture Overview

### Data Flow
```
Backend apps        → [OpenTelemetry SDK] → OTLP/HTTP POST /api/otel/v1/*            ┐
Browser/mobile apps → [Traceway SDKs]     → OTLP, or gzipped POST /api/report        ┴→ Backend → telemetry DB
                                                                                          ↓
Dashboard ← [SvelteKit Frontend] ← JSON API ← Gin Controllers
```

### Authentication
Two-tier system:
1. **Client Auth**: Project bearer tokens (SDK telemetry via `Authorization: Bearer <project_token>`)
2. **App Auth**: JWT-based user authentication (dashboard via `Authorization: Bearer <jwt_token>`)

**App Auth credentials.** `UseAppAuth` accepts three credential shapes on the `Authorization: Bearer` header:
- **Dashboard JWT** — issued by `/api/login` / SSO (7-day expiry).
- **Personal access token (PAT)** — opaque `twp_`-prefixed token; looked up by SHA-256 hash in `personal_access_tokens`, resolves to its user. Created/listed/revoked from the account page (`/api/personal-access-tokens*`). Non-expiring or with an optional TTL; `last_used_at` is touched (throttled to 1/min, off the request path).
- **Device-flow access token** — a short-lived (15-min) JWT minted by the CLI's OAuth device flow.

**Agent scopes and the agent gate.** Every credential except a dashboard session is an *agent* credential with a scope, `traceway:read` or `traceway:write` (`models.ScopeRead/ScopeWrite`). PATs store it (`personal_access_tokens.scope`; read tokens are `twp_`, write tokens `tww_` and must expire within 90 days), refresh tokens and authorization codes store it, and access JWTs carry it as the `scope` claim (plus `cid`, the OAuth client). Device logins take their scope from the client id (`knownDeviceClients`: `traceway-cli` and `traceway-mcp` read, `traceway-cli-write` write); the authorization-code grant takes it from the request's `scope` param (`models.ParseRequestedScope`, default read); a refresh keeps the family's scope; `POST /api/login` with a `scope` field returns a token set instead of a session (`token` is a 15-minute scoped access JWT, plus `refreshToken`/`expiresIn` in a refresh family with client id `traceway-cli` or `traceway-cli-write`, so `POST /api/auth/logout` revokes it; the CLI's password login always sends a scope and stores the result like a device login). An unscoped JWT that lives one hour or less predates scopes and is held to read. `UseAppAuth` sends agent credentials through `authorizeAgentRequest` (`middleware/agent_gate.go`): the route must be in the allowlist `agentRoutes` (`middleware/agent_routes.go`, keyed `METHOD /api/path` by `c.FullPath()`, default deny, so new dashboard routes are never agent-reachable by accident; `TestAgentRoutesAreRegistered` keeps entries pointing at real routes); a write route needs the write scope (else 403 with a `scope="traceway:write"` challenge) and the owning organization's opt-in `organizations.agent_write_enabled` (Settings > Agent Access, `PUT /api/organizations/:organizationId/agent-access`, else 403), resolved from `?projectId=` together with membership, which is why every allowlisted write is project-scoped. Accepted and opt-in-refused agent writes are recorded in `agent_write_events` (main DB, pruned after 90 days by the auth-tokens worker, listed at `GET /api/organizations/:organizationId/agent-activity`). The role checks (`RequireWriteAccess` etc.) still apply after the gate. Dashboard sessions skip the gate entirely. Docs: `docs/pages/learn/agent-write-access.mdx`.

**CLI / OAuth device flow** (`backend/app/services/authserver/`, controllers `device_auth.controller.go` / `wellknown.controller.go` / `pat.controller.go`): RFC 8628 device authorization grant plus rotating refresh tokens. `traceway login` (default) → `POST /api/auth/device/authorize` (client_id allowlisted, per-IP rate-limited, opportunistically prunes expired rows) → user approves at `/device` → the CLI polls `POST /api/auth/device/token` (grant `device_code`; `/api/auth/token` is an equivalent alias) which issues a 15-min access token + 90-day rotating refresh token (family-tracked in `refresh_tokens`). Refresh (`grant_type=refresh_token`) rotates the token atomically and revokes the whole family on genuine reuse; within a 30s grace window a benign concurrent retry is answered with the same rotated token set (from an in-memory rotation cache) instead of `invalid_grant`. `POST /api/auth/logout` revokes a family server-side. Tokens are stored SHA-256-hashed. The grant endpoints **self-manage their transactions** via `db.ExecuteTransaction` (not `middleware.Transactional`) because OAuth returns 400 for normal flow control (`authorization_pending`, reuse-revoke) and those side effects must still commit. `/.well-known/oauth-authorization-server` and `/.well-known/oauth-protected-resource` are served at the **origin root** (registered in `cmd/run.go`, not the `/api` group) per RFC 8414 / 9728. Beyond the device grant, the server implements the **authorization-code + PKCE grant** (S256 only) with **RFC 7591 dynamic client registration** for MCP clients: `POST /api/oauth/register` (rate-limited, open registration of public clients; https/custom-scheme redirect URIs anywhere, plain http only on loopback with port-flexible matching per RFC 8252) -> the client sends the user to `/oauth/authorize` (an SPA consent page like `/device`) -> the page calls `GET /api/oauth/client` + `POST /api/oauth/approve|deny` (approve mints a single-use 5-min `twa_` code bound to user/client/redirect/challenge; the redirect target is validated server-side) -> the client exchanges it at `POST /api/auth/token` (grant `authorization_code`; the code is consumed even on a failed exchange, wrong verifier/client/redirect are all `invalid_grant`). RFC 8707 `resource` params are validated against the issuer origin (`invalid_target`). Expired codes are pruned by the auth-tokens retention worker and opportunistically at approve time.

---

## Frontend (`/frontend`)

### Framework & Build
- **Framework**: SvelteKit 2 with Svelte 5 runes API
- **Styling**: Tailwind CSS v4 with shadcn-svelte components
- **Build**: Vite 7, static adapter with SPA fallback
- **SSR**: Disabled - pure client-side SPA (`ssr = false` in `+layout.ts`)

### Project Structure
```
frontend/
├── src/
│   ├── routes/              # SvelteKit pages
│   ├── lib/
│   │   ├── api.ts           # API client with auth
│   │   ├── state/           # Svelte 5 state management
│   │   ├── components/
│   │   │   ├── ui/          # shadcn-svelte base components
│   │   │   └── traceway/    # Custom Traceway components
│   │   └── utils/           # Helpers (formatting, sorting)
│   └── app.css              # Tailwind + global styles
├── static/                  # Static assets
└── svelte.config.js         # SvelteKit config
```

### State Management (Svelte 5 Runes)

#### Runes Pattern
```typescript
// Use $state() for reactive state
let data = $state<Type>(initial)

// Use $derived() for computed values
let computed = $derived(expression)

// Use $effect() for side effects
$effect(() => { /* reactive code */ })
```

#### State Files
| File | Purpose | Persistence |
|------|---------|-------------|
| `src/lib/state/auth.svelte.ts` | Token auth, login/logout | localStorage |
| `src/lib/state/projects.svelte.ts` | Multi-project management | localStorage |
| `src/lib/state/theme.svelte.ts` | Dark/light mode toggle | localStorage |
| `src/lib/state/timezone.svelte.ts` | UTC/local timezone toggle | localStorage |

#### Singleton Pattern
State files export class instances as singletons:
```typescript
// src/lib/state/auth.svelte.ts
class AuthState {
    token = $state<string | null>(null)
    isAuthenticated = $derived(!!this.token)

    constructor() {
        // Load from localStorage on init
        this.token = localStorage.getItem('token')
    }
}
export const authState = new AuthState()
```

### API Client (`src/lib/api.ts`)

The API client automatically:
- Includes `Authorization: Bearer <token>` header
- Adds `projectId` as a query parameter to all requests
- Handles 401 responses by logging out and redirecting to `/login`

```typescript
// Usage
const data = await api.post<ResponseType>('/endpoint', { body })
const data = await api.get<ResponseType>('/endpoint')
```

### Component Patterns

#### Table System
Tables use shadcn-svelte base components with custom Traceway wrappers:

| Component | Location | Purpose |
|-----------|----------|---------|
| `Table`, `TableHeader`, etc. | `src/lib/components/ui/table/` | Base shadcn components |
| `TracewayTableHeader` | `src/lib/components/traceway/traceway-table-header.svelte` | Adds sorting + tooltips |
| `TableEmptyState` | `src/lib/components/traceway/table-empty-state.svelte` | Empty state display |
| `PaginationFooter` | `src/lib/components/traceway/pagination-footer.svelte` | Pagination controls |

#### Sorting Storage
Table sorting persists to localStorage using a consistent pattern via `src/lib/utils/sort-storage.ts`:

```typescript
// Types
type SortState = { field: string; direction: 'asc' | 'desc' }

// Key format: traceway_sort_{pageKey}
// Example: traceway_sort_issues, traceway_sort_endpoints

// In +page.svelte - load initial state
let sortState = $state<SortState>(getSortState('issues', { field: 'last_seen', direction: 'desc' }))

// After sort change
function onSortClick(field: string) {
    sortState = handleSortClick(field, sortState.field, sortState.direction, 'desc')
    setSortState('issues', sortState)  // Persist to localStorage
}
```

**Available functions (`src/lib/utils/sort-storage.ts`):**
| Function | Description |
|----------|-------------|
| `getSortState(pageKey, defaultState)` | Load sort state from localStorage |
| `setSortState(pageKey, state)` | Save sort state to localStorage |
| `handleSortClick(field, currentField, currentDirection, defaultDirection)` | Toggle sort direction, returns new `SortState` |

#### TracewayTableHeader Component
```svelte
<TracewayTableHeader
    label="Last Seen"
    column="last_seen"
    tooltip="When this issue was last reported"
    {orderBy}
    onclick={() => handleSortClick('last_seen')}
/>
```

### URL State Management

Time range and filters persist in URL query params via `src/lib/utils/url-params.ts`:

```typescript
// Available presets: 30m, 60m, 3h, 6h, 12h, 24h, 3d, 7d, 1M, 3M

// Parse time range from URL (in +page.svelte)
const timeRange = parseTimeRangeFromUrl(timezoneState.timezone, '24h')

// Get resolved Date objects for API calls
const { from, to } = getResolvedTimeRange(timeRange, timezoneState.timezone)

// Update URL with new time range (preserves other params)
updateUrl({ preset: '7d' })
updateUrl({ from: customFrom, to: customTo })  // Custom range
```

**Available functions (`src/lib/utils/url-params.ts`):**
| Function | Description |
|----------|-------------|
| `parseTimeRangeFromUrl(timezone, defaultPreset)` | Parse `TimeRangeParams` from current URL |
| `getResolvedTimeRange(params, timezone)` | Convert params to `{ from: Date, to: Date }` |
| `updateUrl(params, options?)` | Update URL query params, optionally replace history |

### Navigation Utilities

Helper functions for preserving URL params during navigation (`src/lib/utils/navigation.ts`):

```typescript
// Add sticky params (like time range) to href for <a> tags
const href = addStickyParamsToHref('/issues/abc123', 'preset', 'from', 'to')
// Result: "/issues/abc123?preset=24h" (if preset=24h is in current URL)

// Create click handler for table rows that preserves params
const handleClick = createRowClickHandler('/issues/abc123', 'preset', 'from', 'to')
```

**Available functions:**
| Function | Description |
|----------|-------------|
| `addStickyParamsToHref(href, ...stickyParams)` | Returns href with specified params from current URL |
| `createRowClickHandler(href, ...stickyParams)` | Returns click handler that navigates with sticky params |

### Routes
```
/                           Dashboard (protected) - overview metrics
/login                      Login page (public)
/register                   Registration page (public)
/issues                     Issues list with filtering/sorting
/issues/[hash]              Exception details view
/issues/[hash]/events       Exception events timeline
/endpoints                  Endpoint analytics with P50/P95/P99
/endpoints/[endpoint]       Single endpoint details
/tasks                      Background tasks list
/tasks/[task]               Single task details
/spans                      Span explorer: every OTel span of the project, filtered by name, service, kind, status, duration, trace ID and attribute values (time range defaults to 60m)
/spans/[traceId]            Whole trace waterfall across services and across every project of the organization (?at= is required, ?span= highlights one span), with the current project's correlated logs on backend projects
/dashboards                 Dashboards page (tabs of org dashboards; /metrics redirects here)
/monitors                   Monitors (synthetic checks): single sidebar item, TabsRow tabs Monitors | Status Pages | Post-Mortems (?tab=, Status Pages admin-only, Post-Mortems for all members); status pages tab has branding (description, logo upload, custom domain) plus a link to the per-page incidents page (which owns the Record Incident dialog); old /monitors/status-pages redirects to the tab, /monitors/runners to /monitors (runners have no UI — operator infra), bare /monitors/post-mortems to the tab
/monitors/status-pages/[pageId]/incidents  Paginated incident management for one status page (timeline updates, titles, manual resolve/delete, post-mortem links); reads for members, mutations admin-gated in the UI
/monitors/[checkId]         Monitor detail (latency chart, uptime bars, runs w/ result filter, incidents w/ titles + post-mortem links)
/monitors/post-mortems/[id] Post-mortem editor/viewer (@milkdown/crepe WYSIWYG with a Rich text/Markdown raw toggle, tag pills + Add-tags dialog, incident link pill + searchable picker, Activity sheet; write gating follows the effective project role). Project-scoped: opening a doc under a different project 404s. Creation is a title-only dialog (new-post-mortem-dialog.svelte, openable from the tab, monitor detail incidents, and the status page incidents page with an optional incident pre-link) that POSTs immediately and navigates here; /monitors/post-mortems/new redirects to the tab
/slos                       SLOs (service level objectives, `backend/app/slo`): one row per objective with kind badge, SLI summary, target, window, current SLI, error-budget bar, 1h burn rate and status; sort key `slos`; sidebar badge = breached count; frontend and mobile projects see only the `session` kind (crash-free sessions, `sessions` joined to `exceptions_v2.session_id`), backend projects the other five
/slos/[id]                  SLO detail: SLI and error-budget charts over a range clamped to the window (default = the whole window), burn-rate tiles, a coverage note while history backfills, a "View bad events" deep link per kind, and the alert rules that reference the SLO
/status/[slug]              Public status page (light standalone design, no auth, raw fetch, listed in isPublicPath)
/embed/dashboard            Shared dashboard (`?dashboard=<uuid>#token=<jwt>`, optional `&theme=light|dark`): read-only, no auth, frameable, raw fetch with the token as bearer, in PUBLIC_PATHS. See "Shared dashboards" under Dashboards & Templates
/connection                 SDK integration guide
/data-collection            Remote SDK config: rules and captures (browser projects only; the backend also answers OpAMP polls of Flutter projects, but the dashboard has no page for them)
```

### UI Components
Location: `src/lib/components/ui/*`
Uses shadcn-svelte registry with bits-ui primitives. Key components:
- `button`, `card`, `table`, `badge`, `tooltip`
- `select`, `input`, `checkbox`
- `sheet` (slide-out panels), `dialog` (modals)

---

## Backend (`/backend`)

### Architecture
- **Framework**: Gin Gonic HTTP framework
- **Database**: ClickHouse (columnar OLAP for telemetry), PostgreSQL (relational for projects)
- **Port**: 8082
- **Pattern**: Repository pattern with singleton controllers

### Project Structure
```
backend/
├── traceway.go                 # package tracewaybackend - embeddable API (Run, WithPort, ...)
├── cmd/
│   ├── run.go                  # Run() implementation: DB init, routes, server start
│   ├── traceway/main.go        # Binary entry point (package main)
│   └── traceway-runner/        # Synthetics remote runner binary
└── app/
    ├── controllers/            # routes.go + one <name>.controller.go per area
    │   ├── clientcontrollers/  # /api/report ingest
    │   ├── otelcontrollers/    # OTLP ingest
    │   └── opampcontrollers/   # Remote SDK config polls
    ├── repositories/
    │   ├── transactional/      # Main DB: pg/ and sqlite/ twins behind a facade
    │   └── telemetry/          # Telemetry DB: clickhouse/, sqlite/, duckdb/ behind a facade
    ├── models/                 # Data structures (models.Init registers them with lit)
    ├── middleware/             # Auth, Transactional, admission gates, body guards
    ├── db/                     # Connections, GetTx, ExecuteTransaction, build-tag guards
    ├── cache/                  # In-memory project cache; on Postgres, replicas refresh each other through LISTEN/NOTIFY (project_cache_changed, sent by the project repository inside the write transaction)
    ├── migrations/             # pg/, sqlite/, ch/, sqlite_telemetry/, duckdb_telemetry/
    └── traceway/               # Self-monitoring API (errors, spans, tasks, metrics)
```

### Middleware Chain Composition

| Route Type | Middleware Chain |
|-----------|-----------------|
| Read-only telemetry | `UseAppAuth, RequireProjectAccess` |
| Write telemetry | `UseAppAuth, RequireProjectAccess, RequireWriteAccess` |
| PostgreSQL CRUD (read) | `UseAppAuth, RequireProjectAccess, Transactional` |
| PostgreSQL CRUD (write) | `UseAppAuth, RequireProjectAccess, RequireWriteAccess, Transactional` |
| Admin org management | `UseAppAuth, RequireAdminAccess, Transactional` |
| Public (auth/invitations) | `RateLimitPerIP`, `BufferAuthBody`, `Transactional` (limiter first so a throttled request never reads a body or opens a transaction) |
| Client SDK ingestion | `CORSReport, UseClientAuth, PrefetchReportBody, IngestAdmission, UseGzip` (OTLP: `otelcontrollers.PrefetchBody` and no `UseGzip`; the prefetch must stay in front of the gate; `CORSReport` exposes `Retry-After` with a 2h preflight max age) |
| OpAMP (remote SDK config) | `OpampCors, UseClientAuth, OpampAdmission` (no Transactional: the poll reads only the in-memory `cache.SdkConfigCache`; no per-IP limiter, browsers behind one NAT would share it) |
| Synthetic runner API | `UseRunnerAuth` only (shared SYNTHETICS_RUNNER_SECRET bearer + self-registration by X-Traceway-Runner-Name; no Transactional — poll holds the request up to 25s) |
| Public status page | `RateLimitPerIP` only |
| Shared dashboard reads | `RateLimitPerIP` only (the handler validates the embed token itself) |

**Global middleware** (registered in `cmd/run.go` ahead of the routes): `middleware.GuardBodyReads` (every request body gets the progress guard, 30s idle / 10 min total / 1 KB/s floor after 20s, answering through `middleware.BodyReadError` as 408; routes with their own budget call `GuardBodyRead` again, which re-arms the same guard rather than wrapping twice, so uploads get 30s/30 min, `/api/report` 20s/10 min and OTLP 20s/2 min. The guard clears the socket read deadline the moment the body is complete: Go's background connection read would otherwise time out and cancel the request context of any handler that outlives the idle window. The `http.Server` `ReadTimeout` is a 1h backstop that only applies while a body is still arriving (the guard lifts it for bodiless requests too, so a long-lived handler such as the MCP `GET /mcp` stream is not cancelled after an hour), `ReadHeaderTimeout` is 15s), `configureClientIP` (gin's trusted proxies from `Cfg.TrustedProxyList`, plus `TrustedPlatform` from `TRUSTED_PROXY_HEADER`; a malformed list panics), `warnOnUntrustedForwardedFor` (list mode only: one-time log when a forwarding header arrives from a peer outside `TRUSTED_PROXIES`), and `middleware.SecurityHeaders` (`X-Frame-Options: DENY`, CSP `frame-ancestors 'none'; base-uri 'self'; form-action 'self'`, `Referrer-Policy`, `nosniff`). The two frame headers are skipped for any `/status/*` or `/api/status/*` path so public status pages stay embeddable in an iframe, and for exactly `/embed/dashboard` (shared dashboards). A custom-domain status host is exempt as a whole: the SPA handler serves nothing but that status page there, so it drops the two headers itself (see the status domains row below).

### API Endpoints

**Client SDK Ingestion**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/report` | Client | Telemetry ingestion (gzipped) |
| POST | `/api/otel/v1/traces` | Client | OTLP/HTTP trace ingestion |
| POST | `/api/otel/v1/metrics` | Client | OTLP/HTTP metric ingestion |
| POST | `/api/otel/v1/logs` | Client | OTLP/HTTP log ingestion; also turns browser SDK events into issues and sessions (see below) |
| POST | `/api/otel/v1development/profiles` | Client | OTLP/HTTP profile ingestion (development signal) |
| POST | `/api/recordings` | Client | Session replay segments from the OpenTelemetry browser SDK (`{debugIds?, segments: [ClientSessionRecording]}`, 1-100 segments, gzip or plain, `UseGzip` cap). Not OTLP. A segment's `exceptionId` is the SDK's `traceway.exception.id`, which is also the stored exception id, so a clip and its exception link at read time whichever arrives first. Shares `buildRecordingJob` with `/api/report`; gated by `IngestPermission.Replay` |

**Browser SDK conventions over OTLP** (`otelcontrollers/logs_converter.go`, `exception_builder.go`). The OpenTelemetry browser SDK (`@tracewayapp/frontend` 2.x) sends no native frames; the log path derives the dashboard's rows from log **event names** (`LogRecord.event_name`, also stamped into `LogAttributes["event.name"]`, since `log_records` has no column for it; when the field is empty the `event.name` attribute names the event, the older events convention):
- `exception` → an `exceptions_v2` row in every project. The stack is `exception.stacktrace` symbolicated with the ECMA-426 ids in `traceway.debug_ids` (JSON object, base file name → id; record, then span, then resource; dropped from every readable stored attribute column (log, resource, scope, span, event and link attributes, entity and exception maps; `shared.NormalizeStoredAttributes`), kept only in the lossless OTLP payload), then `formatExceptionStackTrace`. The id is `traceway.exception.id` when it is a UUID, otherwise a deterministic `uuid.NewSHA1` of the record, so a retried export rewrites the same id.
- `traceway.message` → an issue with `is_message` (the body is the message, hashed without normalization).
- `session.start` / `session.end` / `traceway.session.update` → `sessions` upserts carrying the full attribute snapshot. `traceway.session.started_at` (Unix ms, or RFC 3339) is required on every event but `session.start`, because ClickHouse sorts sessions by `started_at` and a different start would create a second row. `client.ip` is always the request's `c.ClientIP()`. Sessions are not gated by `IngestPermission`, like `/api/report`. The SQLite/DuckDB upsert only updates a row of the same project. On ClickHouse (`ReplacingMergeTree(version)`) a row with an end gets `version` = insert time + 100 years and an upsert without an end first copies the stored end (`withRecordedEnds`), so a recorded end is never erased.
- `session.id` (a UUID; anything else is ignored; every readable stored copy is rewritten to the canonical lowercase dashed form whatever spelling `uuid.Parse` accepted, so the ClickHouse `session_id` columns and the SQLite expression indexes match the timeline lookup) on the record, span or resource sets `exceptions_v2.session_id` on both the log and the span path. For client-side projects (`IsFrontendFramework`) the client resource attributes (`browser.*`, `os.*`, `device.*`, `user_agent.*`, `user.*`, `enduser.*`, `session.*`, `deployment.*`, `service.instance.id`) are copied onto exception and session rows.
- Ingest order on the logs path is records, sessions, exceptions, and the broadcast carries `ErrorCount`/`ExceptionHashes` so new-issue and regression rules fire for browser errors. A backend SDK that records an exception both as a span event and as an `exception` log (`OTEL_SEMCONV_EXCEPTION_SIGNAL_OPT_IN=logs/dup`) produces two occurrences.
- **Page performance becomes metrics.** For a browser project (`IsFrontendFramework`), `convertLogs` also turns `browser.web_vital`, `browser.long_task`, `browser.route_timing` and `browser.navigation_timing` records into `metric_points` (`otelcontrollers/browser_metrics.go`): `browser.web_vital.{lcp,inp,fcp,ttfb,cls}`, `browser.long_task.duration`, `browser.route_timing.duration`, `browser.page_load.duration`, `browser.page_load.dom_content_loaded`, tagged from an allowlist of resource attributes (`browser.name`, `browser.mobile`, `os.name`, `service.version`, `deployment.environment.name`, `server_name` = service name), the page's `url.path` and a few event attributes (`rating`, `outcome`, `type`). The vital names are a fixed set so a client cannot mint metric names; the log record is stored too. Gated by `IngestPermission.Data` like the records. This is why browser projects show **Dashboards** in the sidebar, get the `browser-performance` template (`Web Performance`) as their default (`defaultTemplateKeysForFramework`; the Dashboards empty state calls `populate-defaults` for a project that has none), and offer the `metric_threshold` rule type with presets (`routes/notifications/browser-metric-presets.ts`). Mobile projects have neither.
- **Breadcrumbs are telemetry, stored once.** The 2.x SDK sends console lines (`browser.console`), navigation (`browser.navigation`), clicks (`browser.user_action.click`), `recordAction` (`traceway.action`) and `track()` events as log records, and fetch/XHR as CLIENT spans, all stamped with `session.id`; replay segments carry rrweb events only. `GET /api/sessions/:sessionId/timeline?from=&to=&anchor=start|end` (App auth + project access, RFC 3339, window at most 3h else 422, `session_timeline.controller.go`) reads them back as the replay player's `{logs, actions, truncated}` rows: `LogRecordRepository.FindBySessionId` + `OtelSpanRepository.FindSessionClientSpans`, 2000 of each (`shared.SessionBreadcrumbLimit`), nearest `from` by default or nearest `to` with `anchor=end` (the issue event card); when one read hits the limit both are cut to the time range each returned in full, and the SDK's own events (`shared.SessionTimelineSkippedEvents`) are filtered in SQL; mapped by `BuildSessionTimeline` (control-plane events, web vitals and timings skipped; network rows carry `traceId`/`spanId` for a link into `/spans`). The lookup is by session id inside the window: ClickHouse through a `session_id` column MATERIALIZED from the attribute map plus a bloom filter (`ch/0103`-`0106`, old parts compute it on read), SQLite through expression indexes on `json_extract(..., '$."session.id"')` (`sqlite_telemetry/0029`), DuckDB by scan. The session page and the issue event card (the 10 minutes before the exception, up to 1s after it) read the timeline and merge it with the logs/actions inside recordings, which 1.x SDKs and native frames still send: `pickBreadcrumbs` (`frontend/src/lib/utils/session-timeline.ts`) drops identical rows and sorts the rest by time, and a failed timeline read leaves the recording rows alone. Whether a breadcrumb exists at all is the signal's mode: buffered breadcrumbs reach the server only with a trigger flush or a capture, so an issue's context is the error clip's lookback.

**Auth & Registration**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/login` | None | Dashboard authentication |
| POST | `/api/register` | None | New user registration |
| GET | `/api/has-organizations` | None | Check if any orgs exist (self-hosted only) |
| POST | `/api/forgot-password` | None | Request password reset email |
| GET | `/api/password-reset/:token` | None | Validate reset token |
| POST | `/api/password-reset/:token` | None | Reset password with token |

**CLI Device Auth & OAuth** (see the Authentication section above)
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/auth/device/authorize` | None | Start device flow; returns device/user code + verification URL |
| POST | `/api/auth/device/token` | None | Poll for the token (grant `device_code`); also accepts `refresh_token`. Shares a 60/min per-IP limiter with `/api/auth/token`; over it a `device_code` grant answers the RFC 8628 `400 slow_down` (`RateLimitOAuthTokenPerIP` peeks `grant_type` from the buffered body), so the shipped CLI backs off instead of aborting the login, while every other grant gets a plain `429` + `Retry-After` because `slow_down` is undefined for them and OAuth clients treat an unknown 400 as terminal. Body capped at 64KB by `BufferAuthBody` like the other anonymous auth routes |
| POST | `/api/auth/token` | None | Token endpoint: `device_code`, `refresh_token` or `authorization_code` grant (JSON or form-encoded). Same shared limiter, grant-aware rejection and 64KB body cap as the device token route |
| POST | `/api/auth/logout` | None | Revoke the presented refresh token's family (idempotent) |
| GET | `/api/device` | App | Look up a user code for the approval screen |
| POST | `/api/device/approve` | App | Approve a pending device authorization (tokens carry only the approving user's own role, so no write guard) |
| POST | `/api/device/deny` | App | Deny a pending device authorization |
| POST | `/api/oauth/register` | None | RFC 7591 dynamic client registration (rate-limited) |
| GET | `/api/oauth/client` | App | Resolve a client_id to its display name (consent page) |
| POST | `/api/oauth/approve` | App | Approve an authorization request; mints the code, returns the validated redirect |
| POST | `/api/oauth/deny` | App | Deny an authorization request; returns the error redirect |
| GET | `/.well-known/oauth-authorization-server` | None | RFC 8414 metadata (served at origin root, not `/api`) |
| GET | `/.well-known/oauth-protected-resource` | None | RFC 9728 metadata (served at origin root, not `/api`) |
| GET/POST/DELETE | `/mcp` | Bearer | Streamable HTTP MCP server (origin root), read tools only for every credential; 401s carry a `WWW-Authenticate` resource-metadata challenge. A session the client has not posted to for an hour is closed (`sessionIdleTimeout` in `services/mcpmount`, the SDK's `SessionTimeout`; without it every session and its server instance lived until restart, since most clients never DELETE): its next request answers 404 and the client initializes a new one |
| GET/POST/DELETE | `/mcp/write` | Bearer (write scope) | Same server with `mcpserver.Config.AllowWrite`: adds the mutating tools and the operations playbook. Tokens without `traceway:write` get 403 with a scope challenge, and so do dashboard sessions (`BearerIdentity.HasAgentWriteScope`), which would otherwise skip the agent gate's opt-in and audit log; resource metadata at `/.well-known/oauth-protected-resource/mcp/write` |

**Personal Access Tokens**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/personal-access-tokens` | App | Create a PAT (returns the token once). `scope` is `read` (default, `twp_`) or `write` (`tww_`, `expiresInDays` 1-90 required). Agent credentials reach it as an agent write (`?projectId=`, Agent Access, audit log) and may create read tokens only (403 for `write`), so a write credential never extends itself; `traceway-write tokens create` / MCP `create_read_token` |
| GET | `/api/personal-access-tokens` | App | List the current user's active PATs |
| DELETE | `/api/personal-access-tokens/:id` | App | Revoke a PAT |

**Projects**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/projects` | App | List projects |
| POST | `/api/projects` | App | Create project (optional `organizationId` body field targets another org; the handler checks the caller's **org role** in the target org and 403s for non-members/`readonly`; creation is org-scoped, so per-project overrides neither grant nor deny it) |
| POST | `/api/projects/source-map-token` | App+Write | Generate source map upload token |

**Dashboard**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/dashboard` | App | Dashboard metrics |
| GET | `/api/dashboard/overview` | App | Recent issues + top endpoints |
| POST | `/api/stats` | App | Homepage stats |

**Metrics**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/metrics/application` | App | Application metrics |
| GET | `/api/metrics/stats` | App | Stats metrics |
| GET | `/api/metrics/server` | App | Server metrics |
| POST | `/api/metrics/query` | App | Custom metric queries. An unknown aggregation answers 422. A metric of registry type `histogram` is read from `metric_histograms` and also answers `p50 p75 p90 p95 p99` and `good_ratio` (see "Client Metrics"); `wholeRange: true` returns one value per series for the whole range. Point metrics: aggregations avg/min/max/sum/count/last plus `rate`, which differences every series (full tag set = identity) against its previous sample, drops resets, and sums the deltas per bucket as a per-second rate; the reported unit gains `/s` (`By/s`), a counter of seconds becomes the ratio `1`, and on ClickHouse `rate` follows `selectTable` like every other aggregation: past 6h it reads the rollups through `maxMerge(max_val)` (a counter only grows, so a rollup bucket's max is its last sample), never `metric_points_1d`, with the chart bucket clamped to the rollup step and the lookback widened by it (`rateSource`). The `traceway-otel-agent` template relies on it (plus `state` filters, `groupBy: server_name`, and the per-source `complement` flag that the widget renderer applies as 1 − value). Bounded in `metric_query.controller.go`: 256KB body, max 50 queries and 20 tag filters each, range at most 731 days and `to >= from` (422), buckets widened so no series exceeds 2000 points (the effective `intervalMinutes` is returned at the top level of the response), at most 200 groups per query (the repositories return one extra so the result carries `truncatedGroups: true`), and a 30s deadline for the whole request that answers 504. The discovery endpoints validate their `from`/`to` the same way |
| GET | `/api/metrics/discover` | App | Discover available metrics |
| GET | `/api/metrics/discover/tags` | App | Discover metric tags |
| GET | `/api/metrics/discover/instances` | App | Distinct `server_name` values in a range (dashboard instance filter) |
| PUT | `/api/metrics/registry` | App+Write | Update metric registry entry |

**Dashboards & Templates**

Dashboards are org-owned JSON documents (`{schemaVersion, widgets: [{id, title, widgetType, config}]}`, widget ids are server-generated `w_xxxxxxxx` strings, array order = display order) applied to projects via `project_dashboards`. Dashboard mutations require org role above `readonly` (checked in-handler); project-scoped routes (list/star/reorder/populate) use the standard middleware chains; apply/unapply also check the effective role of each affected project.

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/dashboards` | App | Dashboards applied to the project (tab order) |
| GET | `/api/dashboards/library` | App | All dashboards across the user's orgs with applied project ids |
| POST | `/api/dashboards` | App | Create in org (auto-applies to current project unless `applyToProjectIds` given) |
| GET | `/api/dashboards/:id` | App | Meta + widgets (+ per-project `isStarred`, `appliedProjectIds`) |
| PUT | `/api/dashboards/:id` | App | Update name/description and/or full `definition` (the as-code path) |
| DELETE | `/api/dashboards/:id` | App | Delete everywhere (assignments + stars cascade) |
| PUT | `/api/dashboards/:id/apply` | App | Set the full project assignment list |
| DELETE | `/api/dashboards/:id/apply/:projectId` | App | Unassign from one project |
| POST | `/api/dashboards/:id/copy` | App | Copy (also cross-org) with optional apply |
| PUT | `/api/dashboards/reorder` | App+Write | Tab order for a project (explicit id order) |
| POST | `/api/dashboards/:id/embed-token` | App+Write | Issue an expiring share link for a dashboard applied to the project (see "Shared dashboards" below) |
| GET | `/api/embedded-dashboards/:uuid` | Embed token (rate-limited) | Name, widgets (display fields only), `from`/`to`, `expiresAt`, `expiresInSeconds` of a shared dashboard |
| GET | `/api/embedded-dashboards/:uuid/widgets/:wid` | Embed token (rate-limited) | One widget's data, in the `/api/metrics/query` response shape |
| POST | `/api/dashboards/:id/widgets` | App | Add widget |
| PUT | `/api/dashboards/:id/widgets/reorder` | App | Reorder widgets (explicit id order) |
| PUT | `/api/dashboards/:id/widgets/:wid` | App | Update widget |
| DELETE | `/api/dashboards/:id/widgets/:wid` | App | Delete widget (+ its stars) |
| PUT | `/api/dashboards/:id/widgets/:wid/star` | App+Write | Star/unstar for the project homepage |
| GET | `/api/dashboards/starred` | App | Starred widgets with homepage layout |
| PUT | `/api/starred-widgets/reorder` | App+Write | Reorder homepage starred widgets (`{ids}` = starred row ids) |
| PUT | `/api/starred-widgets/:id` | App+Write | Update homepage layout (colSpan/size) |
| GET | `/api/dashboards/:id/export` | App | Export one dashboard as JSON |
| GET | `/api/dashboards/export?organizationId=` | App | Export the org bundle |
| POST | `/api/dashboards/import` | App | Import doc/bundle (`mode: create\|upsert`, upsert matches by name) |
| POST | `/api/dashboards/import/grafana` | App | Convert a Grafana export (best-effort, returns `warnings[]`) |
| GET | `/api/dashboard-templates` | App | Marketplace list/search (`search`, `category` params) |
| POST | `/api/dashboard-templates/:key/install` | App | Copy a template into the org and apply |
| POST | `/api/dashboards/populate-defaults` | App+Write | Install the framework-default template set for an empty project |
| GET | `/api/metrics/discover/org` | App | Metric names across all org projects (command palette) |

**Shared dashboards** (`dashboard_embed.controller.go`, `services/dashboard_embed.go`, frontend `routes/embed/dashboard`, `share-dashboard-dialog.svelte`; docs `docs/pages/learn/dashboards.mdx#share-and-embed-a-dashboard`). A share link is a stateless HS256 JWT: nothing is stored, so a link cannot be revoked on its own (the default API lifetime is 5 minutes and no link outlives `embedMaxDays`, 90 days, the same bound as a write token; unassigning the dashboard from the project stops every link for that pair, since each read re-checks `FindAppliedDashboard`). The signing key is an HMAC of `jwtSecret` with the audience `traceway-dashboard-embed`, so an embed token never validates as an app session and the reverse. The claims carry the dashboard id, the project id and the signed view (`from`, `to`, `tagFilters`); the subject is `uuid.NewSHA1(projectId, dashboardId)` and must equal the `:uuid` in the path. `POST /api/dashboards/:id/embed-token` takes an optional body `{from, to, tagFilters, expiresAt, expiresInDays}` (the two expiry fields are alternatives, each at most 90 days ahead, else 422; explicit expiry with a relative timeframe resolves at viewer open and returns a signed widget token pinning that range and deadline) of at most 2 KB with unknown fields refused (400): both or neither of `from`/`to` (neither = relative `timeframe` query parameter, default last 42 hours; explicit timestamps and `timeframe` are mutually exclusive), the metric query's range limit, at most 20 filters, and no empty tag or value (422, `validateEmbedView`: an empty value is a host that failed to resolve its account, and ClickHouse's `tags[k] = ''` matches every row without the tag). It is an agent **read** route (`agent_routes.go`): a link changes nothing and shows only a dashboard that already exists, so a host application's backend holds a `twp_` read token (which can be created without an expiry) and issues a link per authorized viewer, without Agent Access and without an `agent_write_events` row. `RequireWriteAccess` still applies, so the token's owner must not be a `readonly` member. The write CLI and write MCP server issue the same link (`traceway-write dashboards embed <id>`, tool `create_dashboard_embed`, `client.CreateDashboardEmbed` with `DashboardEmbedOptions.Validate` mirroring the server's 422s; both add `url`, the `embedPath` resolved against the instance). The integration guide for a host application is the knowledge chunk `dashboard-embedding.md` (`traceway://knowledge/dashboard-embedding` on both MCP servers), which `just gen-skills` copies into `skills/traceway-setup` and the `traceway-write` skill; keep it in step with `docs/pages/learn/dashboards.mdx` when the request or its limits change. The share dialog accepts a lifetime of 1 to 90 days and a Last timeframe preset. The two read routes take the token as `Authorization: Bearer` behind `RateLimitPerIP` only (`embedOpensPerMinute` = 60 for the dashboard, `embedWidgetReadsPerMinute` = 600 for widgets, since one open reads every widget and a dashboard holds up to 100; the limiter runs before the token is checked); `GetEmbed` returns the widgets through `embedWidgets`, which keeps each config but rebuilds `sources` from the allowlist `embedWidgetSource` (`aggregation`, `groupBy`, `label`, `complement`: what the embed page draws with), so a source's metric name, saved `tagFilters` and threshold never reach the anonymous viewer, and a source field the page starts to need must be added there; the widget read still names the metric in its result, which the chart uses as the series label when the source has none; the viewer never sends a query: `embedWidgetQuery` rebuilds what `widget-renderer.svelte` would send from the stored widget (the signed filters win over a source's own, `wholeRange` and the breakdown table's count query per widget type; `TestEmbedWidgetQueryMatchesTheDashboardPage` holds the two together, so change both when a widget type changes how it queries) and runs it through `executeMetricQuery`. The page: `SecurityHeaders` leaves the two frame headers off for exactly `/embed/dashboard`, and the SPA handler serves that path from the prerendered `embed/dashboard.html` or answers 404, never the app shell, because the path may be framed. In the frontend `dashboardStorage()` returns no storage on that path (a third-party frame may have it blocked, and the page's only identity is the link), the root layout skips theme and project setup there, and `app.html` applies `?theme=`. The token rides in the URL fragment; the host renews by assigning a new `src`, which is a fragment navigation (`hashchange`), and the page swaps the dashboard and its token-bound loader in one step so the frame never blanks. Expiry on the page is timed from `expiresInSeconds`, never the viewer's clock. Exemplar lists are not shown in an embed.

Templates are DB rows seeded by migrations (`traceway-otel-agent` for the OTel host agent, `golang` for Go SDK apps, `browser-performance` for browser projects, `traceway-clickhouse`/`traceway-duckdb` for the telemetry stores of a monitored Traceway instance; SQLite emits no store-specific metrics so it has no template); cloud can insert more rows without a release. The OTLP metric ingest allowlists per-resource identity and grouping tags (`host.name`, `host.id`, `os.type`, `cloud.region`, `container.name`, `k8s.cluster.name`, `k8s.pod.name`, `k8s.node.name`, `postgresql.database.name`, ...) in `otelcontrollers/metric_converter.go` for infrastructure views and custom widgets.

**Endpoints**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/endpoints` | App | List all endpoints |
| POST | `/api/endpoints/grouped` | App | Endpoint aggregates (P50/P95/P99) |
| POST | `/api/endpoints/endpoint` | App | Single endpoint details |
| POST | `/api/endpoints/chart` | App | Stacked chart data |
| GET | `/api/endpoints/slow` | App | Get slow endpoint threshold |
| POST | `/api/endpoints/slow` | App+Write | Set slow endpoint threshold |
| POST | `/api/endpoints/:endpointId` | App | Endpoint detail view |

**Tasks**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/tasks` | App | List all tasks |
| POST | `/api/tasks/grouped` | App | Grouped by task name |
| POST | `/api/tasks/task` | App | Single task details |
| POST | `/api/tasks/:taskId` | App | Task detail view |

**Exceptions**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/exception-stack-traces` | App | Grouped exception list |
| POST | `/api/exception-stack-traces/archive` | App+Write | Archive exceptions |
| POST | `/api/exception-stack-traces/unarchive` | App+Write | Unarchive exceptions |
| POST | `/api/exception-stack-traces/by-id/:exceptionId` | App | Single exception by ID |
| POST | `/api/exception-stack-traces/:hash` | App | Exception by hash |

**AI Traces & Conversations**

One `ai_traces_v2` row per LLM call (any OTLP span with `gen_ai.*` attributes). Each row carries a `conversation_id` resolved at ingest (`gen_ai.conversation.id` -> `session.id` span/resource attr -> trace id), `tool_call_count`/`tool_names` parsed from the completion payload (OpenAI `tool_calls`, Anthropic `tool_use`, OTel output messages, `gen_ai.tool.*` fallback), and `flagged`/`flagged_terms` from an ingest-time word-boundary scan of prompt+completion against per-project selected built-in language packs (`projects.ai_flagged_languages`, default `["en"]`; packs live in `backend/app/services/contentflag/terms/*.txt` — en/de/es/fr/it/pt/sr; empty array = custom terms only) plus per-project custom terms (`projects.ai_flagged_terms`); both edited in the project settings AI tab and cached via the project cache. All three project-creation paths (`Create`, `CreateWithOrganization`, `cmd/seed.go`) must set `AiFlaggedLanguages` explicitly since lit inserts every struct field. `tool_names`/`flagged_terms` are stored comma-separated (values sanitized at ingest); the content matcher lives in `backend/app/services/contentflag/`. Conversation analytics exclude rows with an empty `conversation_id`; user analytics additionally require a non-empty `user_id`.

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/ai-traces/grouped` | App | AI traces grouped by trace name |
| POST | `/api/ai-traces/trace` | App | Calls for one trace name (`?traceName=`) |
| POST | `/api/ai-traces/:traceId` | App | Single call detail + conversation blob |
| POST | `/api/ai-conversations/grouped` | App | Conversations (GROUP BY conversation_id): turns, cost, tokens, tools, models, flagged; filters userId/model/toolName/flaggedOnly/search (search matches conversation id, user, model, tool names, and flagged terms; row-level filters are semi-joins on conversation_id so a match on any turn returns the whole conversation's aggregates); response also carries `thresholds` (range-wide P95 cost/turns for outlier highlighting) and `facets` (models, tools) |
| POST | `/api/ai-conversations/conversation` | App | All turns of one conversation (id in body) ordered by recorded_at, each with its stored input/output payload (capped at 200 turns of payloads), plus stats |
| POST | `/api/ai-users/grouped` | App | Per-user conversation analytics: conversation count, total calls, avg/min/median turns, avg cost per conversation, total cost, flagged conversation count |

Frontend routes: `/ai-traces` (tabs: Traces, Conversations, Users), `/ai-traces/conversations/[conversationId]` (chat timeline with tool calls rendered). Trace names `conversations` and `users` are shadowed by these static routes. Notification rule types `ai_trace_cost` (per call), `ai_conversation_cost` (24h cumulative per conversation), and `ai_flagged_content` (flagged term match, optional term filter) are event-driven; remember both `notification_rule.repository.go` copies list event rule types explicitly.

**Remote SDK Config** (OpAMP; `app/sdkconfig/` pure package, `app/cache/sdk_config_cache.go`, `controllers/opampcontrollers/`, `controllers/sdk_config.controller.go`, dashboard: the `/data-collection` page (`routes/data-collection`, component `sdk-config/data-collection.svelte`, sidebar item with the Upload icon shown for `SDK_CONFIG_FRAMEWORKS`), plus Capture buttons on the session page and the issue event card; docs `docs/pages/learn/remote-config/` (architecture, setup, settings reference), draw.io sources of its diagrams in `diagrams/docs/remote-config/`)

The OpenTelemetry browser SDK (`@tracewayapp/frontend` 2.x, package `@tracewayapp/opamp`; the docs install it without a tag and pin CDN URLs to `@2`, see `docs/AGENTS.md`) polls `POST /api/otel/v1/opamp` with the project token, speaking OpAMP's plain-HTTP transport (protobuf `AgentToServer` / `ServerToAgent`, types from `github.com/open-telemetry/opamp-go/protobufs` only; `opamp-go/server` is deliberately not used: its plain-HTTP path needs `http.Server.ConnContext` and CORS preflights fall into its WebSocket branch). The handler evaluates the project's rules against the agent description in the poll itself, so any replica answers any client and nothing is stored per client. The one write: a poll from a client matched by an active capture, and an OTLP batch carrying `traceway.capture.ids` (`markCaptureDataReceived`), note the stage (`delivered_at`/`applied_at`/`received_at`) with `captureprogress.Mark`, which returns at once: one goroutine per process writes the noted stages in a batched main-DB transaction (`SdkConfigRepository.MarkCaptureProgress`, which only fills an empty column) and remembers what it wrote for 25h (bounded), so each stage is written about once per instance and no poll or ingest request waits on the main DB for it. A failed batch is dropped and comes back with the next report. A poll without a description (a compressing client such as opamp-go's own) gets `ReportFullState`; the Traceway SDK always sends full state. The answer carries `remote_config.config_map["traceway"]` (`application/json`) only when the client's `last_remote_config_hash` differs, plus `connection_settings.opamp.heartbeat_interval_seconds`. 415 for anything but `application/x-protobuf` (no WebSocket), 413 over 64 KB (raw or gunzipped; clients must not retry 413), 400 for a malformed message or an `OpAMP-Instance-UID` header that does not match `instance_uid`, 503 + `Retry-After` when the rules cannot load. `opampcontrollers/opamp_sqlite_test.go` drives the handler with opamp-go's own HTTP client. The poll flattens the agent description with identifying attributes first (they win a key conflict; non-identifying ones only fill what is left of the 128-attribute cap); an array value keeps only its scalar items and is cut off at 1024 bytes while it is built. The path is skipped by self-monitoring (`middleware.SelfMonitoring`).

- **Settings document** (`sdkconfig.Settings`, `schemaVersion: 1`): `enabled` (kill switch), `signals.{errors,messages,network,console,navigation,userActions,webVitals,resourceTiming,longTasks,routeTiming,custom}.{mode,sampleRate,minSeverity}` (a new signal is added in `sdkconfig/settings.go`: `signalNames`, the `Signals` struct and `each`; in the dashboard's `SIGNALS` in `sdk-config.ts`; and in the SDK's `SIGNALS`; `longTasks` = `browser.long_task` and `routeTiming` = `browser.route_timing` are measured by the SDK, off by default, and left out of the session timeline like the other page metrics) where mode is `off | buffer | stream` (buffer = kept on the device until a trigger or capture uploads it), `sessions.enabled`, `replay.{mode,sessionSampleRate,maskAllInputs}`, `buffer.{maxBytes,maxAgeSeconds,replayMaxBytes,persist}`, `triggers[]` (on-device multi-step matchers; a step may need `count` matching records within a sliding `periodSeconds`; actions `flush`/`stream`/`startReplay`/`emit` and `capture`, which uploads `lookbackSeconds` and then streams for `durationSeconds`, persisted on the device across page loads and reported as `traceway.capture.started` with `traceway.capture.source: trigger`; a trigger without steps patches an earlier one by id, e.g. `{"id":"default-exception","enabled":false}` switches the SDK's built-in error clip off). `captures[]` (added by the server only) and `limits` (written by the server only, see below). Log-line triggers match `body` of records with `signal eq console` using `icontains`, `starts_with` or `glob` (`*`/`?`, case-insensitive, one backtrack point, so at worst text length times pattern length); there is deliberately no regex operator, since a user-supplied pattern would run with backtracking on the page's main thread. Rules hold partial documents: the client receives the overlay of every matching enabled rule in position order (later wins), never the SDK defaults, so the defaults stay in the SDK; with no match the document is `{"schemaVersion":1}`. The SDK clamps everything to the ceilings its `init()` options set: remote config can only restrict. `ParseSettings` rejects unknown fields (422), the document is capped at 16 KB, 20 triggers, 5 steps. A matcher is a condition with a key or exactly one of `all`/`any` (non-empty) and `not`; `values` cannot hold null; an unquoted attribute value in a click/hover selector must be a CSS identifier (`[data-step="2"]`, not `[data-step=2]`, which `element.matches` rejects); `metrics.sampleRate` is 0 or at least 0.001 (`MinMetricsSampleRate`, the inverse of ingest's weight cap). An action's `signals` is `omitzero`, not `omitempty`: `[]` means no signals (a replay-only clip) and must survive storage, because the SDK reads a missing list as all of them. `canonicalRule` parses the re-encoded bytes again before saving, so a document that would be skipped at load cannot be stored. On the device a trigger follows every way its sequence could be under way (`TriggerEngine` in js-client `core/src/triggers/engine.ts`: per step, when the steps before it last completed), so the first step happening again does not discard progress; the dashboard gives a template an id no other rule of the project uses (`withTrigger`, `triggerIdsOfRules` in `sdk-config.ts`), since a client matching two rules runs one trigger per id, and the "Error clip" template carries the built-in id `default-exception` so it replaces the SDK's trigger instead of doubling it. `config_hash` is the sha256 of the rendered JSON (struct field order is fixed). The organization's `IngestPermission` is folded in (`sdkconfig.ApplyPermission`): what the plan would drop is switched off at the source, and the same signals go into `limits` (`{"signals":{"errors":"off"},"replay":"off"}`), which the SDK treats like its `init()` ceilings, so no trigger or capture brings them back. A rule containing `limits` or `captures` is a 422.
- **Targeting** (`sdkconfig.Conditions`): `{"all":[{key,op,value|values}], "rollout":{"percent","by"}}` over the agent description's identifying + non-identifying attributes (the SDK sends its resource, `session.id` and every global attribute such as `user.id`). Ops `eq neq in not_in contains icontains starts_with glob exists not_exists gt gte lt lte version` (`sdkconfig.GlobMatch` mirrors the SDK's matcher); `version` takes comparators like `>=2.4 <3` and compares dotted numbers (four-part browser versions work), the same grammar the SDK uses for trigger conditions; `gt`/`gte`/`lt`/`lte` take a number: a numeric string such as `"120"` is saved as the number, anything else is a 422. The rollout bucket is `sha256(ruleId‖instance uid or the named attribute) % 10000`, sticky per browser profile and monotonic when the percentage grows.
- **Captures**: `POST /api/sdk-config/captures` targets one device (`service.instance.id`), user, session or attribute for 1-1440 minutes (max 20 active per project). Matching clients run the lookback flush once (remembered across pages) and stream until the end. A capture (dashboard or the `capture` trigger action) also brings back signals rules switched off: its `"*"` means every signal the SDK collects by default (resource timing, long tasks and route timing, off by default, must be listed), still within the `init()` ceilings, the plan `limits` and the `enabled` kill switch, and the record that fired a capture trigger is sent too even when its signal was off. The plain `stream` action keeps to signals that are not off; a client under a capture is told a 60s heartbeat. They pick it up at the next poll or when the tab regains focus, there is no push.
- **Cache**: `cache.SdkConfigCache` compiles a project's rules and all of its active (unexpired, uncancelled) captures, with no row limit since a project has at most 20 active captures (`loadSdkConfig` through `ListActiveCaptures`); expiry is still checked again at evaluation, so a lapse changes the hash without invalidation, 2 min TTL, singleflight loads, invalidated with `middleware.OnCommit` on every write and, on Postgres, on every replica through `pg_notify('sdk_config_changed')` (own `pq.Listener`; a reconnect drops everything).

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/otel/v1/opamp` | Client | OpAMP plain-HTTP poll (see above) |
| GET | `/api/sdk-config/rules` | App | Rules in position order + `heartbeatIntervalSeconds` |
| POST | `/api/sdk-config/rules` | App+Write | Create (`{name, enabled, conditions, settings}`, re-encoded canonically; max 50) |
| PUT | `/api/sdk-config/rules/reorder` | App+Write | `{ids}` must list every rule once (else 422) |
| PUT/DELETE | `/api/sdk-config/rules/:id` | App+Write | Update / delete |
| POST | `/api/sdk-config/preview` | App | `{attributes, instanceUid?}` -> the document such a client gets, its hash and the matched rules; reads the DB, not the cache; `heartbeatIntervalSeconds` is what that client is told (60 under an active capture, `sdkconfig.Result.HeartbeatSeconds`) |
| GET/POST | `/api/sdk-config/captures` | App / App+Write | Up to 50 captures (`captureListLimit`): the active ones first, then the most recent expired and cancelled ones, each with `active` and a `status` (Waiting for client ... Collecting, Expired · no data received, Completed, Cancelled) / start one |
| DELETE | `/api/sdk-config/captures/:id` | App+Write | Cancel |

**Organization Management**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/organizations` | App | Create an organization owned by the caller; the recovery path for a user removed from their last organization (self-hosted: 422 once one exists; cloud: `OrganizationLimitHook`). Runs `PostRegistrationHooks`, then `PostRegistrationRequestHooks` (the same, plus the `*gin.Context`, for extensions that need the request's cookies or headers), like register and SSO finish-setup |
| GET | `/api/organizations/:orgId/settings` | Admin | Get org settings |
| PUT | `/api/organizations/:orgId/settings` | Admin | Update org settings |
| PUT | `/api/organizations/:orgId/agent-access` | Admin | `{agentWriteEnabled}`: allow agent write credentials to write in this org (settings GET returns the flag) |
| GET | `/api/organizations/:orgId/agent-activity` | Admin | Paginated agent write log (who, credential, project, route, status) |
| GET | `/api/organizations/:orgId/members` | Admin | List members |
| PUT | `/api/organizations/:orgId/members/:userId` | Admin | Update member role |
| DELETE | `/api/organizations/:orgId/members/:userId` | Admin | Remove member |
| GET | `/api/organizations/:orgId/members/:userId/project-roles` | Admin | List member's per-project role overrides |
| PUT | `/api/organizations/:orgId/members/:userId/project-roles/:projectId` | Admin | Set/clear a per-project role override |

**Organization Overview** (frontend routes `/organization` = Servers, `/organization/issues`, `/organization/monitors`, `/organization/on-call`, `/organization/projects`)

Org-wide read views that fan out over every project of the org, in `organization_overview.controller.go`. `RequireOrganizationAccess` (any org role) gates them; the servers/issues/monitors handlers deliberately skip `Transactional` because they run per-project telemetry queries and must not hold the single-connection SQLite main DB across them (they open their own short `db.ExecuteTransaction` for the project list first). The pure main-DB pages endpoint keeps `Transactional`. The frontend renders the organization routes in the same sidebar shell as project pages, with the sidebar switched to organization items (Servers, Issues, Monitors, On-Call, Projects under an "Organization" label; badges come from `/overview/counts`). The selected organization is the shared `organizationContext` in `frontend/src/lib/state/organization-context.svelte.ts` (the `organizationId` query param, else the first membership), and `src/routes/organization/+layout.svelte` owns the redirects: a single-project org into that project, legacy `?tab=` links to the new paths, and frontend-only orgs from Servers/Monitors to Issues. Login auto-lands on `/organization` when the first org has more than one project (`frontend/src/lib/utils/landing.ts`). A single-project organization never shows the view: `/organization` redirects into that project and the switcher renders the org as a plain label, since the project's own pages already cover everything it would aggregate. When none of the org's projects is a backend project (`isBackendFramework` in `projects.svelte.ts`, i.e. every project is a browser or mobile framework), the Servers and Monitors sidebar items are hidden and `/organization` redirects to `/organization/issues`.

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/organizations/:organizationId/overview/servers` | Member | One row per `server_name` per project over the last 30 min: CPU/memory/disk/network + a CPU sparkline, plus host/os/cloud/k8s metadata lifted from metric tags. Reads OTel hostmetrics (`system.cpu.utilization` state=idle inverted, `system.memory.utilization` state=used, `system.filesystem.utilization` max over `mode=rw` mounts, else over every mount (the metric has no `state`; read-only snap mounts always read 100%), `system.network.io` by direction) and falls back to the legacy SDK names (`cpu.used_pcnt`, `mem.used`/`mem.total`). A project that fails to read sets `partial: true` instead of failing the request |
| POST | `/api/organizations/:organizationId/overview/issues` | Member | Paginated issues across all projects. Body mirrors `/api/exception-stack-traces` (`fromDate`, `toDate`, `orderBy`, `search`, `searchType`, `pagination`); every project contributes its top `page*pageSize` groups in the requested order and the page is a slice of their merged order, so it equals what one combined project would return. That per-project fetch is capped at 1000 rows (`orgOverviewMaxIssueFetch`): a deeper page answers 422 and `totalPages` is clamped so the footer never offers one; `FindGrouped` orders ties by `exception_hash` on every backend so the merge is stable across pages. Returns `{data, pagination, partial}` with 24h hourly trends attached; a project that fails to read sets `partial: true` like the servers endpoint |
| POST | `/api/organizations/:organizationId/overview/pages` | Member | Paginated on-call pages across all projects: body `{status, search, fromDate, toDate, pagination}` (status active/open/acknowledged/resolved, search over subject + rule name, range on `created_at`); returns `{data, pagination, openPagesCount}` |
| GET | `/api/organizations/:organizationId/overview/counts` | Member | `openPagesCount` + `downMonitorsCount` for the organization sidebar badges |
| POST | `/api/organizations/:organizationId/overview/incidents` | Member | Paginated incidents across monitors and status pages: body `{search, fromDate, toDate, pagination}`; an incident matches when it overlaps the range (`started_at <= to` and unresolved or `resolved_at >= from`), search covers title, monitor, status page, and error message |
| GET | `/api/organizations/:organizationId/overview/monitors` | Member | Every synthetic check in the org with 30-day aggregates; a project whose results fail to read sets `partial: true` like the servers endpoint |

Instance identity is the `server_name` tag, which comes from the OTLP `service.name` resource attribute; grouping by Kubernetes cluster keys off the `k8s.cluster.name` tag and the group selector only offers it when at least one instance carries one. Clicking a row opens `/dashboards?projectId=&server=&preset=30m` (plus `dashboard=` when the project has the `traceway-otel-agent` template applied); `server` scopes every widget query via `scopeTagFilters` in `widget-grid`/`widget-renderer`. Cluster-wide instrumentation manifests live in `examples/kubernetes/`, documented at `docs/pages/learn/kubernetes.mdx`.

**Invitations**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/organizations/:orgId/invitations` | Admin | Send invitation |
| GET | `/api/organizations/:orgId/invitations` | Admin | List invitations |
| DELETE | `/api/organizations/:orgId/invitations/:id` | Admin | Revoke invitation |
| GET | `/api/invitations/:token` | None | Get invitation info |
| POST | `/api/invitations/:token/accept` | None | Accept (new user) |
| POST | `/api/invitations/:token/accept-existing` | App | Accept (existing user) |

**On-Call** (teams, schedules, escalation policies, pages)

Org-scoped entities in the main DB. Teams (`teams`/`team_members`) own projects one-to-one (`project_teams`, unique on project_id). Schedules (`oncall_schedules`) store PagerDuty-style calendar layers as a JSON `definition` document (rotations daily/weekly/custom, handoff time/day, time-of-day and day-of-week restrictions); one-off overrides are normalized rows (`oncall_overrides`). The pure resolution engine lives in `backend/app/oncall/` (`ResolveRange`/`ResolveAt`, tz-aware calendar math, later layer wins, overrides trump all). Both apply the same stacking, so a schedule puts exactly one person on call at any instant: `ResolveAt` is `ResolveRange` over a single instant, and paging a whole schedule stack (waking the person an override was meant to relieve) is the bug it exists to prevent. Escalation policies (`escalation_policies`) hold JSON steps (`targets` schedule/user/team/channel + `delayMinutes`, `repeatCount`); rules page on-call via the `escalation` notification channel type (config `{"policyId"}`), special-cased in `notifications/dispatch.go` through `RegisterPageOpener` (wired in `cmd/run.go`). A fired rule opens a `pages` row (dedup key `ruleId|dedupToken` with a partial unique index while unresolved; rules without a dedup token dedup at rule level; refires bump `event_count` and never reset the escalation clock). The escalator worker (`oncall/escalator.go`, `ONCALL_POLL_SECONDS`) claims due pages in a transaction (pg advisory lock 824737002 for multi-instance), inserts `page_notifications` rows, resolves targets to users, delivers via each user's `user_contact_methods` (email/slack/pushover/telegram adapter configs; account-email fallback always), then escalates level by level until ack/resolve or exhaustion. `RequireOrganizationAccess` middleware (any org role) gates member-level reads; mutations are org-admin; page acknowledge deliberately requires no write access.

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET/POST | `/api/organizations/:organizationId/teams` | Member / Admin | List (with members+projects) / create |
| PUT/DELETE | `/api/organizations/:organizationId/teams/:teamId` | Admin | Update / delete |
| PUT | `.../teams/:teamId/members`, `.../teams/:teamId/projects` | Admin | Replace ordered members / owned projects |
| GET/POST | `/api/organizations/:organizationId/schedules` | Member / Admin | List / create |
| GET/PUT/DELETE | `.../schedules/:scheduleId` | Member / Admin / Admin | Detail+overrides / whole-document update / delete |
| GET | `.../schedules/:scheduleId/timeline?from=&to=` | Member | Rendered per-layer + final shifts (max 62 days) |
| POST/DELETE | `.../schedules/:scheduleId/overrides(/:overrideId)` | Member | Create (any member, max 30d) / delete (creator, covered user, or admin) |
| GET | `/api/organizations/:organizationId/oncall/now` | Member | Overview: per team/schedule current + next on-call |
| GET | `/api/oncall/current?projectId=` | App | Owning team + current on-call for a project (issue page) |
| GET | `/api/escalation-policies` | App | Policies of the project's org (channel dialog picker) |
| GET/POST | `/api/organizations/:organizationId/escalation-policies` | Member / Admin | List / create |
| PUT/DELETE | `.../escalation-policies/:id` | Admin | Update / delete (422 while referenced by a channel) |
| POST | `/api/pages` | App | List (POST-body: status open/acknowledged/resolved/active + pagination) |
| GET | `/api/pages/:id` | App | Detail + delivery log |
| POST | `/api/pages/:id/acknowledge` | App (no write gate) | open -> acknowledged, stops escalation; 409 if not open |
| POST | `/api/pages/:id/resolve` | App | open/acknowledged -> resolved; 409 if already resolved |
| GET | `/api/pages/open-count` | App | Sidebar badge count |
| GET/POST | `/api/contact-methods` | App | Own contact methods (self-scoped). Types: email, slack, pushover, telegram, sms. SMS requires Twilio (`TWILIO_ACCOUNT_SID` + `TWILIO_AUTH_TOKEN` + one of `TWILIO_FROM_NUMBER` / `TWILIO_MESSAGING_SERVICE_SID`); without them SMS is not offered at all — the list response carries `smsEnabled: false` so the type picker hides it; create, re-point, resend-code and test answer 422; the escalator drops existing sms methods before its "no methods left" check so those users fall back to the account email; and the adapter errors instead of reporting a delivery nobody received. Disabling and deleting a leftover sms method stay available. Creating/re-pointing an sms method starts code verification; unverified numbers are never paged |
| PUT/DELETE | `/api/contact-methods/:id` | App | Update (incl. enabled toggle) / delete |
| POST | `/api/contact-methods/:id/test` | App | Send a canned test through one method (422 for unverified sms) |
| POST | `/api/contact-methods/:id/verify` | App (rate-limited) | Confirm the 6-digit SMS code (hashed at rest, 10-min expiry, 5-attempt cap). Deliberately **not** under `middleware.Transactional`: a wrong code answers 422, which would roll the consumed attempt back, so the handler manages its own transactions (nesting one under the middleware would also deadlock the single-connection SQLite main DB) |
| POST | `/api/contact-methods/:id/resend-code` | App (rate-limited) | Re-issue the verification code |
| GET/PUT | `/api/user-notification-rules` | App | Per-user notification-rule chains `{high: [{contactMethodId, delayMinutes}], low: [...]}` (PagerDuty-style: the page's urgency picks the chain; steps are enqueued at claim time as scheduled outbox deliveries and cancelled on ack; no chain = all enabled+verified methods immediately). Escalation policies carry `urgency: auto\|high\|low` in their definition (auto: critical -> high); pages store the resolved urgency |
| GET/POST | `/api/ack/:token` | None (rate-limited) | Tokenized no-login acknowledge: per-delivery `twk_` tokens (SHA-256-hashed on page_notifications), GET = read-only summary (scanner-safe), POST = idempotent ack recorded as `acknowledged_via='link'` attributed to the delivery's recipient; 404 after resolve. Frontend page: `/ack/[token]` |

**Monitors** (user-facing name for synthetic uptime checks; engine in `backend/app/synthetics`, frontend under `/monitors`, see the `SYNTHETICS_*` variables in `dev-docs/environment.md`)

Checks (`synthetic_checks`, main DB) are http/tcp/browser probes with per-check interval, timeout, and a consecutive-failure threshold (flap damping). Runs flow through the `check_runs` queue (outbox-style guarded claims, terminal rows deleted, expired queued runs recorded as `missed`); results are telemetry (`check_results`, all three backends, pruned by the SQLite retention worker / 90d CH TTL). State transitions open/resolve `check_incidents` and feed the event-driven notification rule type `check_down` (recovery auto-resolves the page a rule-scoped dedup key opened via `oncall.AutoResolveByDedupKey`; recovery never dispatches to escalation channels). The recovery notice carries `Recovered` on `NotificationMessage`: `dispatch` leaves its `info` severity alone instead of applying the rule's, and the Slack adapter colours it green. The notify hook runs post-commit with no ambient tx (SQLite single-connection). Remember: `notification_rule.repository.go` (both copies) lists event rule types explicitly, and `check_down` is one of them.

**Incidents, incident updates & post-mortems.** `check_incidents` covers both auto and manual incidents: auto rows carry `check_id`+`project_id` (nullable), hand-recorded ones carry only `status_page_id` (org admins record them from a status page for outages monitors missed; they never affect uptime numbers). All incidents can be given a public `title` and statuspage.io-style timeline updates (`incident_updates`: status investigating/identified/monitoring/update/resolved + message; auto open/resolve seeds empty-message investigating/resolved updates in the same tx in `synthetics/result.go`, and the probe `error_message` stays internal, never in the public payload). A `resolved` update closes a manual incident; auto lifecycle stays owned by `ProcessOutcome` (manual resolve/time-edit/delete of auto incidents answer 422). The public `/api/status/:slug` payload carries `pastIncidents` (90d union of the page's checks' auto incidents + its manual ones, titles defaulting to "<check name> is down", nested updates, cap 30, no ids/error messages); incident mutations invalidate every status-page cache slug of the org. Post-mortems (`post_mortems`, **project-scoped**: `project_id` NOT-NULL-in-practice with ON DELETE CASCADE) are internal markdown documents, never public: optional 1:1 incident link (`incident_id` unique where set, `ON DELETE SET NULL` so documents survive monitor/incident deletion; linkable incidents are the project's own autos plus the org's manual status-page incidents — cross-project autos answer 422), JSON-array `tags` (models.StringSlice), LIKE search over title/content/tags, and an append-only edit log (`post_mortem_events`: action created/updated + JSON `changes` field list, recorded only when something actually changed; served as the Activity panel). Routes are project-scoped (`/api/post-mortems*`, standard RequireProjectAccess/RequireWriteAccess chains, org resolved from the project for incident validation); incident mutations are org-admin since they are public content. Frontend: Post-Mortems tab on `/monitors` (all members; creation is a title-only dialog that creates the document immediately and opens the editor page), full-page WYSIWYG editor at `/monitors/post-mortems/[id]` (`@milkdown/crepe`, dynamically imported, app font stack overrides the theme's serif), paginated incident management page at `/monitors/status-pages/[pageId]/incidents` (which owns the Record Incident dialog).

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET/POST | `/api/synthetics/checks` | App / App+Write | List / create checks (422 validation incl. browser-mode + cloud gates) |
| GET/PUT/DELETE | `/api/synthetics/checks/:id` | App / +Write / +Write | Detail+incidents / update (type immutable; pausing drops queued runs) / delete (auto-resolves the check's open pages) |
| POST | `/api/synthetics/checks/:id/run` | App+Write | Run now (422 if a run is already queued; OnCommit wake) |
| POST | `/api/synthetics/overview` | App | Checks + per-range aggregates (uptime, avg latency) |
| POST | `/api/synthetics/checks/:id/results` | App | Paginated run history (telemetry read, no Transactional; optional `status` filter up/down/missed + fromDate/toDate) |
| POST | `/api/synthetics/checks/:id/series` | App | Bucketed uptime/latency series (telemetry read) |
| GET | `/api/synthetics/screenshot?key=` | App | Streams a failure screenshot; key prefix-checked against the project |
| GET | `/api/synthetics/output?key=` | App | Streams a failed browser run's stored Playwright output (.log keys only, same prefix check); "logs" link in the run history UI |
| GET | `/api/synthetics/open-count` | App | Down-check count for the sidebar badge |
| POST | `/api/runners/poll` | Runner (shared secret) | Instance-wide long-poll claim of browser runs (25s hold on the wake channel, NO Transactional, excluded from self-monitoring tracing); self-registers the runner's liveness row |
| POST | `/api/runners/results/:runId` | Runner (shared secret) | Report an outcome (MaxBytesReader 6MB incl. screenshotBase64; idempotent: lost/missing claim = 200 no-op; claims keyed "runner:<name>") |
| GET/POST/PUT/DELETE | `/api/organizations/:organizationId/status-pages(/:id)` | Member / Admin | Status page CRUD (slug `[a-z0-9-]{3,60}` unique, checkIds validated against the org; branding fields `description`, `customDomain` unique hostname) |
| POST | `/api/organizations/:organizationId/status-pages/:id/logo` | Admin | Upload a PNG/JPEG logo (raw body, 1MB cap, sniffed content type; SVG rejected as scriptable) stored via storage.Store under `statuspages/<id>/logo` |
| GET | `/api/status/:slug/logo` | None (rate-limited) | Streams a public page's logo |
| GET | `/` on a custom domain | None | Status domains (CNAMEd vanity hosts, TLS at the operator's proxy). The SPA handler in `cmd/run.go` looks the Host up with `controllers.StatusPageForHost` (public pages only, never the `APP_BASE_URL` host) and, on a match, serves `index.html` with a `traceway-status-slug` meta tag plus the page's title and description. The frontend reads it in `$lib/status-domain.ts` and the root layout renders only `StatusPageView` at `/`, whatever the auth state. Every other page path on that host 302s to `/`, so the dashboard never loads on a customer's domain; static assets and `/api` still work. Saving a custom domain equal to `APP_BASE_URL`'s host or the admin's current host answers 422 |
| GET | `/api/status/:slug` | None (rate-limited) | Public status payload: per-check status + 90 daily uptime buckets + incidents + `pastIncidents` (date-grouped titles with timeline updates); cached ~30s per slug; private/unknown slugs are both 404; latency values and internal error messages stripped. Frontend page: `/status/[slug]` (in `isPublicPath`) |
| GET | `/api/organizations/:organizationId/incidents` | Member | Org incidents (90d, joined check/status-page names, updatesCount, postMortemId) |
| GET | `/api/organizations/:organizationId/incidents/:incidentId/updates` | Member | One incident + its timeline updates |
| GET | `/api/organizations/:organizationId/status-pages/:id/incidents` | Member | Paginated incident history of one status page (auto incidents of its checks + its manual ones, no time window; standard pagination envelope + `statusPage` meta). Frontend page: `/monitors/status-pages/[pageId]/incidents` |
| POST | `/api/organizations/:organizationId/status-pages/:id/incidents` | Admin | Record a manual incident on a status page (title required, optional first update message, backdatable, optional resolvedAt) |
| PUT/DELETE | `/api/organizations/:organizationId/incidents/:incidentId` | Admin | Edit title (any incident) / times (manual only, empty resolvedAt reopens; 422 for auto) / delete (manual only, 422 for auto) |
| POST/DELETE | `/api/organizations/:organizationId/incidents/:incidentId/updates(/:updateId)` | Admin | Post / delete a public timeline update (a `resolved` update also closes an unresolved manual incident) |
| GET/POST | `/api/post-mortems` | App / App+Write | Paginated list for the project (query params search/page/pageSize + repeatable `tag` params ANDed together) / create in the project (422 on duplicate or cross-project incident link) |
| GET/PUT/DELETE | `/api/post-mortems/:id` | App / +Write / +Write | Full document with author names (contentMd) / update (records a `post_mortem_events` diff row; no-op saves change nothing; an optional `expectedUpdatedAt` answers 409 when the document changed since that read; the row is locked first (`LockForUpdate`) so concurrent Postgres updates cannot both pass the check. The CLI and MCP send the revision the caller's edit is based on, required when they replace the body (`--expected-updated-at` / `expected_updated_at`), and fall back to their own fresh read for title, tag and incident-only edits) / delete |
| GET | `/api/post-mortems/:id/activity` | App | Edit log for the Activity panel (who created/edited what, joined user names) |

**SLOs** (service level objectives; worker `backend/app/slo`, controller `slo.controller.go`, telemetry table `slo_buckets`, main-DB tables `slos` and `notification_rule_states`; docs `docs/pages/learn/slos.mdx`)
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET/POST | `/api/slos` | App / App+Write | List (snapshot columns included, disabled ones too) / create. `sliKind` is endpoint, span, task, monitor, metric or session with a per-kind `sliConfig` (validated and canonicalized in `validateSLIDefinition`), `targetPercent` in (0,100), `windowDays` 7, 30 or 90, `bucketMinutes` only for metric (1, 5, 15 or 60; other kinds are forced to 1). Create and update call `OnCommit(slo.Wake)` |
| GET/PUT/DELETE | `/api/slos/:id` | App / +Write / +Write | `{slo, rules}` (the project rules of type `slo_*` whose config names it) / update (a change of kind, config or slice length resets the snapshot, and the worker deletes the old buckets on its next tick; target and window changes keep the history) / delete (auto-resolves pages of referencing rules through the `slo:<id>` dedup token, deletes those rules, deletes the buckets post-commit) |
| POST | `/api/slos/:id/series` | App | `{buckets: [{bucket, good, total}]}` re-bucketed to `intervalMinutes` for `{fromDate, toDate}` (max 91 days); telemetry read, no Transactional, scoped by project through the bucket key |
| POST | `/api/slos/preview` | App | `{good, total, sli, from, to}` of a definition over the last 24h straight from raw telemetry (the sheet's Preview button) |
| GET | `/api/slos/breached-count` | App | Sidebar badge |

Agents reach every SLO route: the reads and `preview` with any scope, create/update/delete with the write scope (`agent_routes.go`). The CLI and MCP surface is the `slos` resource in `client.AlertingOperations()` (`slos list/show/series/preview` in both editions, `create/update/delete` in `traceway-write`; tools `list_slos`, `get_slo`, `get_slo_series`, `preview_slo`, `create_slo`, `update_slo`, `delete_slo`), and the write skill's reference is `knowledge/slo-configuration.md`.

**Logs**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/logs` | App | List/search logs (filters: severity, service, trace, body, `attributeFilters` with `exclude`/`contains`/`exists`) |
| POST | `/api/logs/histogram` | App | Volume chart counts per bucket and level |
| POST | `/api/logs/attribute-series` | App | One numeric attribute (`attribute: {scope, key}`) over time: per bucket count, sum, avg, min, max, p50/p95/p99, plus `present`/`numeric` counts |
| POST | `/api/logs/attribute-distribution` | App | The same attribute in round bins (`shared.NiceChartBins`, whole-number bins for whole numbers) with its overall statistics |
| GET/POST | `/api/saved-filters` | App / App+Write | Saved filters of a page (`?page=logs` or `?page=spans`) / create (`{page, name, query}`) |
| PUT/DELETE | `/api/saved-filters/:id` | App+Write | Rename and/or replace the query / delete |

Log filters and charts (`log.controller.go`, `saved_filter.controller.go`, `chart.go`, frontend `routes/logs`, `components/trace-logs/`, the chart components in `components/value-chart/` that the spans page shares). An existence filter (`exists`, with `exclude` for "does not exist") matches on the key alone: `mapContains` on ClickHouse, `json_extract(...) IS [NOT] NULL` on SQLite and DuckDB. In the `attr=` URL param it is `key?=` / `key!?=`, and `escapeFilterKey` (`utils/attribute-filters.ts`) percent-escapes `%`, `=` and a trailing run of `!`, `~`, `?` in keys so they survive a reload. The expanded log row maps display keys to `{scope, key}` through `logAttributeEntries` (a log attribute named `resource.x` is shown as `log.resource.x`), never by parsing the display key. Attribute charts count a value only when it is a JSON number, the same grammar on every backend (`shared.ChartNumberPattern` on ClickHouse and DuckDB, `json_valid`/`json_type` plus a character check on SQLite; the frontend's `isNumericAttributeValue` decides when to offer Chart values), and only when it fits a float64 (`shared.ChartNumberLimit`): `1e999`, or an id such as `4e98213`, parses as infinity, which no bin, sum or JSON response can hold, and `shared.FiniteChartStats` clamps a sum or percentile that overflows anyway. Percentiles interpolate linearly everywhere: `quantiles` on ClickHouse, `quantile_cont` on DuckDB, and on SQLite a window-function read of the values around each position (`ApplyChartQuantiles`, which takes the positions from the count of that second read, since a record stored between the two reads moves them); `log_attribute_chart_test.go` runs on all three backends. The aggregation SQL is one file per backend (`telemetry/{clickhouse,sqlite,duckdb}/chart.go`: `queryChartStats`, `queryChartBins`) over a values subquery, shared by the log and span charts. Both chart endpoints reuse `searchParams` (same filters and body-search gate as the list) and give up after 30s with a 504. The chart lives in the URL (`chart`, `chartView`, `chartAgg`). Saved filters (`saved_filters`, main DB: `pg/0176`-`0177`, `sqlite/0097`) are per project and page, shared by its members, store the page's own URL params (`savedFilterPageParams`, mirrored by `SAVED_FILTER_PARAMS` in `utils/saved-filters.ts`; pages `logs` and `spans`), names unique ignoring case, at most 100 per page; they are not agent routes.

**Source Maps**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/sourcemaps/upload` | SourceMap | Upload source map file |

### Database Schema

#### Tables (ClickHouse)
| Table | Purpose | Partitioning |
|-------|---------|--------------|
| `spans_v2`, `endpoints_v2`, `tasks_v2`, `ai_traces_v2`, `exceptions_v2` | Traces, see "Trace Storage (V2 tables)" below | Daily (`toYYYYMMDD(recorded_at)`) |
| `spans`, `endpoints`, `tasks`, `ai_traces`, `exception_stack_traces` | Legacy trace tables. History only, read by the move-over | Monthly |
| `archived_exceptions` | Archived/resolved exceptions | None |
| `log_records` | OTel logs with 3 attribute maps (resource/scope/log) | Daily (`toDate(timestamp)`) |

> Data retention/TTLs are documented separately under **Data Retention** below.

#### Tables (PostgreSQL)
| Table | Purpose |
|-------|---------|
| `users` | User accounts with email/password |
| `organizations` | Multi-tenant organizations |
| `organization_users` | Junction table linking users to organizations with roles |
| `projects` | Project config + tokens, linked to organizations |
| `project_user_roles` | Per-project role overrides (`user`/`readonly`) for org members |
| `invitations` | Team invitations with token, role, expiry |
| `source_maps` | Uploaded source map files (project, version, storage key) |
| `metric_registry` | Custom metric definitions (type, unit, description) |
| `dashboards` | Org-owned dashboards (name, JSONB definition with widgets, template provenance) |
| `project_dashboards` | Which projects show a dashboard, and tab order |
| `dashboard_templates` | Marketplace templates (key, category, definition), seeded by migrations |
| `starred_dashboard_widgets` | Homepage layout per project (dashboard id + widget id, position, col_span, size) |
| `synthetic_checks` | Synthetic check config + current state (status, consecutive_failures, next_run_at) |
| `check_runs` | Synthetic run queue (queued/claimed only; terminal rows deleted, telemetry is the record) |
| `check_incidents` | Incident spans: auto (nullable check_id+project_id) and manual (nullable status_page_id), plus public title (feeds status pages) |
| `incident_updates` | Public timeline updates on incidents (status + message, seeded on auto open/resolve) |
| `post_mortems` | Internal markdown post-mortems (project-scoped via `project_id` ON DELETE CASCADE, tags JSON array, optional unique incident link with ON DELETE SET NULL) |
| `post_mortem_events` | Append-only post-mortem edit log (action, JSON changes list, user, ON DELETE CASCADE) |
| `sdk_config_rules` | Remote SDK config rules per project: ordered (`position`), `conditions` JSON (targeting) and `settings` JSON (the overlay), `enabled`; see "Remote SDK Config" |
| `sdk_captures` | On-demand captures per project: UUID id, `conditions` (one attribute eq), `actions` (`{lookbackSeconds, signals, replay}`), `expires_at` (max 24h), `cancelled_at` |
| `saved_filters` | Named filter sets of a dashboard page (`page` = `logs` or `spans`), per project: the page's URL query, unique `(project_id, page, name)` |
| `synthetic_runners` | Liveness registry for self-registered runner fleet (unique name, version, first/last_seen_at; no credentials, no org scoping) |
| `status_pages` | Public uptime pages (org-scoped slug + selected check ids + branding: description, logo_key, unique custom_domain) |
| `widget_groups` / `widget_group_widgets` / `starred_widgets` | Legacy, unused. Do not read or write them |

#### ClickHouse vs PostgreSQL Decision Guide
- **PostgreSQL**: Relational/config data needing ACID, frequent updates, JOINs, low volume (users, organizations, projects, invitations, widgets, source maps, metric registry)
- **ClickHouse**: High-volume append-only telemetry with time-series aggregations, batch inserts only (transactions, exceptions, metric points, spans, tasks, sessions)
- Rule of thumb: "Will this data be updated after creation?" → PostgreSQL. "Is this immutable, time-stamped, high-volume data queried with aggregations?" → ClickHouse.

#### SQLite Dual-Database Architecture (self-hosted mode)

In SQLite mode (`DB_TYPE=sqlite`), the backend uses **two separate SQLite databases** mirroring the PostgreSQL/ClickHouse split:

| Database | Variable | File | Purpose | Transactions |
|----------|----------|------|---------|-------------|
| **Main DB** | `db.DB` | `traceway.db` | PostgreSQL replacement — relational/config data | Yes (`middleware.Transactional`, `db.ExecuteTransaction`) |
| **Telemetry DB** | `db.TelemetryDB` | `traceway_telemetry.db` | ClickHouse replacement — append-only telemetry | No — direct inserts without transactions |

Both embedded builds (SQLite and DuckDB telemetry) are single-instance: the databases are local files, and the in-memory project cache only learns about another process's writes through the Postgres LISTEN/NOTIFY listener, which exists only under `transactional_pg`. Running more than one backend instance is supported only on the `transactional_pg telemetry_ch` build (docs: `/server/minimal#running-more-than-one-instance`).

**Main DB tables** (`db.DB` — transactional, uses lit with `*sql.Tx`):
- `users`, `organizations`, `organization_users`, `projects`, `invitations`
- `source_maps`, `metric_registry`, `dashboards`, `project_dashboards`, `dashboard_templates`, `starred_dashboard_widgets` (plus the legacy `widget_groups`/`widget_group_widgets`/`starred_widgets`)
- `notification_channels`, `notification_rules`
- `synthetic_checks`, `check_runs`, `check_incidents`, `incident_updates`, `post_mortems`, `post_mortem_events`, `synthetic_runners`, `status_pages`
- `sdk_config_rules`, `sdk_captures`
- `saved_filters`

**Telemetry DB tables** (`db.TelemetryDB` — non-transactional, uses lit with `db.TelemetryDB` directly):
- `endpoints_v2`, `tasks_v2`, `ai_traces_v2`, `exceptions_v2`, `spans_v2`, `metric_points`
- `session_recordings`, `archived_exceptions`, `slow_endpoints`, `fired_notifications`, `check_results`
- `endpoints`, `tasks`, `ai_traces`, `exception_stack_traces`, `spans`: legacy trace tables, history for the move-over

**How to access each database in repository code:**

```go
// Main DB (PostgreSQL replacement) — use transactions via middleware or ExecuteTransaction
// Repositories receive *sql.Tx from db.GetTx(ctx) or db.ExecuteTransaction
user, err := lit.SelectSingleNamed[models.User](tx, "SELECT ... FROM users WHERE ...", lit.P{...})

// Telemetry DB (ClickHouse replacement) — use db.TelemetryDB directly, no transactions
results, err := lit.SelectNamed[endpointRow](db.TelemetryDB, "SELECT ... FROM endpoints WHERE ...", lit.P{...})

// Telemetry inserts — no transaction wrapping
for _, item := range items {
    row := modelToRow(item)
    lit.InsertExistingUuid(db.TelemetryDB, &row)
}
```

**Migrations** are split into two directories:
- `backend/app/migrations/sqlite/` — runs on `db.DB` (main)
- `backend/app/migrations/sqlite_telemetry/` — runs on `db.TelemetryDB` (telemetry)

**SQLite-specific type helpers** (`backend/app/repositories/telemetry/sqlitetypes/`):
- `SQLiteTime` — implements `sql.Scanner`/`driver.Valuer` for `time.Time` ↔ SQLite TEXT
- `SQLiteJSONMap` — implements `sql.Scanner`/`driver.Valuer` for `map[string]string` ↔ SQLite JSON TEXT
- Row types (e.g., `endpointRow`, `taskRow`) wrap domain models with these types for lit compatibility

#### DuckDB Telemetry Backend (self-hosted, opt-in)

Built with `-tags telemetry_duckdb` (`CGO_ENABLED=1` required), this is an alternative telemetry store for the same `DB_TYPE=sqlite` deployment: the **main DB stays SQLite** (`db.DB`, relational/config), while the **telemetry DB becomes DuckDB** (`db.TelemetryDB`, columnar). It exists because DuckDB's columnar engine is dramatically faster on the analytics/aggregation reads the dashboard issues — at 10M rows it clears read-probe thresholds that SQLite times out on. Backends are selected on two build-tag axes: `telemetry_ch` / `telemetry_duckdb` / *(none = SQLite telemetry)* for the telemetry store and `transactional_pg` / *(none = SQLite main)* for the relational store. Only three combinations are supported — *(no tags)* dual SQLite, `telemetry_duckdb`, and `transactional_pg telemetry_ch` — enforced by compile-time guard files in `backend/app/db/`. Repositories are organized on the same two axes: telemetry repositories live in per-backend packages `backend/app/repositories/telemetry/{clickhouse,sqlite,duckdb}/` and transactional (relational) repositories in `backend/app/repositories/transactional/{pg,sqlite}/`, each re-exported as singletons through tag-guarded facade files at the axis package root (`telemetry/telemetry_ch.go` etc., `transactional/transactional_pg.go` etc.). Consumers import the facade packages — `telemetry.SpanRepository`, `transactional.UserRepository` — never a backend package directly. Helpers shared by all telemetry backends are in `telemetry/shared/`, the SQLite scan/value types shared by the sqlite+duckdb backends are in `telemetry/sqlitetypes/`, and helpers/types shared by the transactional backends (auth-token hashing/time formats, facade-crossing structs) are in `transactional/shared/`. The `transactional/pg` and `transactional/sqlite` implementations are intentionally kept dialect-neutral (lit `:name` queries rendered per `db.Driver`), enforced byte-for-byte by `transactional/parity_test.go`. Running Postgres requires the `transactional_pg` build: the default build's migration runner applies SQLite-dialect migrations unconditionally, so `DB_TYPE=postgres` without the tag is not a supported combination.

- **Driver:** `github.com/duckdb/duckdb-go/v2` (the official driver). Bundles prebuilt static libs for glibc only — **not musl/Alpine**, so the image uses Debian (`Dockerfile.duckdb`).
- **Opened in** `backend/app/db/db_telemetry_duckdb.go`: telemetry path is the SQLite path with `.db` swapped for `_telemetry.duckdb`. By default DuckDB auto-tunes to the host; `DUCKDB_MEMORY_LIMIT`/`DUCKDB_THREADS`/`DUCKDB_CHECKPOINT_THRESHOLD` (passed through as DSN config options) let operators cap memory/threads so a memory-capped container doesn't read the host's RAM and OOM-kill the backend, and raise the WAL checkpoint threshold (default 16MB) so sustained Appender ingest isn't stalled by frequent checkpoints. `preserve_insertion_order=false` is always set — telemetry reads all have explicit ORDER BY, and dropping the guarantee lets DuckDB parallelize bulk loads and large scans with less memory. The read pool is bounded (`SetMaxOpenConns(duckDBMaxReadConns)`) since each DuckDB connection can use all threads + its own query memory; Appender writes use their own `DuckDBConnector.Connect()` connections and bypass that cap. Exposes `db.DuckDBConnector` (needed for the Appender).
- **Writes use the Appender API**, not `INSERT` (`duckdb.NewAppenderFromConn(conn, "", table)` → `AppendRow(...)` → `Close()` flushes). Upserts still go through `ExecContext` with `ON CONFLICT`. The Appender rejects typed `*string` for nullable VARCHAR — use `nullableString()` in `backend/app/repositories/telemetry/duckdb/helpers.go` (returns untyped `nil` or the dereferenced value).
- **Write-path observability:** a row the Appender rejects is dropped rather than failing the whole frame (the SQLite backend 500s instead), so a poison row cannot wedge the SDK's retry loop. Every drop increments a per-table counter (`db.RecordTelemetryRowDropped`) and fires a rate-limited (1/min per table) `traceway.CaptureException`; Appender flush/connect failures still propagate to the request (500, SDK retries) and increment an insert-failure counter. `GET /api/health/deep` (operator endpoint: requires the HEALTH_DEEP_TOKEN bearer secret, unset disables it; all telemetry backends) exposes `telemetryBackend`, `droppedRows` per table, `droppedRowsTotal`, `insertFailures`, `ingestRejected` (requests turned away by the ingest admission gate), and on DuckDB an `engine` object (db/WAL file bytes, `duckdb_memory()` usage, read-pool in-use/wait stats) alongside its existing ClickHouse fields; it 503s only when the configured telemetry backend is ClickHouse and CH is unreachable (the embedded backends answer 200 with `chReachable:false`). The benchmark loadgen polls it before/after every ramp step and fails any step whose drop delta is nonzero; read-probe fills record cumulative `droppedRows` per fill level. When `MONITORING_TRACEWAY_URL` is set, `monitoring.StartTelemetryDBReporter` also emits `traceway.duckdb.*` metrics every 10s: `rows_dropped.delta`, `insert_failures.delta`, `db_size_mb`, `wal_size_mb`, `memory_used_mb`, `read_pool.in_use`, `read_pool.wait_count.delta`, `read_pool.wait_ms.delta`. The hourly retention worker issues a `CHECKPOINT` after its deletes so retention actually reclaims disk (DuckDB otherwise defers reclamation to the WAL checkpoint threshold).
- **`lit` placeholders:** `db.Driver` stays `lit.SQLite`, which emits `?` — DuckDB accepts these, so no separate driver was needed for reads.
- **Migrations:** `backend/app/migrations/duckdb_telemetry/` (mirrors `sqlite_telemetry/` table-for-table; integer columns are `BIGINT`, JSON is `VARCHAR`, no secondary indexes since it's columnar, with one exception: `0005_create_v2_tables` adds `idx_spans_v2_trace_key` on `spans_v2(trace_key)` for the span graph lookup. While that index exists `ALTER TABLE spans_v2 ADD COLUMN` still works, but `DROP COLUMN` and `ALTER COLUMN ... TYPE` fail with a dependency error, so a migration that needs either must drop the index first).
- **Dialect gotchas vs SQLite** (the read queries differ): native `quantile_cont(col, p)` for P50/P95/P99 instead of fetch-and-sort; `strftime('%s',col)`→`epoch(col)`; time bucketing via `time_bucket(to_seconds(N), col, TIMESTAMP '1970-01-01')` — the explicit epoch origin is required because DuckDB anchors sub-day buckets at 2000-01-03 by default, which would misalign chart buckets against the SQLite backend's epoch-floored buckets for any interval that doesn't evenly divide a day; `json_extract`→`json_extract_string`; `json_each`→`LATERAL unnest(json_keys(x))`; strict GROUP BY needs `ANY_VALUE`/`arg_max`; `SUM` returns HUGEINT (CAST to BIGINT); `CAST(.. AS REAL)`→`CAST(.. AS DOUBLE)`.
- **Tests:** `backend/app/repositories/telemetry/testhelper_duckdb_test.go` (tagged `telemetry_duckdb`) provides `setupTestDB` so the entire existing telemetry test suite runs against an in-memory DuckDB.

#### Trace Storage (V2 tables)

Five tables hold traces on every backend: `spans_v2`, `endpoints_v2`, `tasks_v2`, `ai_traces_v2`, `exceptions_v2`. Read `dev-docs/telemetry-internals.md` before changing trace ingest, span reads, the distributed trace card, the span explorer or the move-over. What every change must respect:

- Every row carries the ids its span arrived with as lowercase hex: `trace_id` (32 characters), `span_id` (16 for OTel, 32 for a native run or span), `parent_span_id` (`''` for a root). Entities and exceptions keep a UUID `id` for URLs and notifications.
- The legacy tables (`spans`, `endpoints`, `tasks`, `ai_traces`, `exception_stack_traces`) hold history only. Nothing reads or writes them except the move-over and retention.
- Spans are inserted before the rows promoted from them, and a transient storage error answers 503 + `Retry-After`. Retries are append-only; the span reader deduplicates per span.
- Every span read is bound in time: `recorded_at` within 24 hours either side of the entity it starts from (`shared.OtelLookupWindow`). Tests that store spans under a fixed timestamp must pass that timestamp as the lookup time, never the wall clock.
- Frontend pages hold span payloads in `$state.raw`, never a deep `$state` proxy.

#### Data Retention

Retention is handled in several different ways depending on the deployment.

**0. Main-DB auth prune — `retention.Start` worker** (`backend/app/retention/auth_tokens.go` + `oauth_sessions.go`, sharing `startDBPruneWorker` in `prune_worker.go`). Runs in **all modes** (Postgres and SQLite) against `db.DB`: once at startup, then every 24h, deleting expired/consumed auth rows. `auth_tokens` prunes expired `device_authorizations` (also pruned opportunistically on every `/api/auth/device/authorize` call, so the daily worker is a backstop there), `refresh_tokens` that are expired, revoked, or used more than 30 days ago (used rows are kept a month for replay detection, then dropped to bound per-user growth), plus revoked-or-expired `personal_access_tokens` (PAT expiry was otherwise only enforced lazily at read time). `oauth_sessions` prunes expired SSO login sessions. No env var — always on. (Refresh-token families and PATs also have explicit revoke paths: `POST /api/auth/logout` and the account PAT UI.)

**1. ClickHouse — `TTL` clauses on the table itself.** Only a few tables have a TTL; everything else is kept indefinitely (operators can drop monthly partitions manually if needed).

| Table | Retention | Source migration |
|-------|-----------|------------------|
| `metric_points` (raw) | **7 days** | `0034_add_ttl_metric_points.up.sql` |
| `metric_points_1m` (1-min rollup) | **30 days** | `0035_add_ttl_metric_points_1m.up.sql` |
| `metric_points_1h` (1-hour rollup) | **1 year** | `0036_add_ttl_metric_points_1h.up.sql` |
| `log_records` | **90 days** | `0075_increase_ttl_log_records.up.sql` |
| `profiling_samples` (the bulk) | **30 days** | `0068_add_ttl_profiling_samples.up.sql` |
| `profiles` (slim metadata) | **30 days** | `0069_add_ttl_profiles.up.sql` |
| `profiling_stacks` (dedup table) | **30 days** | `0070_add_ttl_profiling_stacks.up.sql` |
| `check_results` (synthetic check probes) | **90 days** | `0083_add_ttl_check_results.up.sql` |
| All other CH tables (the five V2 trace tables and the five legacy ones, `transactions`, `sessions`, `session_recordings`, `fired_notifications`, `archived_exceptions`, `slow_endpoints`, etc.) | **No TTL, retained indefinitely** | none |

The three profiling tables share a 30-day TTL keyed on each table's time column (`start_time` / `recorded_at` / `last_seen`). `profiling_stacks` is a `ReplacingMergeTree(last_seen)` dedup table, so `last_seen` is bumped on every re-ingest that references a stack — a stack only ages out once it has gone unreferenced for the full window, which is exactly when its samples have also expired, so no sample is ever left pointing at a dropped stack.

**2. SQLite — `retention.Start` worker** (`backend/app/retention/sqlite.go`). In SQLite mode, neither `db.DB` nor `db.TelemetryDB` has any built-in expiry, so a background worker fires once at startup and then every hour and runs a `DELETE FROM <table> WHERE <time_column> < cutoff` against each telemetry table.

| Variable | Default | Notes |
|----------|---------|-------|
| `SQLITE_RETENTION_DAYS` | `30` | TTL in days. Set to `0` to disable the worker entirely. Has no effect outside SQLite mode. |
| `DUCKDB_RETENTION_DAYS` | `30` | Same worker on the DuckDB telemetry backend. When set it takes precedence there; when unset the worker falls back to `SQLITE_RETENTION_DAYS` (selection in `retention.telemetryRetentionConfig`). |

Tables it prunes (and the column used):

| Database | Table | Time column |
|----------|-------|-------------|
| Telemetry (`db.TelemetryDB`) | `endpoints_v2`, `tasks_v2`, `exceptions_v2`, `spans_v2`, `ai_traces_v2`, the five legacy trace tables (`endpoints`, `tasks`, `exception_stack_traces`, `spans`, `ai_traces`), `metric_points`, `session_recordings` | `recorded_at` |
| Telemetry | `log_records` | `timestamp` |
| Telemetry | `sessions` | `started_at` |
| Telemetry | `fired_notifications` | `fired_at` |
| Telemetry | `profiling_samples` | `start_time` |
| Telemetry | `profiles` | `recorded_at` |
| Telemetry | `profiling_stacks` | `last_seen` |
| Telemetry | `check_results` | `recorded_at` |

`archived_exceptions` (per-hash flags) and `slow_endpoints` (per-endpoint config) are intentionally skipped — they are not time-series data. `profiling_stacks` *is* pruned (unlike those two) because it holds no user intent — it is a regenerable dedup table whose `last_seen` tracks the most recent referencing sample, so deleting expired stacks is safe.

**2c. Synthetics failure screenshots — `retention.Start` worker** (`backend/app/retention/synthetics.go`). Browser-check failure screenshots written to local disk under `<STORAGE_PATH>/synthetics/` are aged out hourly by mtime, mirroring the session-recording split: the `check_results` rows referencing them are pruned separately (item 2 / CH TTL) and deliberately not coupled. `SYNTHETICS_SCREENSHOT_RETENTION_DAYS` (default 30, `0` disables); no-op unless `STORAGE_TYPE=local`.

**2b. Log row cap — `retention.Start` worker** (`backend/app/retention/log_cap.go`). SQLite mode only (SQLite or DuckDB telemetry), off by default. When `LOG_RECORDS_MAX_ROWS` is set to a positive N, a worker runs once at startup and then every minute and deletes `log_records` rows strictly older than the Nth-newest row's `timestamp` (single portable DELETE with an `ORDER BY timestamp DESC LIMIT 1 OFFSET N-1` subquery; NULL boundary = no-op under the cap, boundary ties are kept). **The cap is best-effort, not a hard limit**: nothing throttles ingest, so between passes the table can exceed N, and sustained ingest above N rows/minute keeps it above the cap permanently — document it to users as a cleanup task with a 1-minute window, sized with headroom for peak log volume. Bounds log disk usage independently of `SQLITE_RETENTION_DAYS`; disk reclamation still happens via the hourly retention pass / DuckDB WAL checkpointing.

**3. On-disk session recordings — `retention.Start` worker** (`backend/app/retention/recordings.go`). Session recordings written to local disk (`STORAGE_TYPE=local`) accumulate under `<STORAGE_PATH>/recordings/`. A second worker walks that directory once at startup and then every hour and removes files whose `mtime` is older than the TTL, then prunes any directories left empty. The worker is a no-op when `STORAGE_TYPE=s3`.

| Variable | Default | Notes |
|----------|---------|-------|
| `SESSION_RECORDING_RETENTION_DAYS` | `30` | TTL in days. Set to `0` to disable the worker. Only runs when `STORAGE_TYPE=local` (default). |

The DB rows in `session_recordings` are pruned by the SQLite retention worker (above) or by ClickHouse TTL — they are intentionally not coupled to the disk cleanup. Controllers that read recordings already log a non-fatal `traceway.CaptureException` when a referenced file is missing.

**4. On-disk raw profile archives — `retention.Start` worker** (`backend/app/retention/profiles.go`). When `PROFILE_ARCHIVE_RAW` is enabled, the **native pprof ingest path** (`/profiles/ingest`) writes each upload's original pprof bytes to `<STORAGE_PATH>/profiles/<projectId>/<yyyymmdd>/<id>.pprof` (recorded on `Profile.StorageKey`) as a lossless archive for download / re-ingest / PGO — off the read path. The OTLP endpoint does not archive (its rows carry an empty `StorageKey`). A worker walks that directory once at startup and then every hour and removes files whose `mtime` is older than the TTL, then prunes empty directories. It reuses the same generic age-based cleanup as the recordings worker (`runDirAgeCleanup` / `isSafeStorageSubdir`) and is a no-op unless `PROFILE_ARCHIVE_RAW` is on **and** `STORAGE_TYPE=local`.

| Variable | Default | Notes |
|----------|---------|-------|
| `PROFILE_ARCHIVE_RAW` | `false` | Master switch for the raw archive. When off, no blob is written and the disk worker does nothing. |
| `PROFILE_RETENTION_DAYS` | `30` | TTL in days for the on-disk archive. Set to `0` to disable the disk worker. Only runs when `PROFILE_ARCHIVE_RAW` is on and `STORAGE_TYPE=local`. |

The `profiles` DB rows (and their `storage_key`) are pruned by the SQLite retention worker / ClickHouse TTL above — not coupled to this disk cleanup, mirroring the session-recording split.

**5. Main-DB outbox prune — `retention.Start` worker** (`backend/app/retention/outbox.go`, same `startDBPruneWorker` scaffolding as item 0). Runs in **all modes** against `db.DB`: once at startup, then every 24h, deleting terminal `notification_outbox` rows — `sent`/`cancelled` older than 7 days, `failed` older than 30 days. `pending` and `sending` rows are never pruned, so nothing undelivered is dropped. No env var — always on. The durable record of what was notified lives in `fired_notifications` and `page_notifications`, which this does not touch. Note that `pages` and `page_notifications` themselves are currently retained indefinitely.

#### Session Recording Uploader

Session recording segments arriving on `/api/report` are not uploaded inline. The handler enqueues each segment onto a bounded worker pool (`backend/app/recordings/uploader.go`, started from `cmd/run.go` next to `retention.Start`). Workers drain the queue and write the body via `storage.Store.Write` (S3 or local disk); successful writes are handed to a single batcher goroutine that calls `SessionRecordingRepository.InsertAsync` once per ~1000 rows or every 2 s, whichever comes first — single-row inserts are an anti-pattern for ClickHouse. Enqueue is non-blocking: when the queue is full the segment is dropped (newest-first) so a burst of `/api/report` traffic cannot spawn unbounded goroutines or saturate S3.

| Variable | Default | Notes |
|----------|---------|-------|
| `SESSION_RECORDING_UPLOAD_WORKERS` | `32` | Concurrent uploaders. Set to `0` to drop every segment (uploads disabled). |
| `SESSION_RECORDING_UPLOAD_QUEUE_SIZE` | `2048` | Max queued segments. Overflow is dropped. |

Observability (emitted every 10s via `traceway.CaptureMetric`):

| Metric | Type | Meaning |
|--------|------|---------|
| `traceway.recordings.queue_depth` | gauge | Segments currently waiting in the channel. |
| `traceway.recordings.in_flight` | gauge | Workers mid-upload. |
| `traceway.recordings.uploaded.delta` | gauge | Successful S3/local + DB writes since the previous report (10s). |
| `traceway.recordings.dropped.delta` | gauge | Segments dropped on overflow since the previous report. |
| `traceway.recordings.failed.delta` | gauge | Upload or DB-insert errors since the previous report. |

Sustained drops also fire a rate-limited (1/min) `traceway.CaptureException` so overload is visible in the issues feed without flooding it.

**Session end is derived on read.** The browser SDK's closing payload is unreliable (a keepalive body over 64 KB is rejected on unload, and frozen mobile tabs never fire `pagehide`), so the stored `sessions.ended_at` is only a hint. Each segment row carries its client-side `ended_at` (`ch/0100`, `sqlite_telemetry/0027`, `duckdb_telemetry/0008`), and every session read joins the latest segment activity and applies `shared.ResolveSessionEnd`: an explicit end is kept only when it lands within `SessionIdleTimeout` (15 min, the SDK's inactivity timeout) after the last activity, otherwise the session ends at its last activity; it is still live while a segment arrived (server time) within the timeout. Every session is capped at `shared.SessionMaxSpan` (65 min: the SDK's 60 min cap plus slack) because `@tracewayapp/frontend` <= 1.2.0 kept uploading segments under an ended session id for the tab's whole lifetime (18h "sessions"); the end and duration are clamped to it (sessions without segments too), and when segments run past it the session ends at the last segment inside it (`span_activity`, computed per backend against the session start; the usual 15 min rule then honours the SDK's own close), not at the cap, because an idle tab uploads nothing until it wakes hours later (sleep/wake, or 1.2.0's zombie uploads) and resumes under the same id; `Session.CutoffAt` carries the resulting cut-off, a session whose segments run past it is never live (compared on the client clock, so skew between browser and server cannot end a live session), and the session detail and `GET /api/sessions/:id/recording` drop exceptions and segments past `CutoffAt`, anchored on the stored session start rather than the `startedAt` query param (`shared.SegmentsWithinSession`). `HasRecording` is set when any segment exists, and the frontend labels an old session with neither as "No recording" (`utils/session-duration.ts`). Sorting the list by duration uses a SQL copy of the same rule in each backend (`sessionDurationSortKey`); `TestSessionRepository_EndFromRecordingActivity` holds the SQL order to the Go rule. `GET /api/sessions/:id/recording?startedAt=` bounds the segment lookup to the session's window, and segments are read from storage 16 at a time.

#### Users, Organizations & Projects

- **Users to organizations is many-to-many** via `organization_users`, one role per membership. A user joins additional organizations through invitations (`POST /api/invitations/:token/accept-existing` for existing accounts) and, in cloud mode, registration. Login/register/`LoginBundle` return every membership as `Organizations[]` with roles; the frontend keeps them in `authState.organizations` and groups the navbar project selector by organization when there is more than one.
- **Projects belong to exactly one organization** (`projects.organization_id`). Org membership grants read access to all of the org's projects.
- **Per-project role overrides** live in `project_user_roles(project_id, user_id, role)` with role `user` or `readonly`. Overrides only apply to members whose org role is `user` or `readonly`; owners and admins always have full access to every org project. Override rows are kept when a member's org role changes (they are inert for owner/admin), and are deleted when the member is removed from the org or the project is deleted. Managed from Settings > Team Members (expand a member row) via `GET/PUT /api/organizations/:orgId/members/:userId/project-roles(/:projectId)`; `PUT` with `role: "default"` clears the override and is accepted for any member regardless of org role (so stale rows on members promoted to admin can still be cleaned up); setting `user`/`readonly` is rejected with 422 for owners and admins.
- **Effective project role** (`ProjectRepository.GetEffectiveRole`): the org role if `owner`/`admin`, otherwise the override if present, otherwise the org role. `/api/projects` returns it as `role` on each project and masks `token`/`sourceMapToken` when it resolves to `readonly`; the frontend derives write gating from it (`isProjectReadonly` in `projects.svelte.ts`).

#### Organization Roles
| Role | Description |
|------|-------------|
| `owner` | Full access, can manage organization |
| `admin` | Full access to projects |
| `user` | Standard access to projects (can be overridden per project to `readonly`) |
| `readonly` | Read-only access, cannot create projects or archive exceptions (can be overridden per project to `user`) |

Middleware enforcement: `RequireProjectAccess` checks org membership (read access, unaffected by overrides); `RequireWriteAccess` blocks writes when the **effective project role** is `readonly`; `RequireAdminAccess` requires org role `owner`/`admin` for the `:organizationId` route param.

### Database Migrations

**CRITICAL RULES:**
1. `migrations/ch/` and `migrations/pg/` files must contain **exactly ONE SQL statement**. Both run through `golang-migrate`, and the ClickHouse driver is constructed with `MultiStatementEnabled: false` (`migrations_telemetry_ch.go`), so a second statement fails. `pg/` follows the same rule for symmetry.
2. `migrations/sqlite/`, `migrations/sqlite_telemetry/` and `migrations/duckdb_telemetry/` run through `runMigrationsOn` in `migrations.go`, which splits on semicolons outside string literals (`splitStatements`, covered by `split_test.go`). A file there may hold a `CREATE TABLE` plus its indexes — that is the existing convention, e.g. `sqlite/0043_create_pages.up.sql`.
3. Only create `.up.sql` files (no down migrations)
4. Use sequential numbering: `NNNN_description.up.sql`

**Example - Adding two columns requires TWO files:**
```
backend/app/migrations/ch/
├── 0013_add_app_version_to_transactions.up.sql
│   └── ALTER TABLE transactions ADD COLUMN app_version String DEFAULT ''
├── 0014_add_server_name_to_transactions.up.sql
│   └── ALTER TABLE transactions ADD COLUMN server_name String DEFAULT ''
```

### Exception Hash Normalization

The backend normalizes stack traces before hashing to group identical errors despite different runtime values. This happens in `backend/app/controllers/clientcontrollers/client.controller.go`.

**Normalization Steps** (`ComputeExceptionHash`, applied in this order, and skipped entirely when `isMessage` is true):
1. Strip the message from `Caused by:` lines, keeping the class name (`causedByRe`)
2. Strip the message from the error line, keeping the error type (`errorMessageRe`)
3. Collapse JS SDK function-name lines (ending in `()`, directly above a 4-space-indented `file:line:col` location line) to `<fn>` so resolved function names never affect grouping; anchoring on the location line keeps Go traces (tab-indented file lines) untouched (`jsFuncLineRe`)
4. Remove URL origins such as `https://cdn.example.com` (`urlOriginRe`)
5. Remove absolute file paths, keeping `filename:line` (`absolutePathRe`)
6. Drop the column from any frame whose line number is 2 or higher (`laterLineColRe`)
7. Remove `@v1.2.3` module version suffixes (`versionRe`)
8. Replace hexadecimal addresses with `<hex>` (`hexRe`)
9. Replace UUIDs with `<uuid>` (`uuidRe`)
10. Replace runs of 5 or more digits with `<id>`, unless preceded by a colon or another digit (`largeNumberRe`)
11. Replace email addresses with `<email>` (`emailRe`)
12. Replace IP addresses, with an optional port, with `<ip>` (`ipRe`)
13. Replace Go goroutine ids with `goroutine <n>` (`goroutineRe`)
14. Drop line numbers from Java, Kotlin and Scala frames (`javaLineNumRe`)
15. Collapse `... 12 more` to `... more` (`javaEllipsisRe`)
16. Collapse runs of spaces and tabs, then runs of newlines (`spacesRe`, `newlinesRe`)
17. Trim, hash with SHA-256, truncate to 16 hex chars

**Not normalized:** timestamps and ANSI color codes. Two otherwise identical traces that differ only in an embedded timestamp, or only in `\x1b[31m` escapes, produce two separate Issues. Add a regex to the block in `client.controller.go` if that ever matters.

**Note:** the column is kept only on line 1 (step 6). Minified bundles put everything on line 1, so there the column is the only frame disambiguator when no source map matched, while on real source lines the column is noise. Function names are excluded (step 3) because they are derived from the location and change as symbolication improves.

**Result:** Same logical error gets same hash, even if:
- Error message contains different user IDs
- Stack trace has different memory addresses
- File paths differ between environments

### Repository Patterns

#### Singleton Pattern
Repositories are exported as package-level singletons, re-exported per storage axis through the facade packages `app/repositories/transactional` and `app/repositories/telemetry`:
```go
// backend/app/repositories/transactional/sqlite/user.repository.go (and the pg/ twin)
var UserRepository = userRepository{}

// backend/app/repositories/transactional/transactional_sqlite.go (tag-guarded facade)
var UserRepository = sqliterepo.UserRepository

// Usage in controllers
user, err := transactional.UserRepository.FindByEmail(tx, email)
spans, err := telemetry.SpanRepository.FindByTraceId(projectId, traceId)
```

#### Map columns that identify a row (ClickHouse)
ClickHouse compares a `Map` in the order its entries were written, and clickhouse-go writes a Go map in iteration order, which Go randomizes. Where a map is part of a sorting key or a `GROUP BY` (`tags` of `metric_points_1m/_1h/_1d` and of the three `metric_histograms` tables), the same tags in another order are another row that never merges. Three things keep a series one row, and a new table or query of this kind needs all of them:
- **Writes** append the map through `chdb.SortedMap` (`var tags chdb.SortedMap` per batch, `tags.Of(m)` per row), which gives the order of `mapSort`. `metric_points` and `metric_histograms` do; maps that identify nothing (log, span and exemplar attributes) are written as they are.
- **The rollup views** select and group by `mapSort(source.tags)` (`ch/0123`-`0127`), so a writer that does not sort, such as an older instance during a rolling deploy, cannot split a rollup. `metric_histograms` itself has no view in front of it and depends on the writer.
- **Reads** that take the whole map as a series identity use `mapSort(tags)` (`rateDeltasSelect`, `HotSeries`), because rows written unsorted stay until their TTL (`metric_points_1h` a year, `metric_points_1d` forever) unless the repair is run. Reads that only pick tags out (`tags[?]`) and merge states were never affected.

`metric_tag_order_clickhouse_test.go` holds each of the three on its own.

Rollup rows that were written unsorted can be merged by the opt-in repair worker (`METRIC_ROLLUP_REPAIR=true`, `clickhouse/metric_rollup_repair.go`). Its design and invariants are in `dev-docs/telemetry-internals.md`.

### Error Handling Pattern

**IMPORTANT:** When handling errors in controllers, always use `c.AbortWithError` with `traceway.NewStackTraceErrorf` instead of `c.JSON` with a generic error message. This ensures proper error tracking with stack traces.

```go
// CORRECT - Use AbortWithError with descriptive reason
projectId, err := middleware.GetProjectId(c)
if err != nil {
    c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
    return
}

// WRONG - Do not use c.JSON for internal server errors
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
    return
}
```

**Key points:**
- The reason should describe the actual cause (e.g., "RequireProjectAccess middleware must be applied")
- Use `%w` to wrap the original error for proper error chaining
- This pattern applies to all 500 Internal Server Error responses
- For client errors (400, 404), `c.JSON` with an error message is acceptable

**Non-stopping errors:** For errors that should not abort the request (e.g., optional feature failed to load), use `traceway.CaptureException` to report them instead of `log.Printf`:

```go
// CORRECT - Report non-stopping errors via traceway
if err != nil {
    traceway.CaptureException(fmt.Errorf("failed to read session recording (key=%s): %w", key, err))
}

// ALSO CORRECT - inside a handler or middleware, before the response is sent
if err != nil {
    _ = c.Error(fmt.Errorf("failed to warm the project cache: %w", err))
}

// WRONG - Do not use log.Printf for errors
if err != nil {
    log.Printf("Failed to read session recording (key=%s): %v", key, err)
}
```

Inside a handler or middleware, `c.Error(err)` records the error on the request. When self-monitoring is recording the request, `recordRequestOutcome` attaches each entry to the request span. `runCommitHook` uses this path for a panicking commit hook.

**Validation error conventions:**
- `400 Bad Request`: Malformed requests, missing required params, type errors
- `422 Unprocessable Entity`: Business validation in form dialogs (name too long, duplicate name, required field empty). Return `c.JSON(422, gin.H{"error": "descriptive message"})`. The frontend `api.ts` extracts 422 error messages — dialogs catch and display them inline.

**Summary:**
- **Stopping errors** (abort the request): `c.AbortWithError(status, traceway.NewStackTraceErrorf("reason: %w", err))`
- **Non-stopping errors** (continue serving): `traceway.CaptureException(fmt.Errorf("reason: %w", err))`, or `c.Error(err)` when a `*gin.Context` is in hand and the response has not been sent
- **Validation errors** (user-facing): `c.JSON(422, gin.H{"error": "message"})` for form validation
- **Always** wrap errors with `traceway.NewStackTraceErrorf` or `fmt.Errorf` using `%w` — never discard the original error

### Self-monitoring

`MONITORING_TRACEWAY_URL` (`<project_token>@<instance URL>`; a legacy `/api/report` suffix is accepted, embedded: `WithMonitoringURL`) makes the backend report itself over OpenTelemetry (OTLP/HTTP to `<instance>/api/otel/v1/*`). `backend/app/traceway` is the call-site API (`traceway.NewStackTraceErrorf`, `CaptureException`, `Recover`, `StartSpan`, `CaptureMetric*`) that the error-handling rules above use. Everything is a no-op until `traceway.Setup` succeeds (`startSelfMonitoring` in `cmd/run.go`, called right after `config.Init` so the database wrappers see it).

- **Stable issue grouping.** `StackTraceError` keeps its type name and frame format on purpose: ingest prepends `type: message` to `exception.stacktrace` and hashes the result. Stacks do not depend on the instrumentation: captured stacks leave out the frames registered with `traceway.HideFrames`/`HidePackageFrames` (otelgin, `middleware.recordRequestOutcome`, `traceway.RunTask`) together with the `gin.(*Context).Next` each of them called, so a traced request reports the stack an untraced one would, and an otelgin upgrade or a wrapper refactor does not split issues (`otelcontrollers/self_monitoring_hash_test.go`). A panic's stack starts below the innermost `runtime.gopanic`, so a re-panic (`Transactional`) starts at the re-panic. `CaptureMessage` sets the event attribute `traceway.is_message`, which OTLP ingest reads as the native `isMessage` (stored as a message, hashed without normalization).
- **Providers are not global.** The tracer/meter/logger providers are passed explicitly (`traceway.TracerProvider()`), so libraries tracing through `otel.Tracer` (the GCS client) emit nothing. Outbound HTTP is not traced: nothing wraps `http.DefaultTransport`. Wrapping it would not reach the exporters either, since each OTLP HTTP exporter builds its own `http.Transport` (`ourTransport` in its `client.go`) and `app/traceway` passes no `WithHTTPClient`; the export loop is broken on the receiving side, where `middleware.SelfMonitoring` skips requests carrying the monitoring token.
- **Request spans** come from `middleware.SelfMonitoring` (otelgin plus `recordRequestOutcome`, which records `c.Errors` with their embedded frames and turns a panic into an exception and a 500). It never traces a request carrying the monitoring token: those are its own exports, and tracing them loops. It also skips NoRoute/SPA paths, static assets, `/api/runners/poll`, `/api/otel/v1/opamp` and `GET /mcp*`, and rewrites `url.path` to the route for `:token` routes. Customer ingest (including `/api/recordings`) stays traced as endpoints, but `ingestRoutes` marks the request with `traceway.WithoutStatementSpans`, so its statements are not traced (they scale with customer traffic; `traceway.ingest.batch.*` times them). The sampler is `AlwaysSample`: an inbound `traceparent` can name the trace, but its sampled flag must never switch the backend's own error recording off. A response whose Content-Type is `text/event-stream` (the MCP `POST /mcp` and `/mcp/write` answers) is tagged `traceway.is_stream`, so ingest stores it as a stream and keeps it out of percentiles, Apdex and latency rules.
- **`traceway.ParentContext(ctx)`** finds the span for a `*gin.Context` through `gin.ContextRequestKey`. `ContextWithFallback` stays off on purpose, since it would let client disconnects cancel queries. Pass `c` (or any ctx) down; do not enable the fallback. Never hand `c` to a goroutine that outlives the handler (gin pools the context and otelgin rewrites `c.Request`): pass `c.Request.Context()`. `traceway.StartSpan(ctx, name)` returns `(ctx, *Span)`; run the work with the returned context so its statements nest under the span.
- **Database spans** (`app/db/dbtrace`, wrapping SQLite, Postgres and the DuckDB read pool; `chdb.tracedConn` for ClickHouse; a manual span around DuckDB appenders) are only created under a request or task span. `lit` has no context, so a statement inside a transaction takes the span the transaction was begun with: `Transactional` and `db.ExecuteTransactionContext(ctx, …)` begin with `context.WithoutCancel(ctx)`, and lit reads outside a transaction pass `db.BindContext(ctx, conn)` (telemetry repositories through `shared.Traced`). The wrappers find their parent with `traceway.StatementParent`, which honours `WithoutStatementSpans` on the context or on the gin request. Spans are named `{operation} {table}` (`dbtrace.Summarize`). A read's span ends when its rows are closed or exhausted, not when the query returns, because SQLite, DuckDB and ClickHouse return after the first row or block. Consecutive identical statements in one transaction (prepared ones included) fold into one span with `db.operation.batch.size`, whose duration is the time the statements spent in the driver, not the wall time between them. Reads fold too (on Postgres `lit.Insert` is `INSERT … RETURNING` through `QueryRow`): a read joins a batch only after the previous read's rows were closed, and its time counts until its rows are closed; inside a transaction a read span is exported when its batch ends (the next different statement, commit or rollback), with its end time set to when its rows were closed. Failed statements set the span status only (no exception, so no issue; `sql.ErrNoRows` is not a failure). The wrappers delegate every optional `database/sql/driver` interface with database/sql's own defaults; a statement's `CheckNamedValue` falls back to the connection's, because DuckDB's type checks live there. `chdb.BatchCtx(ctx)`/`SendBatch(ctx, …)` carry the caller's trace without its cancellation. Health snapshots, the OAuth grant functions, notification dispatch, page open/resolve hooks and metric auto-registration all take a ctx for this; only startup work (backfills, seed) and the async PAT touch run without one. Pool stats are reported as `traceway.db.pool.*` (tag `pool`).
- **Background work** runs as Tasks via `traceway.RunTask(ctx, name, fn)` (CONSUMER root spans): `outbox.drain`, `oncall.escalate`, `notifications.evaluate`, `synthetics.schedule`, `slo.compute`, `retention.*`. Use `CaptureExceptionContext(ctx, err)` inside them so the failure marks the task. A tick that found nothing to do calls `traceway.SkipTask(ctx)`: `taskFilter` (`traceway/task_filter.go`) holds a task's spans until it ends and drops the whole trace when it was skipped and did not fail; a task past 64 spans releases them as they end. A panic inside `RunTask` is recorded on the task and re-panicked wrapped, so the goroutine's `defer traceway.Recover()` does not report it twice.
- **Metrics** export every 5s with delta temporality: `CaptureMetric*` record sync gauges (one point per series per interval, so the export interval must stay below the fastest reporter tick, 10s), `CaptureMetricMax` reports the largest value per tag set in each interval (the per-batch ingest metrics, so a spike between exports is kept), `CaptureCount` sums events. Go runtime metrics (`go.*`) come from contrib/runtime; host metrics are the OTel agent's job. `traceway.proc.cpu_pcnt` divides by `GOMAXPROCS`, which follows a container's CPU limit (`NumCPU` does not).
- **Logs:** `config.Logf`/`Logln` also emit OTLP log records (severity from a `WARNING`/`ERROR`/`FAILED` prefix); `config.LogfContext(ctx, …)` links a line to its request or task. `LoggingEnabled` (embedded `DisableLogging()`) only quiets stdout; the records are still exported. `config.LogfLocal` writes to stdout only and is never exported: every member of the monitoring project's organization can read its logs, so any line carrying a secret uses it (the SMTP-off email log with its reset and invitation links, seeded project tokens).
- **Setup and shutdown.** A malformed `OTEL_RESOURCE_ATTRIBUTES` entry is skipped (`resource.ErrPartialResource`), never a reason to turn monitoring off. `traceway.Shutdown(ctx)` flushes; `exitOnMisconfiguration` (`cmd/run.go`) calls it before the startup `os.Exit(1)` paths, and `cmd.ShutdownOnSignal` (exported as `tracewaybackend.ShutdownOnSignal`, with `Shutdown` for an app that has its own signal handling) runs `cmd.Shutdown` on SIGTERM/SIGINT, which releases the SLO shard leases, writes the client metric cells still in memory and flushes self-monitoring last, then re-raises the signal so the exit status is unchanged. `Run` never installs it, since an embedding app owns its signals: the standalone binary (`cmd/traceway/main.go`) starts it, and any other binary built on `Run` must start it (or call `Shutdown`) itself, or every restart loses those cells and leaves the SLO leases blocking the next instance.
- A self-monitoring project must be a backend framework project (OpenTelemetry): OTLP sent with a browser project's token keeps only issues.

### Emails

Every email Traceway sends is a hardcoded template in `backend/app/services/emailtemplates/` (embedded with `go:embed`, parsed once at init). The copy of an email lives in its own `.gohtml` file; the payload is the only thing it interpolates. There is no shared layout document, so changing one email cannot reshape the others, and every file is a complete HTML document styled like the rest.

Templates: `new_error`, `error_regression`, `check_down`, `check_recovered`, `alert` (the threshold email every rate/latency/apdex/throughput/task/impact/cost rule sends), `ai_flagged`, `page` (on-call escalation), `test` (both test buttons), `invitation`, `password_reset`.

- **One send path** — `services.SendEmail(ctx, services.Email{...})` in `backend/app/services/email.service.go` is the only way mail leaves the process: it renders the template, wraps it as `multipart/alternative` (plaintext part first, quoted-printable), and talks SMTP. With `SMTP_ENABLED` off it logs the plaintext instead. `services.RenderEmail` is the render-only half, used by the preview endpoint.
- **`services.Email`** carries the chrome every template shares (`Title`, `Badge`/`BadgeColor` via `EmailColor*`, `URL`, `Footer`, `LogoURL` defaulting to `{APP_BASE_URL}/traceway-mark.png`) plus `Template` and `Data`, the typed payload reached as `{{.Data.X}}`. `Text` is the plaintext alternative and must stand on its own.
- **Notification payloads** live on `models.NotificationMessage.Email` (`models.NotificationEmail`: a template name plus one typed struct — `EmailException`, `EmailCheck`, `EmailAlert`, `EmailFlagged`, `EmailPage`, `EmailTest`). The builders in `notifications/messages.go` attach it; `services.NotificationEmail` turns a message into the `Email` at send time (severity chip, absolute link, "why you got this" footer). It rides through the notification outbox as JSON, so the shape is a wire format: add fields, never repurpose them. Only the email adapter reads it — Slack/Telegram/SMS/webhook and `fired_notifications` still see `Body` alone, so `Body` must always stay complete.
- **Adding an email**: add `emailtemplates/<name>.gohtml`, add its payload struct, and call `SendEmail` with `Template: "<name>"`. For a new rule type that only needs a sentence and a link, reuse `alert` by filling `models.EmailAlert`.
- **Extension templates**: `services.RegisterEmailTemplates(fsys, patterns...)` adds templates from another module, named by file name like the core ones; call it from an `init` function, since `html/template` refuses to parse after its first execution. `Email.ReplyTo` sets a `Reply-To` header (sanitized and encoded like `To`).
- **Preview**: `EMAIL_PREVIEW_ENABLED=true` registers `GET /api/email-preview` (index) and `GET /api/email-preview/:template`, which renders any template with sample data (`?format=text` shows the plaintext alternative). Off by default, so it is never a route in production. Sample data lives in `controllers/email_preview.controller.go`.

---

## Client Metrics

Performance metrics of browsers and mobile apps are distributions over a population, never a time series per client. The stored unit is a mergeable histogram per `(project, metric, dimensions, minute)`, so storage grows with dimension combinations and not with users. Read `dev-docs/telemetry-internals.md` before changing them (histogram math, storage, the `app/clientmetrics` aggregator and its guards, ingest, alerts, SLOs, dashboards). User-facing notes: `docs/pages/learn/client-metrics.mdx`.

- Percentiles are always computed in Go from merged buckets (`repositories/telemetry/shared/histogram.go`), so the three backends give the same answer; never compute a percentile from percentiles.
- Metric names come from the closed catalog in `clientmetrics/catalog.go`, and dimensions pass the guard in `dims.go`.
- `route.go` (`NormalizePath`) must stay in step with js-client `core/src/metrics/route.ts`; both test tables are the same.

---

## Native `/api/report` Protocol

There is **no Traceway Go SDK**: every backend, Go included, instruments with OpenTelemetry and exports over OTLP/HTTP. The docs carry no Go SDK pages, and `docs/public/_redirects` 301s `/client/sdk` and `/client/*-middleware` to `/client/otel`. Do not reintroduce them.

The native protocol is **deprecated**. The public docs do not describe `/api/report` as a way to send data: there is no protocol page (`/protocol` 301s to `/client/otel`) and `docs/pages/learn/data-flow.mdx` is written around OTLP. Do not document it. The URL shows up in the docs in one role only, as part of the connection string the browser and mobile SDKs are configured with.

The endpoint is still served, because those SDKs (`@tracewayapp/*`, the iOS and Android libraries, Flutter) speak it. This section is the engineering reference for keeping it working. Its rows land in the same V2 tables as OTLP: a run is stored as the root span of a trace with its spans under it (see "Trace Storage (V2 tables)").

### Data Format (Frame)

Clients send data as gzipped JSON. The wire shape is `ReportRequest` in `backend/app/controllers/clientcontrollers/client.controller.go` wrapping `CollectionFrame` from `backend/app/models/clientmodels/`:

```json
{
  "appVersion": "1.2.3",
  "serverName": "myapp-host-1",
  "collectionFrames": [
    {
      "traces": [
        {
          "id": "5b8e1a2f-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
          "endpoint": "GET /api/users",
          "duration": 45200000,
          "statusCode": 200,
          "recordedAt": "2024-01-15T10:30:00Z",
          "isTask": false,
          "attributes": {},
          "spans": []
        }
      ],
      "stackTraces": [
        {
          "stackTrace": "RuntimeError: connection refused\n  at ...",
          "recordedAt": "2024-01-15T10:30:00Z",
          "attributes": {"user_id": "123"},
          "isMessage": false,
          "isTask": false
        }
      ],
      "metrics": [
        {
          "name": "cpu.used_pcnt",
          "value": 45.2,
          "recordedAt": "2024-01-15T10:30:00Z",
          "tags": {}
        }
      ]
    }
  ]
}
```

Notes:
- Timestamps are `recordedAt` (RFC 3339), never `timestamp`. `duration` is a Go `time.Duration`, i.e. integer nanoseconds (45200000 = 45.2ms).
- Endpoints vs tasks share the `traces` array, split by `isTask`. `CollectionFrame` also carries `sessionRecordings` and `sessions`.
- Each entry in a trace's `spans` may carry an optional `parentSpanId` (UUID) and `attributes` (string map). A span that names no parent hangs under the run's own span. The ids are stored as 32 hex characters.
- **Unknown fields are silently ignored**: a payload in the wrong shape (e.g. top-level `metrics`) still returns 200 but inserts nothing. When hand-crafting test payloads, confirm ingestion landed via `POST /api/metrics/query` (or the relevant list endpoint) instead of trusting the status code.

---

## Common Patterns

### Adding a New API Endpoint

1. **Add model** in `backend/app/models/`
   ```go
   type NewEntity struct {
       ID        uuid.UUID `json:"id"`
       Name      string    `json:"name"`
       CreatedAt time.Time `json:"created_at"`
   }
   ```

2. **Add repository** in `backend/app/repositories/`
   ```go
   func (r *NewEntityRepository) GetAll(projectID uuid.UUID) ([]models.NewEntity, error) {
       // ClickHouse query
   }
   ```

3. **Add controller** in `backend/app/controllers/`
   ```go
   func (c *NewEntityController) List(ctx *gin.Context) {
       entities, err := repo.GetAll(projectID)
       ctx.JSON(200, entities)
   }
   ```

4. **Register route** in `backend/app/controllers/routes.go`
   ```go
   api.GET("/new-entities", newEntityController.List)
   ```

5. **Add frontend API call** in `frontend/src/lib/api.ts` or directly in page

**POST body convention:** List/search endpoints use POST with a JSON body containing filters and pagination:
```go
type ListRequest struct {
    ProjectId  string           `json:"projectId"`
    FromDate   string           `json:"fromDate"`
    ToDate     string           `json:"toDate"`
    OrderBy    string           `json:"orderBy"`
    Search     string           `json:"search"`
    Pagination PaginationParams `json:"pagination"`
}
```

**Paginated response:** Use `PaginatedResponse[T]` from `routes.go` for all paginated endpoints:
```go
type PaginatedResponse[T any] struct {
    Data       []T        `json:"data"`
    Pagination Pagination `json:"pagination"`
}

type Pagination struct {
    Page       int   `json:"page"`
    PageSize   int   `json:"pageSize"`
    Total      int64 `json:"total"`
    TotalPages int64 `json:"totalPages"`
}

type PaginationParams struct {
    Page     int `json:"page" binding:"min=1"`
    PageSize int `json:"pageSize" binding:"min=1,max=100"`
}
```

### Adding a New Frontend Page

1. **Create route folder**: `frontend/src/routes/new-page/`

2. **Add page component**: `+page.svelte` with the standard loading pattern:
   ```svelte
   <script lang="ts">
     import { onMount } from 'svelte'
     import { api } from '$lib/api'
     import { projectsState } from '$lib/state/projects.svelte'
     import { ErrorDisplay } from '$lib/components/ui/error-display'
     import { getErrorMessage, getErrorStatus } from '$lib/utils/errors'

     let data = $state<DataType[]>([])
     let loading = $state(true)
     let error = $state('')
     let notFound = $state(false)

     async function loadData() {
       loading = true
       error = ''
       try {
         const response = await api.post<ResponseType>('/endpoint', payload, {
           projectId: projectsState.currentProjectId ?? undefined
         })
         data = response.data || []
       } catch (e) {
         if (getErrorStatus(e) === 404) {
           notFound = true
         } else {
           error = getErrorMessage(e) || 'Failed to load data'
         }
       } finally {
         loading = false
       }
     }

     onMount(() => { loadData() })
   </script>

   {#if loading}
     <LoadingCircle size="xlg" />
   {:else if notFound}
     <ErrorDisplay status={404} title="Not Found" description="..." onRetry={() => loadData()} />
   {:else if error}
     <ErrorDisplay status={400} title="Error" description={error} onRetry={() => loadData()} />
   {:else}
     <!-- Content -->
   {/if}
   ```

3. **Add data loading** (optional): `+page.ts` for URL params
   ```typescript
   export const load = async ({ params }) => {
       return { param: params.id }
   }
   ```

4. **Add navigation** in `src/lib/components/app-sidebar.svelte`

### Adding a New Framework

**Backends do not get a new framework value.** OpenTelemetry is the single backend integration path, and `opentelemetry` is the only backend option in the project-creation picker (plus the preselected default). A new backend language or web framework is a *documentation and Connection-page* change, never a new project framework:

1. **Connection page targets** — `frontend/src/lib/utils/otel-setup.ts`: add the framework to the matching `OTEL_TARGETS` language entry (or add a new language target) and its setup steps
2. **Docs guide** — `docs/pages/client/otel/<framework>/`: create `_meta.json` and `index.mdx`, then list it in `docs/pages/client/otel/_meta.json` and the "Next Steps" block of `docs/pages/client/otel/index.mdx`
3. **Docs OTel language table** — `docs/pages/client/otel/index.mdx`: add a row
4. **Docs framework picker** — nothing to do. `docs/components/FrameworkPicker.jsx` carries exactly one backend card, **OpenTelemetry**, pointing at `/client/otel`, so it mirrors the dashboard's project-creation picker. Do not add a per-framework backend card; the guide is discovered from `/client/otel` instead.
5. **Combobox search keywords** — `frontend/src/lib/components/framework-combobox.svelte`: append the name to the OpenTelemetry entry's `keywords` so searching for it still finds the right option
6. **README** — add to the supported-frameworks table if it warrants a row

Only a **browser or mobile** framework needs a real new framework value, because those use platform SDKs with their own Connection-page content:

1. **Backend** — `backend/app/controllers/project.controller.go`: add to `validFrameworks` and update the validation error message
2. **Frontend state** — `frontend/src/lib/state/projects.svelte.ts`: add to the `Framework` union, `FRAMEWORK_LABELS`, and `MOBILE_FRAMEWORKS`/`FRONTEND_FRAMEWORKS`
3. **Frontend combobox** — `frontend/src/lib/components/framework-combobox.svelte`: add an entry to the `Browser` or `Mobile` group with `keywords`
4. **Frontend icon** — `frontend/src/lib/components/framework-icon.svelte`: add the icon mapping
5. **Framework code** — `frontend/src/lib/utils/framework-code.ts`: add install command, integration snippet, label, code language, and testing routes
6. **Connection page** — `frontend/src/routes/connection/+page.svelte`: add highlight language mapping and install description
7. **Dashboard page** — `frontend/src/routes/+page.svelte`: add highlight language mapping
8. **Docs** — `docs/pages/client/<framework>/`, `docs/pages/client/_meta.json`, `SDK_OPTIONS` + `FOLDER_SDK` in `docs/components/SdkContext.jsx`, `SDK_VISIBILITY` in `docs/theme.config.jsx`, `SDK_QUICK_START` in `docs/components/SdkSelector.jsx`, and a `FrameworkPicker.jsx` card

Values the picker does not offer (`gin`, `fiber`, `chi`, `fasthttp`, `stdlib`, `custom`, `nextjs`, `nestjs`, `express`, `remix`, `hono`, `cloudflare`, `symfony`, `laravel`, `django`) stay valid in `validFrameworks` and in `FRAMEWORK_LABELS`: existing projects carry them and must keep rendering. For the Go ones (`LEGACY_GO_FRAMEWORKS` in `projects.svelte.ts`) the home and Connection pages show the OpenTelemetry setup guide (`usesOtelSetupGuide`), opened on Go and the matching framework (`legacyOtelGuide` in `otel-setup.ts`).
