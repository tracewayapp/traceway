# Traceway SDK Integration Testing API Specification

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Port the test app listens on |
| `TRACEWAY_ENDPOINT` | `default_token_change_me@http://localhost:8082/api/report` | Traceway connection string (`<token>@<server_url>`) |

## Running Test Apps

### Go Frameworks

```bash
# Gin
cd testing/go/gin && go run .

# Chi
cd testing/go/chi && go run .

# Stdlib (net/http)
cd testing/go/stdlib && go run .

# Fiber
cd testing/go/fiber && go run .

# Fasthttp
cd testing/go/fasthttp && go run .
```

### JS Frameworks

```bash
# Express
cd testing/js/express && npm install && npm run dev

# NestJS
cd testing/js/nestjs && npm install && npm run dev

# Next.js
cd testing/js/nextjs && npm install && npm run dev

# Remix
cd testing/js/remix && npm install && npm run dev
```

## Endpoints

All endpoints use `http://localhost:8080` as the base URL (configurable via `PORT`).

For Next.js and Remix, all endpoints are prefixed with `/api/` (e.g., `/api/test-ok` instead of `/test-ok`).

---

### 1. GET /test-ok

**Purpose:** Basic transaction tracking - verifies the SDK captures a successful HTTP request.

**Expected Response:**
```json
{"status": "ok"}
```
Status: `200`

**curl:**
```bash
curl http://localhost:8080/test-ok
```

**Verification:**
- `transactions` table: row with matching `endpoint` = `GET /test-ok`, `status_code` = `200`
- Duration should be non-zero

---

### 2. GET /test-not-found

**Purpose:** Error status code tracking - verifies the SDK captures non-200 status codes.

**Expected Response:**
```json
{"status": "not-found"}
```
Status: `404`

**curl:**
```bash
curl http://localhost:8080/test-not-found
```

**Verification:**
- `transactions` table: row with `endpoint` = `GET /test-not-found`, `status_code` = `404`

---

### 3. GET /test-exception

**Purpose:** Panic recovery and uncaught exception capture.

**Expected Response:** Status `500` (body varies by framework)

**curl:**
```bash
curl http://localhost:8080/test-exception
```

**Verification:**
- `exception_stack_traces` table: row with `type` containing panic/Error type, `value` containing `"test panic from /test-exception"`
- Stack trace should be present
- `transactions` table: row with `status_code` = `500`

---

### 4. GET /test-error-simple

**Purpose:** Explicit error capture with a plain error (no stack trace attached).

**Expected Response:**
```json
{"error": "simple error"}
```
Status: `500`

**curl:**
```bash
curl http://localhost:8080/test-error-simple
```

**Verification:**
- `exception_stack_traces` table: row with `value` containing `"simple error without stack"`
- `transactions` table: row with `status_code` = `500`

---

### 5. GET /test-error-stacktrace

**Purpose:** Error capture with `NewStackTraceErrorf` (Go) or `Error` with stack (JS).

**Expected Response:**
```json
{"error": "stacktrace error"}
```
Status: `500`

**curl:**
```bash
curl http://localhost:8080/test-error-stacktrace
```

**Verification:**
- `exception_stack_traces` table: row with `value` containing `"error with stack trace"`
- Stack trace should contain the handler function name
- `transactions` table: row with `status_code` = `500`

---

### 6. GET /test-error-wrapped

**Purpose:** Wrapped/chained error capture.

**Expected Response:**
```json
{"error": "wrapped error"}
```
Status: `500`

**curl:**
```bash
curl http://localhost:8080/test-error-wrapped
```

**Verification:**
- `exception_stack_traces` table: row with `value` containing `"layer 2"` and `"base error"` in the chain
- Error chain should preserve wrapping (Go: `fmt.Errorf %w`, JS: `Error.cause`)
- `transactions` table: row with `status_code` = `500`

---

### 7. GET /test-error-nested

**Purpose:** 3-level nested function call error with stack trace from `NewStackTraceErrorf` (Go) or `Error.captureStackTrace` (JS).

**Expected Response:**
```json
{"error": "nested error"}
```
Status: `500`

**curl:**
```bash
curl http://localhost:8080/test-error-nested
```

**Verification:**
- `exception_stack_traces` table: row with `value` containing `"error from inner function"`
- Stack trace should show `innerFunction`, `middleFunction`, `outerFunction` in the call chain
- `transactions` table: row with `status_code` = `500`

---

### 8. GET /test-message

**Purpose:** Message capture via `CaptureMessage` / `captureMessage`.

**Expected Response:**
```json
{"status": "message sent"}
```
Status: `200`

**curl:**
```bash
curl http://localhost:8080/test-message
```

**Verification:**
- `exception_stack_traces` table: row with `value` = `"test message from /test-message"` and `type` = `"message"` (or similar message type marker)
- `transactions` table: row with `status_code` = `200`

---

### 9. GET /test-message-attributes

**Purpose:** Message capture with attributes. Tests `CaptureMessageAttributes` (Go) or plain `captureMessage` (JS - no attributes support).

**Expected Response:**
```json
{"status": "message with attributes sent"}
```
Status: `200`

**curl:**
```bash
curl http://localhost:8080/test-message-attributes
```

**Verification:**
- `exception_stack_traces` table: row with `value` = `"test message with attributes"`
- **Go only:** `tags` map should contain `source` = `"test-message-attributes"` and `priority` = `"high"`
- **JS limitation:** `captureMessage` does not support attributes; message is captured without custom tags

---

### 10. GET /test-spans

**Purpose:** Nested span capture via `StartSpan`/`EndSpan` within a request.

**Expected Response:**
```json
{"status": "spans captured"}
```
Status: `200`

**curl:**
```bash
curl http://localhost:8080/test-spans
```

**Verification:**
- `transactions` table: row with `status_code` = `200` and `endpoint` = `GET /test-spans`
- The transaction should have associated spans:
  - `db.query` span (~50ms)
  - `cache.set` span (~20ms)
  - `http.external_api` span (~100ms)
- Total request duration should be >= 170ms

**Note:** For Fiber and Fasthttp, spans use `context.Background()` since these frameworks don't use `context.Context` natively, so spans won't be attached to the trace context automatically.

---

### 11. GET /test-task

**Purpose:** Background task measurement via `MeasureTask` / `measureTask`.

**Expected Response:**
```json
{"status": "task started"}
```
Status: `200`

**curl:**
```bash
curl http://localhost:8080/test-task
```

**Verification:**
- `transactions` table: a task record with `endpoint` = `"background-data-processor"` should appear after the collection interval (default 5s)
- The task should have a `processing` span (~200ms)
- The task is async, so data may arrive in a subsequent collection frame

---

### 12. GET /test-metric

**Purpose:** Custom metric capture via `CaptureMetric` / `captureMetric`.

**Expected Response:**
```json
{"status": "metric captured"}
```
Status: `200`

**curl:**
```bash
curl http://localhost:8080/test-metric
```

**Verification:**
- `metric_records` table: row with `name` = `"test.custom_metric"` and `value` = `42.0`
- `transactions` table: row with `status_code` = `200`

---

### 13. GET /test-attributes

**Purpose:** Exception capture with custom attributes map via `CaptureExceptionWithAttributes` / `captureExceptionWithAttributes`.

**Expected Response:**
```json
{"status": "exception with attributes captured"}
```
Status: `200`

**curl:**
```bash
curl http://localhost:8080/test-attributes
```

**Verification:**
- `exception_stack_traces` table: row with `value` containing `"exception with custom attributes"`
- `tags` map should contain:
  - `user_id` = `"usr_123"`
  - `request_id` = `"req_456"`
  - `env` = `"testing"`
- `transactions` table: row with `status_code` = `200`

---

### 14. POST /test-recording

**Purpose:** Request body/header recording on error. Tests `WithOnErrorRecording` capturing request details when a panic occurs.

**Request Body:**
```json
{"action": "panic", "data": "test payload"}
```

**Expected Response:** Status `500` (panic response)

**curl (trigger panic with recording):**
```bash
curl -X POST http://localhost:8080/test-recording \
  -H "Content-Type: application/json" \
  -H "X-Custom-Header: test-value" \
  -d '{"action": "panic", "data": "test payload"}'
```

**curl (normal request):**
```bash
curl -X POST http://localhost:8080/test-recording \
  -H "Content-Type: application/json" \
  -d '{"action": "ok", "data": "test payload"}'
```

Normal response:
```json
{"status": "ok", "received": {"action": "ok", "data": "test payload"}}
```

**Verification (panic case):**
- `exception_stack_traces` table: row with `value` containing `"panic triggered by /test-recording"`
- `tags` map should contain recorded request details (depending on framework recording support):
  - URL/path information
  - Request body content
  - Request headers
- `transactions` table: row with `status_code` = `500`

**Note:** Request recording is a Go SDK middleware feature (`WithOnErrorRecording`). JS frameworks capture request details manually in error handlers.

---

## Framework Matrix

| Feature | Gin | Chi | Stdlib | Fiber | Fasthttp | Express | NestJS | Next.js | Remix |
|---------|-----|-----|--------|-------|----------|---------|--------|---------|-------|
| Auto transaction tracking | Middleware | Middleware | Middleware | Middleware | Middleware | Manual | Manual | Manual | Manual |
| Panic/exception recovery | Middleware | Middleware | Middleware | Middleware | Middleware | Error handler | Exception filter | Wrapper | Wrapper |
| Context-based spans | Yes | Yes | Yes | No* | No* | Manual | Manual | Manual | Manual |
| `CaptureMessageAttributes` | Yes | Yes | Yes | Yes | Yes | No** | No** | No** | No** |
| Request recording on error | Middleware | Middleware | Middleware | Middleware | Middleware | Manual | Manual | Manual | Manual |
| Error via framework API | `ctx.Error()` | Manual | Manual | Return err | Manual | Throw | Throw | Throw | Throw |

\* Fiber and Fasthttp don't have native `context.Context` in handlers; spans use `context.Background()`.

\** JS SDK `captureMessage` does not have an attributes parameter; messages are captured without custom tags.

---

## Verification Guide

### 1. Start Backend
```bash
cd backend && go run .
```

### 2. Start Test App
```bash
cd testing/go/gin && go run .
```

### 3. Run All Endpoints
```bash
BASE=http://localhost:8080

curl $BASE/test-ok
curl $BASE/test-not-found
curl $BASE/test-exception
curl $BASE/test-error-simple
curl $BASE/test-error-stacktrace
curl $BASE/test-error-wrapped
curl $BASE/test-error-nested
curl $BASE/test-message
curl $BASE/test-message-attributes
curl $BASE/test-spans
curl $BASE/test-task
curl $BASE/test-metric
curl $BASE/test-attributes
curl -X POST $BASE/test-recording -H "Content-Type: application/json" -d '{"action":"panic","data":"test"}'
```

### 4. Wait for Collection
Wait at least 10 seconds for the SDK to rotate collection frames and upload data.

### 5. Query Backend API

Check transactions:
```bash
curl -H "Authorization: Bearer <jwt>" \
  "http://localhost:8082/api/transactions?projectId=<project-id>"
```

Check exceptions:
```bash
curl -H "Authorization: Bearer <jwt>" \
  "http://localhost:8082/api/exception-stack-traces/grouped?projectId=<project-id>"
```

Check metrics (dashboard):
```bash
curl -H "Authorization: Bearer <jwt>" \
  "http://localhost:8082/api/dashboard?projectId=<project-id>"
```

---

## ClickHouse Tables Reference

| Table | Data From Endpoints |
|-------|-------------------|
| `transactions` | All endpoints (1-14) generate transaction records |
| `exception_stack_traces` | Endpoints 3-9, 13-14 generate exception/message records |
| `metric_records` | Endpoint 12 generates custom metric + automatic system metrics |
