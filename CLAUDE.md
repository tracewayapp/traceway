# CLAUDE.md - Traceway Project

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Traceway is an error tracking and monitoring platform consisting of:
- **Frontend**: SvelteKit 5 dashboard application
- **Backend**: Go/Gin API server with ClickHouse database
- **Go Client**: Distributed tracing SDK for Go applications

## Quick Reference

### Development Commands
| Component | Command | Description |
|-----------|---------|-------------|
| Frontend | `cd frontend && npm run dev` | Dev server (port 5173) |
| Frontend | `npm run build` | Production build |
| Frontend | `npm run check` | TypeScript checking |
| Backend | `cd backend && go run .` | API server (port 8082) |
| Client | `cd clients/go-client && go build ./...` | Build SDK |

### Tech Stack
- Frontend: SvelteKit 2.49, Svelte 5.45, Tailwind 4, shadcn-svelte
- Backend: Go 1.25, Gin 1.11, ClickHouse
- Client SDK: Go 1.25, Gin middleware

---

## Frontend (`/frontend`)

### Architecture
- **Framework**: SvelteKit 2 with Svelte 5 runes API
- **Styling**: Tailwind CSS v4 with shadcn-svelte components
- **Build**: Vite 7, static adapter with SPA fallback
- **SSR**: Disabled - pure client-side SPA

### Key Patterns

#### State Management (Svelte 5 Runes)
```typescript
// Use $state() for reactive state
let data = $state<Type>(initial)

// Use $derived() for computed values
let computed = $derived(expression)

// Use $effect() for side effects
$effect(() => { /* reactive code */ })
```

#### State Files
- `src/lib/state/auth.svelte.ts` - Token auth, localStorage sync
- `src/lib/state/projects.svelte.ts` - Multi-project management
- `src/lib/state/theme.svelte.ts` - Dark/light mode

#### API Client (`src/lib/api.ts`)
- Auto-includes Authorization header
- Auto-includes projectId in requests
- 401 responses trigger logout + redirect to /login

### Routes
```
/                    Dashboard (protected)
/login               Login page (public)
/issues              Issues list with filtering
/issues/[hash]       Exception details
/issues/[hash]/events Exception events timeline
/transactions        Endpoint analytics
/transactions/[endpoint] Endpoint details
/metrics             System metrics dashboard
/connection          SDK integration guide
```

### UI Components
Location: `src/lib/components/ui/*`
Uses shadcn-svelte registry with bits-ui primitives

---

## Backend (`/backend`)

### Architecture
- **Framework**: Gin Gonic HTTP framework
- **Database**: ClickHouse (columnar OLAP)
- **Port**: 8082
- **Pattern**: Repository pattern with singleton controllers

### Project Structure
```
backend/
├── main.go                 # Entry point
├── app/
│   ├── controllers/        # API handlers
│   │   ├── routes.go       # Route registration
│   │   └── clientcontrollers/ # Telemetry ingestion
│   ├── repositories/       # ClickHouse queries
│   ├── models/             # Data structures
│   ├── middleware/         # Auth, gzip
│   ├── cache/              # In-memory project cache
│   └── migrations/ch/      # Database migrations
```

### API Endpoints
| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| POST | /api/report | Client | Telemetry ingestion |
| POST | /api/login | None | App authentication |
| GET/POST | /api/projects | App | Project CRUD |
| GET | /api/dashboard | App | Dashboard metrics |
| GET | /api/dashboard/overview | App | Recent issues + endpoints |
| POST | /api/transactions | App | Transaction search |
| POST | /api/transactions/grouped | App | Endpoint aggregates |
| POST | /api/exception-stack-traces | App | Exception search |

### Authentication
Two-tier system:
1. **Client Auth**: Project bearer tokens (for SDK telemetry)
2. **App Auth**: APP_TOKEN env var (for dashboard)

### Environment Variables
```
APP_TOKEN=<dashboard auth token>
CLICKHOUSE_SERVER=localhost:9000
CLICKHOUSE_DATABASE=traceway
CLICKHOUSE_USERNAME=default
CLICKHOUSE_PASSWORD=
CLICKHOUSE_TLS=false
```

### Database Tables
- `projects` - Multi-tenant projects with tokens
- `transactions` - HTTP request metadata (partitioned by month)
- `metric_records` - Time-series system metrics
- `exception_stack_traces` - Exceptions with hash grouping

### Exception Hash Normalization
The backend normalizes stack traces before hashing:
- Removes error message content (keeps type only)
- Removes absolute paths (keeps filename:line)
- Replaces hex addresses with `<hex>`
- Replaces UUIDs with `<uuid>`
- Replaces IPs with `<ip>`
- Result: Same errors grouped despite different runtime values

---

## Go Client SDK (`/clients/go-client`)

### Purpose
Distributed tracing SDK for Go applications. Captures:
- HTTP transactions (endpoint, duration, status code)
- Exceptions/panics with stack traces
- System metrics (CPU, memory, goroutines)

### Installation
```go
import (
    "traceway"
    "traceway/traceway_gin"
)
```

### Gin Integration
```go
router := gin.Default()
traceway_gin.Use(router, "myapp", "token@http://host:8082/api/report")
```

### Manual Capture
```go
// Metrics
traceway.CaptureMetric("custom.metric", 42.0)

// Exceptions
traceway.CaptureException(err)
traceway.CaptureExceptionWithScope(err, scope)

// Panic recovery
defer traceway.Recover()
```

### Scope (Contextual Tags)
```go
// Global scope
traceway.ConfigureScope(func(s *traceway.Scope) {
    s.SetTag("environment", "production")
})

// Request scope (Gin)
scope := traceway_gin.GetScopeFromGin(c)
scope.SetTag("user_id", "123")
```

### Configuration Options
```go
traceway.Init(app, connString,
    traceway.WithDebug(true),
    traceway.WithMaxCollectionFrames(20),
    traceway.WithCollectionInterval(10 * time.Second),
    traceway.WithUploadTimeout(3 * time.Second),
)
```

### Default Metrics Collected
- `cpu.used_pcnt` - CPU percentage
- `mem.used` - Memory in MB
- `mem.used_pcnt` - Memory percentage
- `go.go_routines` - Goroutine count
- `go.heap_objects` - Heap object count
- `go.num_gc` - GC cycle count
- `go.gc_pause` - GC pause time (ns)

### Data Flow
1. SDK collects data into in-memory ring buffer
2. Batches into "frames" every 5 seconds (configurable)
3. GZIP compresses and POSTs to /api/report
4. Backend normalizes, hashes, stores to ClickHouse

---

## Common Patterns

### Adding a New API Endpoint
1. Add model in `backend/app/models/`
2. Add repository method in `backend/app/repositories/`
3. Add controller in `backend/app/controllers/`
4. Register route in `backend/app/controllers/routes.go`
5. Add frontend API call in `frontend/src/lib/api.ts` or page

### Adding a New Frontend Page
1. Create folder in `frontend/src/routes/[name]/`
2. Add `+page.svelte` (component) and `+page.ts` (data loading)
3. Add navigation link in `app-sidebar.svelte`

### Adding a New Dashboard Metric
1. Ensure metric is captured by SDK (or add to `traceway.go`)
2. Add to `MetricRecordRepository` queries
3. Add to `DashboardController.GetDashboard()`
4. Frontend auto-renders from API response
