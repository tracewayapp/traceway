# CLAUDE.md - Traceway Project

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Traceway is an error tracking and monitoring platform consisting of:
- **Frontend**: SvelteKit 2 dashboard application with Svelte 5
- **Backend**: Go/Gin API server with ClickHouse database
- **CLI**: Go/Cobra command-line client for the backend HTTP API (`/cli`)
- **Go Client SDK**: Distributed tracing SDK for Go applications (external repo)

---

## Code Style

- **No pointless comments**: Do not add comments that simply describe what the code does. The code should be self-explanatory. Only add comments when explaining non-obvious "why" decisions.
- **No `py-4` in dialog form content**: Do not add `py-4` on the content wrapper inside `AlertDialog` or `Dialog` components — it creates too much blank space between the form and the action buttons.
- **Dialog button labels & toasts**: For form dialogs, use descriptive button labels with icons instead of generic "Create"/"Update". Create actions: `<Plus icon> {Action} {Entity}` (e.g., "+ New Widget Group"). Update actions: `<Check icon> Update {Entity}`. After successful create/update, show a `toast.success('Successfully {action} the {Entity}', { position: 'top-center' })`. The button should only be `disabled` during the loading state — never disable it to enforce validation; let the backend return 422 and show the error in the dialog instead.

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
| Go Client | External repo at `/Users/dusanstanojevic/Documents/workspace/go-client` | Build with `go build ./...` |

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
    return repositories.ProjectRepository.FindById(tx, id)
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
    tx := middleware.GetTx(ctx)  // Get transaction from context

    // Use tx for all repository calls
    user, err := repositories.UserRepository.FindByEmail(tx, email)
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

**Preference:** For CRUD controller methods, always prefer using `middleware.Transactional` in the route + `middleware.GetTx(ctx)` in the controller over `pgdb.ExecuteTransaction`. The middleware approach keeps controllers flat, avoids nested closures, and follows the established pattern.

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
user, err := repositories.UserRepository.FindByEmail(tx, email)
if err != nil {
    return nil, err  // actual database error
}
if user == nil {
    // record not found - handle accordingly
    return nil, errors.New("user not found")
}

// WRONG - do not use sql.ErrNoRows with lit
user, err := repositories.UserRepository.FindByEmail(tx, email)
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

# Notifications
NOTIFICATION_POLL_SECONDS=60          # polled rule evaluation interval; minimum 5, invalid values fall back to 60

# Retention (see "Data Retention" section below)
SQLITE_RETENTION_DAYS=30              # 0 to disable; only applies in SQLite mode
SESSION_RECORDING_RETENTION_DAYS=30   # 0 to disable; only applies when STORAGE_TYPE=local

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
/metrics                    System metrics dashboard (CPU, memory, etc.)
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

### API Endpoints

**Client SDK Ingestion**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/report` | Client | Telemetry ingestion (gzipped) |
| POST | `/api/otel/v1/traces` | Client | OTLP/HTTP trace ingestion |
| POST | `/api/otel/v1/metrics` | Client | OTLP/HTTP metric ingestion |
| POST | `/api/otel/v1/logs` | Client | OTLP/HTTP log ingestion |

**Auth & Registration**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/login` | None | Dashboard authentication |
| POST | `/api/register` | None | New user registration |
| GET | `/api/has-organizations` | None | Check if any orgs exist (self-hosted only) |
| POST | `/api/forgot-password` | None | Request password reset email |
| GET | `/api/password-reset/:token` | None | Validate reset token |
| POST | `/api/password-reset/:token` | None | Reset password with token |

**Projects**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/projects` | App | List projects |
| POST | `/api/projects` | App+Write | Create project |
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

**Widget Groups & Widgets**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/widget-groups` | App | List widget groups |
| POST | `/api/widget-groups` | App+Write | Create widget group |
| GET | `/api/widget-groups/:id` | App | Get group with widgets |
| PUT | `/api/widget-groups/:id` | App+Write | Update widget group |
| DELETE | `/api/widget-groups/:id` | App+Write | Delete widget group |
| POST | `/api/widget-groups/:id/widgets` | App+Write | Add widget |
| PUT | `/api/widget-groups/:id/widgets/:wid` | App+Write | Update widget |
| PUT | `/api/widget-groups/:id/widgets/:wid/move` | App+Write | Reorder widget |
| DELETE | `/api/widget-groups/:id/widgets/:wid` | App+Write | Delete widget |

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

**Organization Management**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/api/organizations/:orgId/settings` | Admin | Get org settings |
| PUT | `/api/organizations/:orgId/settings` | Admin | Update org settings |
| GET | `/api/organizations/:orgId/members` | Admin | List members |
| PUT | `/api/organizations/:orgId/members/:userId` | Admin | Update member role |
| DELETE | `/api/organizations/:orgId/members/:userId` | Admin | Remove member |

**Invitations**
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/organizations/:orgId/invitations` | Admin | Send invitation |
| GET | `/api/organizations/:orgId/invitations` | Admin | List invitations |
| DELETE | `/api/organizations/:orgId/invitations/:id` | Admin | Revoke invitation |
| GET | `/api/invitations/:token` | None | Get invitation info |
| POST | `/api/invitations/:token/accept` | None | Accept (new user) |
| POST | `/api/invitations/:token/accept-existing` | App | Accept (existing user) |

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
| `invitations` | Team invitations with token, role, expiry |
| `source_maps` | Uploaded source map files (project, version, storage key) |
| `metric_registry` | Custom metric definitions (type, unit, description) |
| `widget_groups` | Dashboard widget groups (name, default flag) |
| `widget_group_widgets` | Individual widgets within groups (type, config, position) |

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
- `source_maps`, `metric_registry`, `widget_groups`, `widget_group_widgets`
- `notification_channels`, `notification_rules`

**Telemetry DB tables** (`db.TelemetryDB` — non-transactional, uses lit with `db.TelemetryDB` directly):
- `endpoints`, `tasks`, `exception_stack_traces`, `spans`, `metric_points`
- `session_recordings`, `archived_exceptions`, `slow_endpoints`, `fired_notifications`

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

**SQLite-specific type helpers** (`backend/app/repositories/sqlite_types.go`):
- `SQLiteTime` — implements `sql.Scanner`/`driver.Valuer` for `time.Time` ↔ SQLite TEXT
- `SQLiteJSONMap` — implements `sql.Scanner`/`driver.Valuer` for `map[string]string` ↔ SQLite JSON TEXT
- Row types (e.g., `endpointRow`, `taskRow`) wrap domain models with these types for lit compatibility

#### Data Retention

Retention is handled in three different ways depending on the deployment.

**1. ClickHouse — `TTL` clauses on the table itself.** Only a few tables have a TTL; everything else is kept indefinitely (operators can drop monthly partitions manually if needed).

| Table | Retention | Source migration |
|-------|-----------|------------------|
| `metric_points` (raw) | **7 days** | `0034_add_ttl_metric_points.up.sql` |
| `metric_points_1m` (1-min rollup) | **30 days** | `0035_add_ttl_metric_points_1m.up.sql` |
| `metric_points_1h` (1-hour rollup) | **1 year** | `0036_add_ttl_metric_points_1h.up.sql` |
| `log_records` | **30 days** | `0045_create_log_records.up.sql` |
| All other CH tables (`transactions`, `exception_stack_traces`, `tasks`, `spans`, `sessions`, `ai_traces`, `session_recordings`, `fired_notifications`, `archived_exceptions`, `slow_endpoints`, `endpoints`, etc.) | **No TTL — retained indefinitely** | — |

**2. SQLite — `retention.Start` worker** (`backend/app/retention/sqlite.go`). In SQLite mode, neither `db.DB` nor `db.TelemetryDB` has any built-in expiry, so a background worker fires once at startup and then every hour and runs a `DELETE FROM <table> WHERE <time_column> < cutoff` against each telemetry table.

| Variable | Default | Notes |
|----------|---------|-------|
| `SQLITE_RETENTION_DAYS` | `30` | TTL in days. Set to `0` to disable the worker entirely. Has no effect outside SQLite mode. |

Tables it prunes (and the column used):

| Database | Table | Time column |
|----------|-------|-------------|
| Telemetry (`db.TelemetryDB`) | `endpoints`, `tasks`, `exception_stack_traces`, `spans`, `metric_points`, `session_recordings`, `ai_traces` | `recorded_at` |
| Telemetry | `log_records` | `timestamp` |
| Telemetry | `sessions` | `started_at` |
| Telemetry | `fired_notifications` | `fired_at` |

`archived_exceptions` (per-hash flags) and `slow_endpoints` (per-endpoint config) are intentionally skipped — they are not time-series data.

**3. On-disk session recordings — `retention.Start` worker** (`backend/app/retention/recordings.go`). Session recordings written to local disk (`STORAGE_TYPE=local`) accumulate under `<STORAGE_PATH>/recordings/`. A second worker walks that directory once at startup and then every hour and removes files whose `mtime` is older than the TTL, then prunes any directories left empty. The worker is a no-op when `STORAGE_TYPE=s3`.

| Variable | Default | Notes |
|----------|---------|-------|
| `SESSION_RECORDING_RETENTION_DAYS` | `30` | TTL in days. Set to `0` to disable the worker. Only runs when `STORAGE_TYPE=local` (default). |

The DB rows in `session_recordings` are pruned by the SQLite retention worker (above) or by ClickHouse TTL — they are intentionally not coupled to the disk cleanup. Controllers that read recordings already log a non-fatal `traceway.CaptureException` when a referenced file is missing.

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

#### Organization Roles
| Role | Description |
|------|-------------|
| `owner` | Full access, can manage organization |
| `admin` | Full access to projects |
| `user` | Standard access to projects |
| `readonly` | Read-only access, cannot create projects or archive exceptions |

The `RequireWriteAccess` middleware blocks write operations for users with `readonly` role.

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
1. Each migration file must contain **exactly ONE SQL statement**
2. Only create `.up.sql` files (no down migrations)
3. Use sequential numbering: `NNNN_description.up.sql`

**Why one statement per file?** ClickHouse migration runner executes each file as a single statement. Multiple statements will fail.

**Example - Adding two columns requires TWO files:**
```
backend/app/migrations/ch/
├── 0013_add_app_version_to_transactions.up.sql
│   └── ALTER TABLE transactions ADD COLUMN app_version String DEFAULT ''
├── 0014_add_server_name_to_transactions.up.sql
│   └── ALTER TABLE transactions ADD COLUMN server_name String DEFAULT ''
```

### Exception Hash Normalization

The backend normalizes stack traces before hashing to group identical errors despite different runtime values. This happens in `backend/app/repositories/exceptions.go`.

**Normalization Steps:**
1. Extract error type only (remove error message content)
2. Collapse JS SDK function-name lines (ending in `()`, directly above a 4-space-indented `file:line:col` location line) to `<fn>` so resolved function names never affect grouping; anchoring on the location line keeps Go traces (tab-indented file lines) untouched
3. Remove absolute file paths (keep `filename:line` only)
4. Replace hexadecimal addresses with `<hex>`
5. Replace UUIDs with `<uuid>`
6. Replace IP addresses with `<ip>`
7. Replace timestamps with `<timestamp>`
8. Replace numeric IDs in paths with `<id>`
9. Normalize whitespace
10. Remove ANSI color codes
11. Hash the normalized string with SHA-256, truncate to 16 chars

**Note:** columns are intentionally kept in the hash. Minified bundles put everything on line 1, so the column is the only frame disambiguator when no source map matched. Function names are excluded (step 2) because they are derived from the location and change as symbolication improves.

**Result:** Same logical error gets same hash, even if:
- Error message contains different user IDs
- Stack trace has different memory addresses
- File paths differ between environments

### Repository Patterns

#### Singleton Pattern
Repositories are exported as package-level singletons for simple access:
```go
// backend/app/repositories/users.go
var UserRepository = userRepository{}

// backend/app/repositories/projects.go
var ProjectRepository = projectRepository{}

// Usage in controllers
user, err := repositories.UserRepository.FindByEmail(tx, email)
project, err := repositories.ProjectRepository.FindById(tx, id)
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

## Go Client SDK

**Location:** External repository at `/Users/dusanstanojevic/Documents/workspace/go-client`

The SDK is a separate Go module that applications import to send telemetry to Traceway.

### Installation
```go
import (
    "github.com/user/traceway"           // Core SDK
    "github.com/user/traceway/traceway_gin"  // Gin middleware
)
```

### Connection String Format
```
<project_token>@<server_url>

Examples:
abc123@http://localhost:8082/api/report
abc123@https://traceway.example.com/api/report
```

### Architecture

#### Ring Buffer Batching
The SDK uses a ring buffer to batch telemetry data:
1. Data collected into current "frame" (transactions, exceptions, metrics)
2. Frame rotates every 5 seconds (configurable via `WithCollectionInterval`)
3. Completed frames uploaded async via HTTP POST
4. Failed uploads retry with exponential backoff

#### Data Types Collected
- **Transactions**: HTTP requests (endpoint, duration, status code)
- **Exceptions**: Errors/panics with full stack traces
- **Metrics**: System metrics (CPU, memory, Go runtime)

### Gin Middleware Integration

```go
package main

import (
    "github.com/gin-gonic/gin"
    "traceway"
    "traceway/traceway_gin"
)

func main() {
    router := gin.Default()

    // Initialize with app name and connection string
    traceway_gin.Use(router, "myapp", "token@http://localhost:8082/api/report")

    // Or with options
    traceway_gin.Use(router, "myapp", "token@http://localhost:8082/api/report",
        traceway.WithDebug(true),
        traceway.WithCollectionInterval(10 * time.Second),
    )

    router.GET("/api/users", getUsers)
    router.Run(":8080")
}
```

The middleware automatically:
- Tracks request duration and status code
- Captures endpoint as `"METHOD /path"` (e.g., `"GET /api/users"`)
- Recovers from panics and reports them as exceptions
- Attaches request scope for contextual tags

### Capture Methods

#### Exceptions
```go
// Basic capture
traceway.CaptureException(err)

// With additional scope
traceway.CaptureExceptionWithScope(err, scope)

// Panic recovery (use in defer)
defer traceway.Recover()

// Recover with custom scope
defer traceway.RecoverWithScope(scope)
```

#### Metrics
```go
// Capture custom metric
traceway.CaptureMetric("custom.metric.name", 42.0)

// Metrics are batched and sent with the next frame
```

#### Transactions (Manual)
```go
// For non-HTTP transactions or custom tracking
txn := traceway.StartTransaction("operation_name")
defer txn.End()

// Add segments for sub-operations
seg := txn.StartSegment("database_query")
// ... do work ...
seg.End()
```

#### Tasks and Segments
```go
// Capture a task with duration
traceway.CaptureTask("background_job", startTime, endTime, nil)

// Measure a function
result := traceway.MeasureTask("compute", func() interface{} {
    return heavyComputation()
})

// Segments within tasks
traceway.CaptureSegment(taskID, "subtask", startTime, endTime)
```

### Scope System

Scopes attach contextual tags to exceptions and transactions.

#### Global Scope
```go
// Configure tags that apply to all telemetry
traceway.ConfigureScope(func(s *traceway.Scope) {
    s.SetTag("environment", "production")
    s.SetTag("version", "1.2.3")
    s.SetTag("region", "us-east-1")
})
```

#### Request Scope (Gin)
```go
func handler(c *gin.Context) {
    // Get request-scoped scope from Gin context
    scope := traceway_gin.GetScopeFromGin(c)

    // Tags only apply to this request
    scope.SetTag("user_id", userID)
    scope.SetTag("tenant", tenantID)

    // Any exceptions captured in this request will include these tags
}
```

#### Scope Methods
```go
scope.SetTag("key", "value")      // Set a single tag
scope.SetTags(map[string]string{  // Set multiple tags
    "key1": "value1",
    "key2": "value2",
})
scope.SetUser(userID)             // Shorthand for user_id tag
scope.SetExtra("key", anyValue)   // Set extra data (serialized to JSON)
```

### Configuration Options

```go
traceway.Init(appName, connectionString,
    // Debug mode - logs all telemetry to stdout
    traceway.WithDebug(true),

    // Max frames to keep in ring buffer (default: 20)
    traceway.WithMaxCollectionFrames(20),

    // How often to rotate frames (default: 5s)
    traceway.WithCollectionInterval(5 * time.Second),

    // HTTP upload timeout (default: 3s)
    traceway.WithUploadTimeout(3 * time.Second),

    // Metric collection interval (default: 30s)
    traceway.WithMetricInterval(30 * time.Second),

    // Disable automatic metric collection
    traceway.WithMetricsDisabled(),

    // Custom HTTP transport
    traceway.WithTransport(customTransport),
)
```

### Default Collected Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `cpu.used_pcnt` | CPU usage percentage | % |
| `mem.used` | Memory usage | MB |
| `mem.used_pcnt` | Memory usage percentage | % |
| `go.go_routines` | Active goroutine count | count |
| `go.heap_objects` | Heap object count | count |
| `go.num_gc` | Total GC cycles | count |
| `go.gc_pause` | Last GC pause time | nanoseconds |

### Data Format (Frame)

The SDK sends data as gzipped JSON:
```json
{
  "app": "myapp",
  "transactions": [
    {
      "trace_id": "abc123",
      "endpoint": "GET /api/users",
      "duration_ms": 45.2,
      "status_code": 200,
      "timestamp": "2024-01-15T10:30:00Z"
    }
  ],
  "exceptions": [
    {
      "type": "RuntimeError",
      "value": "connection refused",
      "stacktrace": "...",
      "tags": {"user_id": "123"},
      "timestamp": "2024-01-15T10:30:00Z"
    }
  ],
  "metrics": [
    {
      "name": "cpu.used_pcnt",
      "value": 45.2,
      "timestamp": "2024-01-15T10:30:00Z"
    }
  ]
}
```

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

2. **Add repository query** in `backend/app/repositories/metrics.go`
   ```go
   func (r *MetricRepository) GetNewMetric(projectID uuid.UUID, from, to time.Time) ([]MetricPoint, error) {
       // Query metric_records table
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

Every framework addition touches multiple files across backend, frontend, and docs. Use this checklist to avoid missing any:

1. **Backend** — `backend/app/controllers/project.controller.go`: Add to `validFrameworks` map and update the validation error message
2. **Frontend state** — `frontend/src/lib/state/projects.svelte.ts`: Add to `Framework` type union and `FRAMEWORK_LABELS` map
3. **Frontend combobox** — `frontend/src/lib/components/framework-combobox.svelte`: Add entry with correct group (Go / JavaScript / PHP / etc.)
4. **Frontend icon** — `frontend/src/lib/components/framework-icon.svelte`: Add icon mapping for the new framework value
5. **Framework code** — `frontend/src/lib/utils/framework-code.ts`: Add install command, integration code snippet, label, code language, and testing routes
6. **Connection page** — `frontend/src/routes/connection/+page.svelte`: Add highlight language mapping and install description
7. **Dashboard page** — `frontend/src/routes/+page.svelte`: Add highlight language mapping
8. **Docs sidebar** — `docs/pages/client/_meta.json`: Add navigation entry
9. **Docs SDK selector** — `docs/components/SdkContext.jsx`: Add to `SDK_OPTIONS` array and `PATH_SDK_MAP` object
10. **Docs framework picker** — `docs/components/FrameworkPicker.jsx`: Add card to `FRAMEWORKS` array
11. **Docs icon** — `docs/public/`: Add framework icon (PNG, ~45x45)
12. **Docs page** — `docs/pages/client/<framework>/`: Create `_meta.json` and `index.mdx`
13. **Docs OTel language table** — `docs/pages/client/otel/index.mdx`: Add row if the framework uses OTLP
