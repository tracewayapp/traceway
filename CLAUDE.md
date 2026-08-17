# CLAUDE.md - Traceway Project

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Traceway is an error tracking and monitoring platform consisting of:
- **Frontend**: SvelteKit 2 dashboard application with Svelte 5
- **Backend**: Go/Gin API server with ClickHouse database
- **CLI**: Go/Cobra command-line client for the backend HTTP API (`/cli`)

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
| Backend | `cd backend && go run .` | API server (port 8082) |
| CLI | `cd cli && just build` | Builds `bin/traceway` |
| CLI | `cd cli && just test` | Runs unit tests |
| CLI | `cd cli && just check` | Lint + test + vulncheck (pre-commit gate) |
| CLI | `cd cli && just smoke-test` | Live E2E (needs `TRACEWAY_SMOKE_*` env vars) |

### Tech Stack
- **Frontend**: SvelteKit 2.49, Svelte 5.45, Tailwind CSS v4, shadcn-svelte, Vite 7
- **Backend**: Go 1.25, Gin 1.11, ClickHouse, PostgreSQL
- **CLI**: Go 1.26, Cobra 1.10, separate Go module (`github.com/tracewayapp/traceway/cli`); flake.nix dev shell, justfile entrypoints
- **Client SDK**: Go 1.25, Gin middleware support

### go-lightning Library (PostgreSQL ORM)
- **Import**: `github.com/tracewayapp/go-lightning/lit`
- **Purpose**: Lightweight generic CRUD operations for PostgreSQL

#### Model Registration (required before use)
All models are registered centrally in `models/models.go` via `models.Init()`:
```go
func Init() {
    lit.RegisterModel[User](lit.PostgreSQL)
    lit.RegisterModel[Project](lit.PostgreSQL)
    // ...all models registered here
}
```
Repository-local result models (e.g., aggregate structs only used in one repo) can use file-level `init()` instead.

#### Naming Conventions
- Fields: CamelCase → snake_case (`FirstName` → `first_name`)
- Consecutive uppercase: stay together (`HTTPCode` → `http_code`)
- Tables: pluralize + snake_case (`User` → `users`)
- Override via struct tag: `lit:"custom_name"`

#### Core CRUD Operations
All lit functions take `*sql.Tx` as the first argument for transactional consistency:

| Function | Description |
|----------|-------------|
| `lit.Insert[T](tx, &entity)` | Insert, returns auto-generated int ID |
| `lit.InsertUuid[T](tx, &entity)` | Insert with auto-generated UUID |
| `lit.InsertExistingUuid[T](tx, &entity)` | Insert with pre-set UUID |
| `lit.Select[T](tx, query, args...)` | Retrieve multiple records (returns `[]*T`) |
| `lit.SelectSingle[T](tx, query, args...)` | Retrieve one record (returns `*T`) |
| `lit.Update[T](tx, &entity, "id = $1", id)` | Update (auto-prepends WHERE) |
| `lit.UpdateNative(tx, "UPDATE table SET col = $1 WHERE ...", args...)` | Raw SQL update for partial/single-field changes |
| `lit.Delete(tx, "DELETE FROM table WHERE id = $1", id)` | Delete records |

#### Transaction Helper (`pgdb.ExecuteTransaction`)
All PostgreSQL operations should use `ExecuteTransaction` for automatic commit/rollback:

```go
// ExecuteTransaction[T] wraps a function in a transaction
// - Commits on success, rolls back on error or panic
// - Returns (T, error) directly - no pointer wrapping

project, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
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

    ctx.JSON(201, user)  // Transaction auto-commits on 200/201/303
}
```

**Auto-commit/rollback behavior:**
- Commits on status codes: 200, 201, 303
- Rolls back on all other status codes or panics

**Preference:** For CRUD controller methods, always prefer using `middleware.Transactional` in the route + `db.GetTx(ctx)` in the controller over `pgdb.ExecuteTransaction`. The middleware approach keeps controllers flat, avoids nested closures, and follows the established pattern. (The tx getter lives in the `db` package: `db.GetTx(ctx)`, not `middleware.GetTx`.)

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
```
JWT_SECRET=<min 32 char secret for JWT signing>
APP_BASE_URL=                         # public origin of this server (e.g. https://traceway.example.com). Used as the OAuth issuer / device verification URL and SSO redirect base. If unset, the device-auth + well-known endpoints derive it per-request from the Host / X-Forwarded-* headers; set it explicitly behind a proxy that doesn't forward Host.
CLICKHOUSE_SERVER=localhost:9000
CLICKHOUSE_DATABASE=traceway
CLICKHOUSE_USERNAME=default
CLICKHOUSE_PASSWORD=
CLICKHOUSE_TLS=false
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DATABASE=traceway
POSTGRES_USERNAME=traceway
POSTGRES_PASSWORD=
POSTGRES_SSLMODE=disable

# DuckDB telemetry backend (only with -tags telemetry_duckdb build; see "DuckDB Telemetry Backend" below)
DUCKDB_MEMORY_LIMIT=                  # e.g. 4GB. Unset = DuckDB auto-tunes (~80% RAM). Set explicitly in memory-capped containers to avoid OOM.
DUCKDB_THREADS=                       # e.g. 4. Unset = DuckDB auto-tunes (= cores). Cap in constrained/shared environments.
DUCKDB_CHECKPOINT_THRESHOLD=          # e.g. 256MB. Unset = DuckDB default (16MB). Raise under sustained ingest to reduce WAL checkpoint stalls; costs a larger WAL and longer restart replay.

# Ingest admission gate (all telemetry ingest endpoints: /api/report, /api/profiles/ingest, /api/otel/*)
INGEST_MAX_CONCURRENT=                # max concurrently processed ingest requests. Unset = 2×CPU cores, min 4. Bounds ingest memory so overload sheds load with 503s instead of the process being OOM-killed (on DuckDB an OOM death is followed by a minutes-long WAL-replay stall on restart).
INGEST_ADMISSION_WAIT_SECONDS=5       # how long a request may wait for a slot before the 503 + Retry-After; 0 = reject immediately when saturated

# Notifications
NOTIFICATION_POLL_SECONDS=60          # polled rule evaluation interval; minimum 5, invalid values fall back to 60
ONCALL_POLL_SECONDS=30                # on-call escalation worker interval; minimum 5, invalid values fall back to 30. Kept separate from NOTIFICATION_POLL_SECONDS so raising rule-evaluation intervals never delays paging. A buffered Wake() channel makes freshly opened pages notify L1 near-instantly regardless of this interval.
OUTBOX_POLL_SECONDS=15                # notification outbox drain interval; minimum 5, invalid values fall back to 15. The outbox (backend/app/outbox, notification_outbox table in the main DB) is the persist-then-send layer for ALL notifications: rule dispatch and the escalator only enqueue (with an adapter-config snapshot) inside their transactions; the drain worker sends with retries (backoff 1m/5m/15m/60m, 5 attempts, then terminal failed + CaptureException). Crash-safe at-least-once: stale 'sending' rows are reclaimed after 5 min, cancelled rows can never resurrect (guarded status transitions), ack/resolve cancels queued page deliveries via outbox.CancelByKey. Cooldown and event-rule dedup record at enqueue commit (the durable promise), and fired_notifications is written at the terminal outcome. /api/health/deep exposes an `outbox` block; `traceway.outbox.*` metrics are emitted when monitoring is on; terminal rows are pruned daily (sent/cancelled 7d, failed 30d).

# Synthetics (synthetic uptime monitoring: backend/app/synthetics, /synthetics frontend route)
SYNTHETICS_POLL_SECONDS=15            # scheduler tick for due checks; minimum 5, invalid values fall back to 15. The scheduler enqueues due checks into the check_runs queue (main DB, outbox-style guarded claims, advisory lock 824737004) and records expired queued runs as `missed` in telemetry — a probe is never executed late. In-process executors claim http/tcp runs always, browser runs only when mode=embedded.
SYNTHETICS_BROWSER_MODE=off           # off | embedded | remote. Browser checks are real @playwright/test specs executed by spawning Node against a harness dir (no npm/npx at runtime; allowlisted env so user scripts never see server secrets). `embedded` requires the :browser image (Dockerfile.browser, DuckDB base + Node + Chromium) and fails fast at startup otherwise; `remote` queues browser runs for traceway-runner binaries that long-poll /api/runners/poll authenticating with SYNTHETICS_RUNNER_SECRET. Hard-blocked in cloud mode (startup panic + 422 at check creation).
SYNTHETICS_RUNNER_SECRET=             # shared bearer credential for the operator's runner fleet; required for remote mode (fail-fast at startup). Runners are deployment infrastructure, NOT tenant entities: no dashboard CRUD, no per-runner tokens. A runner self-registers a liveness row in synthetic_runners under its X-Traceway-Runner-Name on first poll (upsert throttled to 1/min via an in-memory cache; claim identity is "runner:<name>"). Rotation = change the secret + restart runners. Runner-side env: TRACEWAY_URL, TRACEWAY_RUNNER_SECRET, TRACEWAY_RUNNER_NAME (default hostname), RUNNER_WORKERS (default 2, max 16), and optional TRACEWAY_RUNNER_MONITORING (<project_token>@<url>/api/report) which self-instruments the runner with the Traceway Go SDK: default server metrics, a "browser_run" task trace per executed run (tags check_id/run_id/status, server_name = runner name), and CaptureException on infra failures (start/report errors, per-job panics recovered without killing the worker).
HEALTH_DEEP_TOKEN=                    # operator bearer secret gating GET /api/health/deep (its payload is instance-wide: cross-tenant queue depth, runner fleet, storage engine stats). Unset = endpoint disabled with a 401 pointing here.
SYNTHETICS_HTTP_CONCURRENCY=8         # concurrent in-process http/tcp probes
SYNTHETICS_BROWSER_CONCURRENCY=2      # concurrent Chromium instances in embedded mode (~300-500MB each)
SYNTHETICS_ALLOW_PRIVATE_TARGETS=true # "false" rejects checks that resolve to private/LAN addresses, validated at save AND at dial time for both http and tcp probes (netguard.GuardedDialContext dials the vetted IP, DNS-rebinding safe). Forced false in cloud mode regardless of env (tenants must never probe the platform's network). Default allow elsewhere: probing LAN services is a core self-hosted use case. When the guard is off, http probes honor HTTP(S)_PROXY; when on, the proxy is ignored so the guard vets the real target. Check creation also runs controllers.CheckLimitHook (nil = unlimited; cloud caps monitors per org plan).
SYNTHETICS_PLAYWRIGHT_DIR=/opt/traceway-playwright   # Playwright harness dir (node_modules with pinned @playwright/test; docker/playwright/package.json pins the version)
SYNTHETICS_SCREENSHOT_RETENTION_DAYS=30 # browser failure artifacts under STORAGE_PATH/synthetics/ (.png screenshots AND .log Playwright output tails — failed browser runs store both, referenced by screenshot_key/output_key on check_results, served via GET /api/synthetics/screenshot|output?key= with project prefix checks); local storage only, 0 disables the cleanup worker

# Retention (see "Data Retention" section below)
SQLITE_RETENTION_DAYS=30              # 0 to disable; only applies in SQLite mode
DUCKDB_RETENTION_DAYS=30              # telemetry TTL on the DuckDB backend; when set it wins over SQLITE_RETENTION_DAYS there, when unset SQLITE_RETENTION_DAYS applies as fallback; 0 to disable
LOG_RECORDS_MAX_ROWS=                 # optional cap on log_records rows; unset/0 = disabled. NOT a hard limit: a cleanup worker trims to the newest N rows once per minute, so ingest above N rows/minute overshoots the cap between runs. Only applies in SQLite mode (SQLite or DuckDB telemetry)
SESSION_RECORDING_RETENTION_DAYS=30   # 0 to disable; only applies when STORAGE_TYPE=local
PROFILE_ARCHIVE_RAW=false             # native pprof ingest only: write the original pprof bytes to object storage as a lossless archive
PROFILE_RETENTION_DAYS=30             # 0 to disable; on-disk archive TTL, only with PROFILE_ARCHIVE_RAW + STORAGE_TYPE=local

# Session recording uploads (see "Session Recording Uploader" section below)
SESSION_RECORDING_UPLOAD_WORKERS=32   # 0 to disable uploads entirely
SESSION_RECORDING_UPLOAD_QUEUE_SIZE=2048

# Source map symbolicator
SYMBOLICATOR_PARSER=goja              # goja (default) or oxc (requires -tags oxc build, see scripts/build-oxc-shim.sh)
SOURCEMAP_CACHE_TYPE=memory           # memory (default) or disk (mmap-backed .tw cache)
SOURCEMAP_DISK_CACHE_PATH=./twcache   # only used when SOURCEMAP_CACHE_TYPE=disk
SOURCEMAP_DISK_CACHE_MAX_MB=2048      # capacity-based LRU eviction of local .tw files
```

---

## Architecture Overview

### Data Flow
```
Go Application → [traceway SDK] → GZIP POST /api/report → Backend → ClickHouse
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
/dashboards                 Dashboards page (tabs of org dashboards; /metrics redirects here)
/monitors                   Monitors (synthetic checks): single sidebar item, TabsRow tabs Monitors | Status Pages (?tab=, Status Pages admin-only); status pages tab has branding (description, logo upload, custom domain); old /monitors/status-pages redirects to the tab, /monitors/runners to /monitors (runners have no UI — operator infra)
/monitors/[checkId]         Monitor detail (latency chart, uptime bars, runs w/ result filter, incidents)
/status/[slug]              Public status page (light standalone design, no auth, raw fetch, listed in isPublicPath)
/connection                 SDK integration guide
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
├── main.go                     # Entry point, DB init, server start
├── app/
│   ├── controllers/
│   │   ├── routes.go           # Route registration
│   │   ├── dashboard.go        # Dashboard metrics
│   │   ├── auth.go             # Login handler
│   │   ├── projects.go         # Project CRUD
│   │   └── clientcontrollers/
│   │       └── report.go       # Telemetry ingestion (/api/report)
│   ├── repositories/           # ClickHouse queries
│   │   ├── transactions.go
│   │   ├── exceptions.go
│   │   ├── metrics.go
│   │   └── projects.go
│   ├── models/                 # Data structures
│   ├── middleware/
│   │   ├── auth.go             # Token validation
│   │   └── gzip.go             # Request decompression
│   ├── cache/                  # In-memory project token cache
│   ├── pgdb/                   # PostgreSQL connection manager
│   └── migrations/
│       ├── ch/                 # ClickHouse migrations
│       └── pg/                 # PostgreSQL migrations
```

### Middleware Chain Composition

| Route Type | Middleware Chain |
|-----------|-----------------|
| Read-only telemetry | `UseAppAuth, RequireProjectAccess` |
| Write telemetry | `UseAppAuth, RequireProjectAccess, RequireWriteAccess` |
| PostgreSQL CRUD (read) | `UseAppAuth, RequireProjectAccess, Transactional` |
| PostgreSQL CRUD (write) | `UseAppAuth, RequireProjectAccess, RequireWriteAccess, Transactional` |
| Admin org management | `UseAppAuth, RequireAdminAccess, Transactional` |
| Public (auth/invitations) | `Transactional` only |
| Client SDK ingestion | `CORSReport, UseClientAuth, UseGzip` |
| Synthetic runner API | `UseRunnerAuth` only (shared SYNTHETICS_RUNNER_SECRET bearer + self-registration by X-Traceway-Runner-Name; no Transactional — poll holds the request up to 25s) |
| Public status page | `RateLimitPerIP` only |

### API Endpoints

**Client SDK Ingestion**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/report` | Client | Telemetry ingestion (gzipped) |
| POST | `/api/otel/v1/traces` | Client | OTLP/HTTP trace ingestion |
| POST | `/api/otel/v1/metrics` | Client | OTLP/HTTP metric ingestion |
| POST | `/api/otel/v1/logs` | Client | OTLP/HTTP log ingestion |
| POST | `/api/otel/v1development/profiles` | Client | OTLP/HTTP profile ingestion (development signal) |

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
| POST | `/api/auth/device/token` | None | Poll for the token (grant `device_code`); also accepts `refresh_token` |
| POST | `/api/auth/token` | None | Token endpoint: `device_code` or `refresh_token` grant (JSON or form-encoded) |
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
| GET/POST/DELETE | `/mcp` | Bearer | Streamable HTTP MCP server (origin root); 401s carry a `WWW-Authenticate` resource-metadata challenge |

**Personal Access Tokens**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/personal-access-tokens` | App | Create a PAT (returns the `twp_` token once) |
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
| POST | `/api/metrics/query` | App | Custom metric queries |
| GET | `/api/metrics/discover` | App | Discover available metrics |
| GET | `/api/metrics/discover/tags` | App | Discover metric tags |
| PUT | `/api/metrics/registry` | App+Write | Update metric registry entry |

**Dashboards & Templates**

Dashboards are org-owned JSON documents (`{schemaVersion, widgets: [{id, title, widgetType, config}]}`, widget ids are server-generated `w_xxxxxxxx` strings, array order = display order) applied to projects via `project_dashboards`. Dashboard mutations require org role above `readonly` (checked in-handler); project-scoped routes (list/star/reorder/populate) use the standard middleware chains; apply/unapply also check the effective role of each affected project. The old per-project widget-group tables are converted once at startup by `backfill.RunDashboards` (advisory-locked on PG) and retained for rollback.

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

Templates are DB rows seeded by migrations (`traceway-otel-agent` for the OTel host agent, `golang` for Go SDK apps, `traceway-clickhouse`/`traceway-duckdb` for the telemetry stores of a monitored Traceway instance; SQLite emits no store-specific metrics so it has no template); cloud can insert more rows without a release. The OTLP metric ingest allowlists per-resource grouping tags (`container.name`, `k8s.pod.name`, `k8s.node.name`, `postgresql.database.name`, ...) in `otelcontrollers/metric_converter.go` for custom widgets.

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

One `ai_traces` row per LLM call (any OTLP span with `gen_ai.*` attributes). Each row carries a `conversation_id` resolved at ingest (`gen_ai.conversation.id` -> `session.id` span/resource attr -> distributed trace id -> empty), `tool_call_count`/`tool_names` parsed from the completion payload (OpenAI `tool_calls`, Anthropic `tool_use`, OTel output messages, `gen_ai.tool.*` fallback), and `flagged`/`flagged_terms` from an ingest-time word-boundary scan of prompt+completion against per-project selected built-in language packs (`projects.ai_flagged_languages`, default `["en"]`; packs live in `backend/app/services/contentflag/terms/*.txt` — en/de/es/fr/it/pt/sr; empty array = custom terms only) plus per-project custom terms (`projects.ai_flagged_terms`); both edited in the project settings AI tab and cached via the project cache. All three project-creation paths (`Create`, `CreateWithOrganization`, `cmd/seed.go`) must set `AiFlaggedLanguages` explicitly since lit inserts every struct field. `tool_names`/`flagged_terms` are stored comma-separated (values sanitized at ingest); the content matcher lives in `backend/app/services/contentflag/`. Conversation analytics exclude rows with an empty `conversation_id`; user analytics additionally require a non-empty `user_id`.

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/ai-traces/grouped` | App | AI traces grouped by trace name |
| POST | `/api/ai-traces/trace` | App | Calls for one trace name (`?traceName=`) |
| POST | `/api/ai-traces/:traceId` | App | Single call detail + conversation blob |
| POST | `/api/ai-conversations/grouped` | App | Conversations (GROUP BY conversation_id): turns, cost, tokens, tools, models, flagged; filters userId/model/toolName/flaggedOnly/search (search matches conversation id, user, model, tool names, and flagged terms; row-level filters are semi-joins on conversation_id so a match on any turn returns the whole conversation's aggregates); response also carries `thresholds` (range-wide P95 cost/turns for outlier highlighting) and `facets` (models, tools) |
| POST | `/api/ai-conversations/conversation` | App | All turns of one conversation (id in body) ordered by recorded_at, each with its stored input/output payload (capped at 200 turns of payloads), plus stats |
| POST | `/api/ai-users/grouped` | App | Per-user conversation analytics: conversation count, total calls, avg/min/median turns, avg cost per conversation, total cost, flagged conversation count |

Frontend routes: `/ai-traces` (tabs: Traces, Conversations, Users), `/ai-traces/conversations/[conversationId]` (chat timeline with tool calls rendered). Trace names `conversations` and `users` are shadowed by these static routes. Notification rule types `ai_trace_cost` (per call), `ai_conversation_cost` (24h cumulative per conversation), and `ai_flagged_content` (flagged term match, optional term filter) are event-driven; remember both `notification_rule.repository.go` copies list event rule types explicitly.

**Organization Management**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/organizations/:orgId/settings` | Admin | Get org settings |
| PUT | `/api/organizations/:orgId/settings` | Admin | Update org settings |
| GET | `/api/organizations/:orgId/members` | Admin | List members |
| PUT | `/api/organizations/:orgId/members/:userId` | Admin | Update member role |
| DELETE | `/api/organizations/:orgId/members/:userId` | Admin | Remove member |
| GET | `/api/organizations/:orgId/members/:userId/project-roles` | Admin | List member's per-project role overrides |
| PUT | `/api/organizations/:orgId/members/:userId/project-roles/:projectId` | Admin | Set/clear a per-project role override |

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

**Monitors** (user-facing name for synthetic uptime checks; engine in `backend/app/synthetics`, frontend under `/monitors`, see the SYNTHETICS_* env block)

Checks (`synthetic_checks`, main DB) are http/tcp/browser probes with per-check interval, timeout, and a consecutive-failure threshold (flap damping). Runs flow through the `check_runs` queue (outbox-style guarded claims, terminal rows deleted, expired queued runs recorded as `missed`); results are telemetry (`check_results`, all three backends, pruned by the SQLite retention worker / 90d CH TTL). State transitions open/resolve `check_incidents` and feed the event-driven notification rule type `check_down` (recovery auto-resolves the page a rule-scoped dedup key opened via `oncall.AutoResolveByDedupKey`; recovery never dispatches to escalation channels). The notify hook runs post-commit with no ambient tx (SQLite single-connection). Remember: `notification_rule.repository.go` (both copies) lists event rule types explicitly, and `check_down` is one of them.

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
| POST | `/api/runners/poll` | Runner (shared secret) | Instance-wide long-poll claim of browser runs (25s hold on the wake channel, NO Transactional, excluded from tracewaygin self-monitoring); self-registers the runner's liveness row |
| POST | `/api/runners/results/:runId` | Runner (shared secret) | Report an outcome (MaxBytesReader 6MB incl. screenshotBase64; idempotent: lost/missing claim = 200 no-op; claims keyed "runner:<name>") |
| GET/POST/PUT/DELETE | `/api/organizations/:organizationId/status-pages(/:id)` | Member / Admin | Status page CRUD (slug `[a-z0-9-]{3,60}` unique, checkIds validated against the org; branding fields `description`, `customDomain` unique hostname) |
| POST | `/api/organizations/:organizationId/status-pages/:id/logo` | Admin | Upload a PNG/JPEG logo (raw body, 1MB cap, sniffed content type; SVG rejected as scriptable) stored via storage.Store under `statuspages/<id>/logo` |
| GET | `/api/status/:slug/logo` | None (rate-limited) | Streams a public page's logo |
| GET | `/api/status-domains/resolve` | None (rate-limited) | Maps the request Host header to a public status page slug — backs CNAMEd vanity domains (TLS terminates at the operator's proxy); the SPA calls it for anonymous visits to `/` |
| GET | `/api/status/:slug` | None (rate-limited) | Public status payload: per-check status + 90 daily uptime buckets + incidents; cached ~30s per slug; private/unknown slugs are both 404; latency values stripped. Frontend page: `/status/[slug]` (in `isPublicPath`) |

**Logs**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/logs` | App | List/search logs (filters: severity, service, trace, body) |

**Source Maps**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/sourcemaps/upload` | SourceMap | Upload source map file |

### Data Ingestion Flow (`/api/report`)

1. **Gzip Middleware**: Decompresses request body (SDK sends gzipped data)
2. **Auth Middleware**: Validates `Authorization: Bearer <project_token>`
3. **Parse Frame**: JSON decode into `models.Frame` (transactions, exceptions, metrics)
4. **Batch Insert**: Repository methods insert batches into ClickHouse

```go
// backend/app/controllers/clientcontrollers/report.go
func (c *ReportController) Report(ctx *gin.Context) {
    var frame models.Frame
    ctx.BindJSON(&frame)

    // Insert each data type
    transactionRepo.BatchInsert(frame.Transactions)
    exceptionRepo.BatchInsert(frame.Exceptions)
    metricRepo.BatchInsert(frame.Metrics)
}
```

### Database Schema

#### Tables (ClickHouse)
| Table | Purpose | Partitioning |
|-------|---------|--------------|
| `transactions` | HTTP request metadata | Monthly (`toYYYYMM(timestamp)`) |
| `exception_stack_traces` | Exceptions with stack traces | Monthly |
| `metric_records` | Time-series system metrics | Monthly |
| `endpoints` | Endpoint aggregates (materialized) | None |
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
| `check_incidents` | Down/up incident spans per check (feeds status pages) |
| `synthetic_runners` | Liveness registry for self-registered runner fleet (unique name, version, first/last_seen_at; no credentials, no org scoping) |
| `status_pages` | Public uptime pages (org-scoped slug + selected check ids + branding: description, logo_key, unique custom_domain) |
| `widget_groups` / `widget_group_widgets` / `starred_widgets` | Legacy pre-dashboards tables, retained read-only for rollback until a follow-up drop |

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

**Main DB tables** (`db.DB` — transactional, uses lit with `*sql.Tx`):
- `users`, `organizations`, `organization_users`, `projects`, `invitations`
- `source_maps`, `metric_registry`, `dashboards`, `project_dashboards`, `dashboard_templates`, `starred_dashboard_widgets` (plus the legacy `widget_groups`/`widget_group_widgets`/`starred_widgets`)
- `notification_channels`, `notification_rules`
- `synthetic_checks`, `check_runs`, `check_incidents`, `synthetic_runners`, `status_pages`

**Telemetry DB tables** (`db.TelemetryDB` — non-transactional, uses lit with `db.TelemetryDB` directly):
- `endpoints`, `tasks`, `exception_stack_traces`, `spans`, `metric_points`
- `session_recordings`, `archived_exceptions`, `slow_endpoints`, `fired_notifications`, `check_results`

**How to access each database in repository code:**

```go
// Main DB (PostgreSQL replacement) — use transactions via middleware or ExecuteTransaction
// Repositories receive *sql.Tx from middleware.GetTx(ctx) or db.ExecuteTransaction
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

Built with `-tags telemetry_duckdb` (`CGO_ENABLED=1` required), this is an alternative telemetry store for the same `DB_TYPE=sqlite` deployment: the **main DB stays SQLite** (`db.DB`, relational/config), while the **telemetry DB becomes DuckDB** (`db.TelemetryDB`, columnar). It exists because DuckDB's columnar engine is dramatically faster on the analytics/aggregation reads the dashboard issues — at 10M rows it clears read-probe thresholds that SQLite times out on. Backends are selected on two build-tag axes: `telemetry_ch` / `telemetry_duckdb` / *(none = SQLite telemetry)* for the telemetry store and `transactional_pg` / *(none = SQLite main)* for the relational store. Only three combinations are supported — *(no tags)* dual SQLite, `telemetry_duckdb`, and `transactional_pg telemetry_ch` — enforced by compile-time guard files in `backend/app/db/` (stale `pgch`/`duckdb`/`oltp_*` tags also fail with a rename message). Repositories are organized on the same two axes: telemetry repositories live in per-backend packages `backend/app/repositories/telemetry/{clickhouse,sqlite,duckdb}/` and transactional (relational) repositories in `backend/app/repositories/transactional/{pg,sqlite}/`, each re-exported as singletons through tag-guarded facade files at the axis package root (`telemetry/telemetry_ch.go` etc., `transactional/transactional_pg.go` etc.). Consumers import the facade packages — `telemetry.SpanRepository`, `transactional.UserRepository` — never a backend package directly. Helpers shared by all telemetry backends are in `telemetry/shared/`, the SQLite scan/value types shared by the sqlite+duckdb backends are in `telemetry/sqlitetypes/`, and helpers/types shared by the transactional backends (auth-token hashing/time formats, facade-crossing structs) are in `transactional/shared/`. The `transactional/pg` and `transactional/sqlite` implementations are intentionally kept dialect-neutral (lit `:name` queries rendered per `db.Driver`), enforced byte-for-byte by `transactional/parity_test.go`. Running Postgres requires the `transactional_pg` build: the default build's migration runner applies SQLite-dialect migrations unconditionally, so `DB_TYPE=postgres` without the tag is not a supported combination.

- **Driver:** `github.com/duckdb/duckdb-go/v2` (the official driver; marcboeker/go-duckdb is deprecated). Bundles prebuilt static libs for glibc only — **not musl/Alpine**, so the image uses Debian (`Dockerfile.duckdb`).
- **Opened in** `backend/app/db/db_telemetry_duckdb.go`: telemetry path is the SQLite path with `.db` swapped for `_telemetry.duckdb`. By default DuckDB auto-tunes to the host; `DUCKDB_MEMORY_LIMIT`/`DUCKDB_THREADS`/`DUCKDB_CHECKPOINT_THRESHOLD` (passed through as DSN config options) let operators cap memory/threads so a memory-capped container doesn't read the host's RAM and OOM-kill the backend, and raise the WAL checkpoint threshold (default 16MB) so sustained Appender ingest isn't stalled by frequent checkpoints. `preserve_insertion_order=false` is always set — telemetry reads all have explicit ORDER BY, and dropping the guarantee lets DuckDB parallelize bulk loads and large scans with less memory. The read pool is bounded (`SetMaxOpenConns(duckDBMaxReadConns)`) since each DuckDB connection can use all threads + its own query memory; Appender writes use their own `DuckDBConnector.Connect()` connections and bypass that cap. Exposes `db.DuckDBConnector` (needed for the Appender).
- **Writes use the Appender API**, not `INSERT` (`duckdb.NewAppenderFromConn(conn, "", table)` → `AppendRow(...)` → `Close()` flushes). Upserts still go through `ExecContext` with `ON CONFLICT`. The Appender rejects typed `*string` for nullable VARCHAR — use `nullableString()` in `backend/app/repositories/telemetry/duckdb/helpers.go` (returns untyped `nil` or the dereferenced value).
- **Write-path observability:** a row the Appender rejects is dropped rather than failing the whole frame (the SQLite backend 500s instead), so a poison row cannot wedge the SDK's retry loop. Every drop increments a per-table counter (`db.RecordTelemetryRowDropped`) and fires a rate-limited (1/min per table) `traceway.CaptureException`; Appender flush/connect failures still propagate to the request (500, SDK retries) and increment an insert-failure counter. `GET /api/health/deep` (operator endpoint: requires the HEALTH_DEEP_TOKEN bearer secret, unset disables it; all telemetry backends) exposes `telemetryBackend`, `droppedRows` per table, `droppedRowsTotal`, `insertFailures`, `ingestRejected` (requests turned away by the ingest admission gate), and on DuckDB an `engine` object (db/WAL file bytes, `duckdb_memory()` usage, read-pool in-use/wait stats) alongside its existing ClickHouse fields; it 503s only when the configured telemetry backend is ClickHouse and CH is unreachable (the embedded backends answer 200 with `chReachable:false`). The benchmark loadgen polls it before/after every ramp step and fails any step whose drop delta is nonzero; read-probe fills record cumulative `droppedRows` per fill level. When `MONITORING_TRACEWAY_URL` is set, `monitoring.StartTelemetryDBReporter` also emits `traceway.duckdb.*` metrics every 10s: `rows_dropped.delta`, `insert_failures.delta`, `db_size_mb`, `wal_size_mb`, `memory_used_mb`, `read_pool.in_use`, `read_pool.wait_count.delta`, `read_pool.wait_ms.delta`. The hourly retention worker issues a `CHECKPOINT` after its deletes so retention actually reclaims disk (DuckDB otherwise defers reclamation to the WAL checkpoint threshold).
- **`lit` placeholders:** `db.Driver` stays `lit.SQLite`, which emits `?` — DuckDB accepts these, so no separate driver was needed for reads.
- **Migrations:** `backend/app/migrations/duckdb_telemetry/` (mirrors `sqlite_telemetry/` table-for-table; integer columns are `BIGINT`, JSON is `VARCHAR`, no secondary indexes since it's columnar).
- **Dialect gotchas vs SQLite** (the read queries differ): native `quantile_cont(col, p)` for P50/P95/P99 instead of fetch-and-sort; `strftime('%s',col)`→`epoch(col)`; time bucketing via `time_bucket(to_seconds(N), col, TIMESTAMP '1970-01-01')` — the explicit epoch origin is required because DuckDB anchors sub-day buckets at 2000-01-03 by default, which would misalign chart buckets against the SQLite backend's epoch-floored buckets for any interval that doesn't evenly divide a day; `json_extract`→`json_extract_string`; `json_each`→`LATERAL unnest(json_keys(x))`; strict GROUP BY needs `ANY_VALUE`/`arg_max`; `SUM` returns HUGEINT (CAST to BIGINT); `CAST(.. AS REAL)`→`CAST(.. AS DOUBLE)`.
- **Tests:** `backend/app/repositories/telemetry/testhelper_duckdb_test.go` (tagged `telemetry_duckdb`) provides `setupTestDB` so the entire existing telemetry test suite runs against an in-memory DuckDB.

#### Data Retention

Retention is handled in several different ways depending on the deployment.

**0. Main-DB auth prune — `retention.Start` worker** (`backend/app/retention/auth_tokens.go` + `oauth_sessions.go`, sharing `startDBPruneWorker` in `prune_worker.go`). Runs in **all modes** (Postgres and SQLite) against `db.DB`: once at startup, then every 24h, deleting expired/consumed auth rows. `auth_tokens` prunes expired `device_authorizations` (also pruned opportunistically on every `/api/auth/device/authorize` call, so the daily worker is a backstop there), `refresh_tokens` that are expired, revoked, or used more than 30 days ago (used rows are kept a month for replay detection, then dropped to bound per-user growth), plus revoked-or-expired `personal_access_tokens` (PAT expiry was otherwise only enforced lazily at read time). `oauth_sessions` prunes expired SSO login sessions. No env var — always on. (Refresh-token families and PATs also have explicit revoke paths: `POST /api/auth/logout` and the account PAT UI.)

**1. ClickHouse — `TTL` clauses on the table itself.** Only a few tables have a TTL; everything else is kept indefinitely (operators can drop monthly partitions manually if needed).

| Table | Retention | Source migration |
|-------|-----------|------------------|
| `metric_points` (raw) | **7 days** | `0034_add_ttl_metric_points.up.sql` |
| `metric_points_1m` (1-min rollup) | **30 days** | `0035_add_ttl_metric_points_1m.up.sql` |
| `metric_points_1h` (1-hour rollup) | **1 year** | `0036_add_ttl_metric_points_1h.up.sql` |
| `log_records` | **90 days** | `0075_increase_ttl_log_records.up.sql` (raised from 30d set in `0045_create_log_records.up.sql`) |
| `profiling_samples` (the bulk) | **30 days** | `0068_add_ttl_profiling_samples.up.sql` |
| `profiles` (slim metadata) | **30 days** | `0069_add_ttl_profiles.up.sql` |
| `profiling_stacks` (dedup table) | **30 days** | `0070_add_ttl_profiling_stacks.up.sql` |
| `check_results` (synthetic check probes) | **90 days** | `0083_add_ttl_check_results.up.sql` |
| All other CH tables (`transactions`, `exception_stack_traces`, `tasks`, `spans`, `sessions`, `ai_traces`, `session_recordings`, `fired_notifications`, `archived_exceptions`, `slow_endpoints`, `endpoints`, etc.) | **No TTL — retained indefinitely** | — |

The three profiling tables share a 30-day TTL keyed on each table's time column (`start_time` / `recorded_at` / `last_seen`). `profiling_stacks` is a `ReplacingMergeTree(last_seen)` dedup table, so `last_seen` is bumped on every re-ingest that references a stack — a stack only ages out once it has gone unreferenced for the full window, which is exactly when its samples have also expired, so no sample is ever left pointing at a dropped stack.

**2. SQLite — `retention.Start` worker** (`backend/app/retention/sqlite.go`). In SQLite mode, neither `db.DB` nor `db.TelemetryDB` has any built-in expiry, so a background worker fires once at startup and then every hour and runs a `DELETE FROM <table> WHERE <time_column> < cutoff` against each telemetry table.

| Variable | Default | Notes |
|----------|---------|-------|
| `SQLITE_RETENTION_DAYS` | `30` | TTL in days. Set to `0` to disable the worker entirely. Has no effect outside SQLite mode. |
| `DUCKDB_RETENTION_DAYS` | `30` | Same worker on the DuckDB telemetry backend. When set it takes precedence there; when unset the worker falls back to `SQLITE_RETENTION_DAYS` (selection in `retention.telemetryRetentionConfig`). |

Tables it prunes (and the column used):

| Database | Table | Time column |
|----------|-------|-------------|
| Telemetry (`db.TelemetryDB`) | `endpoints`, `tasks`, `exception_stack_traces`, `spans`, `metric_points`, `session_recordings`, `ai_traces` | `recorded_at` |
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
| `traceway.recordings.uploaded` | counter | Successful S3/local + DB writes since startup. |
| `traceway.recordings.dropped` | counter | Segments dropped on overflow since startup. |
| `traceway.recordings.failed` | counter | Upload or DB-insert errors since startup. |

Sustained drops also fire a rate-limited (1/min) `traceway.CaptureException` so overload is visible in the issues feed without flooding it.

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

#### Key Columns - transactions
```sql
project_id UUID,
timestamp DateTime64(3),
trace_id String,
endpoint String,           -- normalized: "GET /api/users"
duration_ms Float64,
status_code UInt16,
app_version String,
server_name String,
-- Indexes: bloom_filter(trace_id), tokenbf_v1(endpoint)
```

#### Key Columns - exception_stack_traces
```sql
project_id UUID,
timestamp DateTime64(3),
hash String,               -- normalized hash for grouping
type String,               -- error type (e.g., "RuntimeError")
value String,              -- error message
stacktrace String,         -- full stack trace
tags Map(String, String),  -- contextual tags from scope
```

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

#### Batch Insert (ClickHouse)
```go
func (r *TransactionRepository) BatchInsert(txns []models.Transaction) error {
    batch, _ := r.db.PrepareBatch(ctx, "INSERT INTO transactions ...")
    for _, txn := range txns {
        batch.Append(txn.ProjectID, txn.Timestamp, ...)
    }
    return batch.Send()
}
```

#### Aggregation with Quantiles
```go
// P50, P95, P99 percentiles
query := `
    SELECT
        endpoint,
        count() as count,
        quantile(0.5)(duration_ms) as p50,
        quantile(0.95)(duration_ms) as p95,
        quantile(0.99)(duration_ms) as p99
    FROM transactions
    WHERE project_id = ? AND timestamp BETWEEN ? AND ?
    GROUP BY endpoint
    ORDER BY count DESC
`
```

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

// WRONG - Do not use log.Printf for errors
if err != nil {
    log.Printf("Failed to read session recording (key=%s): %v", key, err)
}
```

**Validation error conventions:**
- `400 Bad Request`: Malformed requests, missing required params, type errors
- `422 Unprocessable Entity`: Business validation in form dialogs (name too long, duplicate name, required field empty). Return `c.JSON(422, gin.H{"error": "descriptive message"})`. The frontend `api.ts` extracts 422 error messages — dialogs catch and display them inline.

**Summary:**
- **Stopping errors** (abort the request): `c.AbortWithError(status, traceway.NewStackTraceErrorf("reason: %w", err))`
- **Non-stopping errors** (continue serving): `traceway.CaptureException(fmt.Errorf("reason: %w", err))`
- **Validation errors** (user-facing): `c.JSON(422, gin.H{"error": "message"})` for form validation
- **Always** wrap errors with `traceway.NewStackTraceErrorf` or `fmt.Errorf` using `%w` — never discard the original error

---

## Native `/api/report` Protocol

There is **no Traceway Go SDK**. It was retired, and every backend, Go included, instruments with
OpenTelemetry and exports over OTLP/HTTP. The docs carry no Go SDK pages, and `docs/public/_redirects`
301s the old `/client/sdk` and `/client/*-middleware` URLs to `/client/otel`. Do not reintroduce them.

The `/api/report` endpoint below still exists and is still served. The browser and mobile SDKs
(`@tracewayapp/*`, the iOS and Android libraries, Flutter) speak it, and the backend uses it to report
its own telemetry. Keep this section accurate for those clients.

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
       } catch (e: any) {
         if (e.status === 404) {
           notFound = true
         } else {
           error = e.message || 'Failed to load data'
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

### Adding a New Metric to Dashboard

1. **Ensure SDK captures metric** (or add to `traceway.go` metrics collection)

2. **Add repository query** to each telemetry backend's `metric_point.repository.go` under `backend/app/repositories/telemetry/{clickhouse,sqlite,duckdb}/`
   ```go
   func (r *metricPointRepository) GetNewMetric(ctx context.Context, projectId uuid.UUID, from, to time.Time) ([]models.TimeSeriesPoint, error) {
       // Query metric_points with the backend's dialect
   }
   ```

3. **Add to dashboard controller** in `backend/app/controllers/dashboard.go`

4. **Frontend auto-renders** from API response (metrics dashboard uses dynamic rendering)

### Adding a Database Column

1. **Create migration file** (remember: ONE statement per file!)
   ```
   backend/app/migrations/ch/0015_add_new_column.up.sql
   ```
   ```sql
   ALTER TABLE transactions ADD COLUMN new_column String DEFAULT ''
   ```

2. **Update model** in `backend/app/models/`

3. **Update repository queries** to include new column

4. **Run migrations**: Backend runs migrations automatically on startup

### Adding Table Sorting to a Page

1. **Import and add state**:
   ```typescript
   import { getSortState, setSortState, handleSortClick } from '$lib/utils/sort-storage'
   import type { SortState } from '$lib/utils/sort-storage'

   let sortState = $state<SortState>(getSortState('page-key', { field: 'default_column', direction: 'desc' }))
   ```

2. **Add sort handler**:
   ```typescript
   function onSortClick(field: string) {
       sortState = handleSortClick(field, sortState.field, sortState.direction, 'desc')
       setSortState('page-key', sortState)
   }
   ```

3. **Use TracewayTableHeader**:
   ```svelte
   <TracewayTableHeader
       label="Column"
       column="column_name"
       orderBy={`${sortState.field} ${sortState.direction}`}
       onclick={() => onSortClick('column_name')}
   />
   ```

4. **Pass to API call** - convert to backend format: `"column asc"` or `"column desc"`:
   ```typescript
   const orderBy = `${sortState.field} ${sortState.direction}`
   ```

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

Values removed from the picker (`gin`, `fiber`, `chi`, `fasthttp`, `stdlib`, `custom`, `nextjs`, `nestjs`, `express`, `remix`, `hono`, `cloudflare`, `symfony`, `laravel`, `django`) stay valid in `validFrameworks` and in `FRAMEWORK_LABELS` — existing projects still carry them and must keep rendering.
