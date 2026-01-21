# CLAUDE.md - Traceway Project

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Traceway is an error tracking and monitoring platform consisting of:
- **Frontend**: SvelteKit 2 dashboard application with Svelte 5
- **Backend**: Go/Gin API server with ClickHouse database
- **Go Client SDK**: Distributed tracing SDK for Go applications (external repo)

---

## Quick Start

### Development Commands
| Component | Command | Description |
|-----------|---------|-------------|
| Frontend | `cd frontend && npm run dev` | Dev server (port 5173) |
| Frontend | `npm run build` | Production build |
| Frontend | `npm run check` | TypeScript checking |
| Backend | `cd backend && go run .` | API server (port 8082) |
| Go Client | External repo at `/Users/dusanstanojevic/Documents/workspace/go-client` | Build with `go build ./...` |

### Tech Stack
- **Frontend**: SvelteKit 2.49, Svelte 5.45, Tailwind CSS v4, shadcn-svelte, Vite 7
- **Backend**: Go 1.25, Gin 1.11, ClickHouse, PostgreSQL
- **Client SDK**: Go 1.25, Gin middleware support

### go-lightning Library (PostgreSQL ORM)
- **Import**: `github.com/tracewayapp/go-lightning/lit`
- **Purpose**: Lightweight generic CRUD operations for PostgreSQL

#### Model Registration (required before use)
```go
lit.RegisterModel[User](lit.PostgreSQL)
```

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
| `lit.Update[T](tx, &entity, "WHERE id = $1", id)` | Update (WHERE required) |
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

### Environment Variables (Backend)
```
APP_TOKEN=<dashboard auth token>
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
1. **Client Auth**: Project bearer tokens (SDK telemetry via `Authorization: Bearer <token>`)
2. **App Auth**: `APP_TOKEN` env var (dashboard authentication)

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
- Includes `projectId` in POST request bodies
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
Table sorting persists to localStorage using a consistent pattern:

```typescript
// Key format: traceway_sort_{pageKey}
// Example: traceway_sort_issues, traceway_sort_transactions

// In +page.svelte
let orderBy = $state(loadSortFromStorage('issues', 'last_seen desc'))

// After sort change
function handleSortClick(column: string) {
    orderBy = handleSortClickUtil(orderBy, column, 'issues')
}
```

The `handleSortClick` utility (`src/lib/utils/sorting.ts`):
1. Toggles sort direction if same column
2. Defaults to descending for new column
3. Saves to localStorage
4. Returns new `orderBy` string in backend format: `"column_name asc|desc"`

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

Time range and filters persist in URL query params:
```typescript
// src/lib/utils/url-state.ts
// Presets: 1h, 6h, 24h, 7d, 30d, custom

// Reading from URL
const range = getTimeRangeFromUrl(url.searchParams)

// Updating URL (preserves other params)
goto(updateUrlWithTimeRange(url, newRange))
```

### Routes
```
/                           Dashboard (protected) - overview metrics
/login                      Login page (public)
/issues                     Issues list with filtering/sorting
/issues/[hash]              Exception details view
/issues/[hash]/events       Exception events timeline
/transactions               Endpoint analytics with P50/P95/P99
/transactions/[endpoint]    Single endpoint details
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

### API Endpoints

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | `/api/report` | Client | Telemetry ingestion (gzipped) |
| POST | `/api/login` | None | Dashboard authentication |
| GET | `/api/projects` | App | List projects |
| POST | `/api/projects` | App | Create project |
| GET | `/api/dashboard` | App | Dashboard metrics |
| GET | `/api/dashboard/overview` | App | Recent issues + top endpoints |
| POST | `/api/transactions` | App | Transaction search |
| POST | `/api/transactions/grouped` | App | Endpoint aggregates (P50/P95/P99) |
| POST | `/api/exception-stack-traces` | App | Exception search |
| POST | `/api/exception-stack-traces/grouped` | App | Grouped exception list |
| GET | `/api/exception-stack-traces/:hash` | App | Single exception details |

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

#### Tables (PostgreSQL)
| Table | Purpose |
|-------|---------|
| `projects` | Multi-tenant project config + tokens |

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
2. Remove absolute file paths (keep `filename:line` only)
3. Replace hexadecimal addresses with `<hex>`
4. Replace UUIDs with `<uuid>`
5. Replace IP addresses with `<ip>`
6. Replace timestamps with `<timestamp>`
7. Replace numeric IDs in paths with `<id>`
8. Normalize whitespace
9. Remove ANSI color codes
10. Hash the normalized string with SHA-256, truncate to 16 chars

**Result:** Same logical error gets same hash, even if:
- Error message contains different user IDs
- Stack trace has different memory addresses
- File paths differ between environments

### Repository Patterns

#### Singleton Pattern
```go
// backend/app/repositories/transactions.go
var transactionRepoInstance *TransactionRepository
var transactionRepoOnce sync.Once

func GetTransactionRepository() *TransactionRepository {
    transactionRepoOnce.Do(func() {
        transactionRepoInstance = &TransactionRepository{db: GetDB()}
    })
    return transactionRepoInstance
}
```

#### Batch Insert
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

### Adding a New Frontend Page

1. **Create route folder**: `frontend/src/routes/new-page/`

2. **Add page component**: `+page.svelte`
   ```svelte
   <script lang="ts">
     let data = $state<DataType[]>([])

     $effect(() => {
       loadData()
     })
   </script>
   ```

3. **Add data loading** (optional): `+page.ts`
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

1. **Add state and load from storage**:
   ```typescript
   let orderBy = $state(loadSortFromStorage('page-key', 'default_column desc'))
   ```

2. **Add sort handler**:
   ```typescript
   function handleSortClick(column: string) {
       orderBy = handleSortClickUtil(orderBy, column, 'page-key')
   }
   ```

3. **Use TracewayTableHeader**:
   ```svelte
   <TracewayTableHeader
       label="Column"
       column="column_name"
       {orderBy}
       onclick={() => handleSortClick('column_name')}
   />
   ```

4. **Pass orderBy to API call** - backend expects format `"column asc"` or `"column desc"`
