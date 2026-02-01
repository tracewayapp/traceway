# Traceway Protocol Specification

This document is the complete protocol specification for the Traceway `/api/report` endpoint. Use it as a reference when building a new Traceway client SDK in any language.

## Endpoint

```
POST /api/report
```

A single endpoint accepts all telemetry data: **traces** (endpoints and tasks), **exceptions** (errors and messages), and **metrics**.

## Authentication

Every request must include a bearer token in the `Authorization` header. The token is a project token (UUID v4) obtained from the Traceway dashboard.

```
Authorization: Bearer {token}
```

The backend validates the token against a project cache. Invalid or missing tokens result in a `401 Unauthorized` response.

## Transport

### Connection String

SDKs accept a connection string in the format:

```
{token}@{apiUrl}
```

**Example:**

```
550e8400-e29b-41d4-a716-446655440000@https://traceway.example.com/api/report
```

Parse by splitting on the first `@`:
- Left side → bearer token
- Right side → full URL to POST to

### Required Headers

| Header | Value |
|--------|-------|
| `Content-Type` | `application/json` |
| `Content-Encoding` | `gzip` |
| `Authorization` | `Bearer {token}` |

### Gzip Requirement

The request body **must** be gzip-compressed JSON. The backend checks for the `Content-Encoding: gzip` header and returns `400 Bad Request` if it is missing. The body is decompressed server-side before JSON parsing.

## Request Payload

### Top-Level Structure

```json
{
  "collectionFrames": [ ... ],
  "appVersion": "1.2.3",
  "serverName": "web-01"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `collectionFrames` | `CollectionFrame[]` | Yes | Array of collection frames containing telemetry data |
| `appVersion` | `string` | Yes | Application version string (can be empty `""`) |
| `serverName` | `string` | Yes | Hostname or server identifier (can be empty `""`) |

### CollectionFrame

Each frame is a batch of data collected during one collection interval.

```json
{
  "stackTraces": [ ... ],
  "metrics": [ ... ],
  "traces": [ ... ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `stackTraces` | `ExceptionStackTrace[]` | Yes | Array of exception/message records |
| `metrics` | `MetricRecord[]` | Yes | Array of metric data points |
| `traces` | `Trace[]` | Yes | Array of endpoint and task traces |

All three arrays should be present. Use empty arrays `[]` or `null` when there is no data of that type.

## Traces

A trace represents either an **endpoint** (HTTP request) or a **task** (background job). The `isTask` field distinguishes them.

### Trace Object

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "endpoint": "GET /api/users/:id",
  "duration": 15234000,
  "recordedAt": "2025-01-15T10:30:00.123Z",
  "statusCode": 200,
  "bodySize": 1024,
  "clientIP": "192.168.1.1",
  "attributes": { "user_id": "42" },
  "spans": [ ... ],
  "isTask": false
}
```

| Field | JSON Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Id | `id` | `string` | Yes | UUID v4 identifying this trace |
| Endpoint | `endpoint` | `string` | Yes | Endpoint name or task name |
| Duration | `duration` | `integer` | Yes | Duration in **nanoseconds** (int64) |
| RecordedAt | `recordedAt` | `string` | Yes | Timestamp in **RFC 3339** format |
| StatusCode | `statusCode` | `integer` | Yes | HTTP status code (0 for tasks) |
| BodySize | `bodySize` | `integer` | Yes | Response body size in bytes (0 for tasks) |
| ClientIP | `clientIP` | `string` | Yes | Client IP address (empty `""` for tasks) |
| Attributes | `attributes` | `object` | No | Key-value string pairs. Omit or `null` when empty. |
| Spans | `spans` | `Span[]` | No | Sub-operation spans. Omit or `null` when empty. |
| IsTask | `isTask` | `boolean` | No | `true` for tasks, omit or `false` for endpoints. |

### Important Notes on Trace Fields

**Duration is nanoseconds.** Go's `time.Duration` serializes as an int64 representing nanoseconds. A duration of 15.234ms is sent as `15234000`. Other languages must convert their duration representation to nanoseconds before serializing.

**Timestamps are RFC 3339.** Go's `time.Time` serializes as RFC 3339 with optional nanosecond precision (e.g., `"2025-01-15T10:30:00.123456789Z"`).

**UUIDs** are v4, formatted as `"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"`.

**`attributes` and `spans`** use `omitempty` in Go — they are omitted from JSON when null/empty. Your SDK may omit them or send `null`/`{}` and `[]`.

**`isTask`** uses `omitempty` — it is omitted when `false`. The backend treats a missing `isTask` as `false`.

### Endpoints vs. Tasks

| | Endpoint | Task |
|---|----------|------|
| `isTask` | `false` (or omitted) | `true` |
| `endpoint` | `"METHOD /path"` (e.g., `"GET /api/users/:id"`) | Descriptive name (e.g., `"report.monthly"`) |
| `statusCode` | HTTP status code | `0` |
| `bodySize` | Response body size | `0` |
| `clientIP` | Client IP | `""` |

### Span Object

Spans represent sub-operations within a trace.

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "db.query",
  "startTime": "2025-01-15T10:30:00.100Z",
  "duration": 5000000
}
```

| Field | JSON Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Id | `id` | `string` | Yes | UUID v4 identifying this span |
| Name | `name` | `string` | Yes | Descriptive name of the operation |
| StartTime | `startTime` | `string` | Yes | Start timestamp in RFC 3339 format |
| Duration | `duration` | `integer` | Yes | Duration in **nanoseconds** (int64) |

## Exceptions

An exception record represents either an **error/panic** or an explicitly **captured message**. The `isMessage` field distinguishes them.

### ExceptionStackTrace Object

```json
{
  "traceId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "isTask": false,
  "stackTrace": "*errors.errorString: connection refused\nhandleRequest()\n    handler.go:42\n",
  "recordedAt": "2025-01-15T10:30:01.500Z",
  "attributes": { "user_id": "42" },
  "isMessage": false
}
```

| Field | JSON Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| TraceId | `traceId` | `string \| null` | No | UUID of the linked trace, or `null`/omitted for standalone exceptions |
| IsTask | `isTask` | `boolean` | No | `true` if the linked trace is a task. Omit or `false` for endpoints. |
| StackTrace | `stackTrace` | `string` | Yes | Stack trace string or message text |
| RecordedAt | `recordedAt` | `string` | Yes | Timestamp in RFC 3339 format |
| Attributes | `attributes` | `object` | No | Key-value string pairs. Omit or `null` when empty. |
| IsMessage | `isMessage` | `boolean` | Yes | `false` for errors/panics, `true` for captured messages |

### Notes

- **`traceId`** is a nullable string pointer. When there is no linked trace, send `null` or omit the field entirely.
- **`isTask`** uses `omitempty` — omitted when `false`.
- **`isMessage=false`** → error or panic with a stack trace. The backend normalizes and hashes the stack trace for grouping.
- **`isMessage=true`** → an explicitly captured message (e.g., via `CaptureMessage`). The full `stackTrace` string is hashed as-is for grouping.

## Stack Trace Format

The Go client produces stack traces in this format:

```
{ErrorType}: {message}
{FuncName}()
    {file}:{line}
{FuncName}()
    {file}:{line}
...
```

**Example:**

```
*errors.errorString: connection refused
handleRequest()
    handler.go:42
processConnection()
    server.go:128
main()
    main.go:15
```

- The first line contains the Go type name (via `reflect.TypeOf(err).String()`) and the error message.
- Each subsequent frame has the function name (short form, without full package path) on one line, then the file path and line number indented with 4 spaces on the next.
- Function names are shortened: the full path is trimmed to everything after the last `/`, then everything after the first `.` (e.g., `github.com/user/pkg.Handler` → `Handler`).

### Backend Normalization for Grouping

The backend computes a SHA-256 hash (truncated to 16 hex characters) of a normalized form of the stack trace. Normalization (for errors, not messages) includes:

- Removing the error message content (keeping only the error type)
- Replacing absolute file paths with `filename:line`
- Replacing hex addresses, UUIDs, large numbers, email addresses, IP addresses, and goroutine numbers with placeholders
- Removing module version strings (e.g., `@v1.2.3`)
- Normalizing whitespace

This means the same logical error is grouped together even when runtime values differ.

## Metrics

### MetricRecord Object

```json
{
  "name": "cpu.used_pcnt",
  "value": 45.2,
  "recordedAt": "2025-01-15T10:30:00Z"
}
```

| Field | JSON Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Name | `name` | `string` | Yes | Metric name |
| Value | `value` | `number` | Yes | Metric value (float64) |
| RecordedAt | `recordedAt` | `string` | Yes | Timestamp in RFC 3339 format |

### Predefined Metric Names

The Go client collects these system metrics automatically:

| Name | Description | Unit |
|------|-------------|------|
| `mem.used` | Allocated memory | MB |
| `mem.total` | Total system memory | MB |
| `cpu.used_pcnt` | CPU usage percentage | % (0–100) |
| `go.go_routines` | Active goroutine count | count |
| `go.heap_objects` | Heap object count | count |
| `go.num_gc` | Total GC cycles | count |
| `go.gc_pause` | Total GC pause time | nanoseconds |

### Custom Metrics

SDKs can send any `name`/`value` pair. There is no registry — any string name is accepted.

```json
{ "name": "queue.length", "value": 42.0, "recordedAt": "2025-01-15T10:30:00Z" }
```

## Batching and Collection Strategy

This section describes the recommended client-side implementation, based on the Go SDK.

### Collection Interval

Collect telemetry data into a "current frame" (`CollectionFrame`). Every **5 seconds** (default), rotate:

1. Push the current frame into a **send queue** (ring buffer).
2. Create a new empty frame for incoming data.

### Ring Buffer

Use a fixed-capacity ring buffer (default: **12 frames**) for the send queue. When the buffer is full, the oldest frame is overwritten. This prevents unbounded memory growth if uploads fail repeatedly.

### Upload Flow

On each rotation tick:

1. Check if the current frame has data and enough time has passed since it was created.
2. Rotate: push current frame to send queue, set current to `nil`.
3. Trigger upload: read all frames from the send queue, gzip-compress the JSON, POST to the API.
4. On `200 OK`: remove the successfully sent frames from the send queue.
5. On failure: frames remain in the send queue for retry on the next interval.

### Upload Throttling

Enforce a minimum gap between upload attempts (default: **2 seconds**). This prevents rapid-fire uploads if the collection interval is very short.

### Metrics Collection

System metrics (CPU, memory, goroutines, etc.) are collected on a separate interval (default: **30 seconds**) and written into the current collection frame like any other data.

## Sampling

SDKs should support two sampling rates:

| Parameter | Range | Default | Description |
|-----------|-------|---------|-------------|
| `sampleRate` | 0.0–1.0 | 1.0 | Probability of recording a normal trace |
| `errorSampleRate` | 0.0–1.0 | 1.0 | Probability of recording an error trace |

### Sampling Logic

A trace is considered an **error** if:
- A panic was recovered during execution, OR
- The HTTP status code is ≥ 500

The sampling decision should be made **after** execution completes (so you know whether it's an error) but **before** recording:

```
isError = (panicRecovered || statusCode >= 500)
rate = isError ? errorSampleRate : sampleRate

if rate >= 1.0 → always record
if rate <= 0.0 → never record
else → record if random() < rate
```

When a trace is not sampled, do not send the trace or any associated exceptions.

## Response Handling

| Status | Meaning | Action |
|--------|---------|--------|
| `200` | Success | Remove sent frames from the send queue |
| `400` | Missing `Content-Encoding: gzip` or malformed JSON | Fix the request — do not retry as-is |
| `401` | Invalid or expired token | Check the project token configuration |
| `429` | Rate limited | Back off and retry on the next interval |
| Any other | Server error or network issue | Retry on the next interval |

The `200 OK` response body is `{}` (empty JSON object).

## Complete Example Payload

Below is a full JSON payload (before gzip compression) demonstrating all three data types:

```json
{
  "collectionFrames": [
    {
      "stackTraces": [
        {
          "traceId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
          "stackTrace": "*errors.errorString: connection refused\nhandleRequest()\n    handler.go:42\nprocessConnection()\n    server.go:128\n",
          "recordedAt": "2025-01-15T10:30:01.500Z",
          "attributes": {
            "user_id": "1234",
            "endpoint": "/api/users"
          },
          "isMessage": false
        },
        {
          "traceId": null,
          "stackTrace": "Deployment completed successfully for version 1.2.3",
          "recordedAt": "2025-01-15T10:30:02.000Z",
          "isMessage": true
        }
      ],
      "metrics": [
        {
          "name": "cpu.used_pcnt",
          "value": 45.2,
          "recordedAt": "2025-01-15T10:30:00Z"
        },
        {
          "name": "mem.used",
          "value": 256.5,
          "recordedAt": "2025-01-15T10:30:00Z"
        },
        {
          "name": "mem.total",
          "value": 8192.0,
          "recordedAt": "2025-01-15T10:30:00Z"
        },
        {
          "name": "go.go_routines",
          "value": 47.0,
          "recordedAt": "2025-01-15T10:30:00Z"
        },
        {
          "name": "queue.length",
          "value": 12.0,
          "recordedAt": "2025-01-15T10:30:00Z"
        }
      ],
      "traces": [
        {
          "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
          "endpoint": "GET /api/users/:id",
          "duration": 15234000,
          "recordedAt": "2025-01-15T10:30:00.123Z",
          "statusCode": 200,
          "bodySize": 1024,
          "clientIP": "192.168.1.100",
          "attributes": {
            "user_id": "1234"
          },
          "spans": [
            {
              "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
              "name": "db.query.find_user",
              "startTime": "2025-01-15T10:30:00.125Z",
              "duration": 5200000
            },
            {
              "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
              "name": "cache.set",
              "startTime": "2025-01-15T10:30:00.131Z",
              "duration": 800000
            }
          ]
        },
        {
          "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
          "endpoint": "POST /api/orders",
          "duration": 45000000,
          "recordedAt": "2025-01-15T10:30:00.200Z",
          "statusCode": 500,
          "bodySize": 256,
          "clientIP": "10.0.0.50"
        },
        {
          "id": "d4e5f6a7-b8c9-0123-defa-234567890123",
          "endpoint": "report.monthly",
          "duration": 3200000000,
          "recordedAt": "2025-01-15T10:30:00.300Z",
          "statusCode": 0,
          "bodySize": 0,
          "clientIP": "",
          "isTask": true,
          "attributes": {
            "report_type": "revenue"
          }
        }
      ]
    }
  ],
  "appVersion": "1.2.3",
  "serverName": "web-01"
}
```

### Payload Notes

- The first trace is a successful endpoint with two spans and attributes.
- The second trace is a failed endpoint (status 500) with no spans or attributes (fields omitted).
- The third trace is a task (`isTask: true`) with a 3.2-second duration.
- The first exception links to the first trace via `traceId`. The second is a standalone message with no trace link.
- Metrics include both predefined system metrics and a custom metric (`queue.length`).
