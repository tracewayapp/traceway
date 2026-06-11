# How Traceway Interprets OpenTelemetry Data

This is the framework-agnostic reference for connecting any OpenTelemetry project to Traceway. It explains what Traceway shows in the dashboard, how it classifies incoming OTel spans, and the quirks the instrumentation MUST respect for the data to display correctly. Read this before following any framework-specific guide.

## Sending Data

| What | Value |
|---|---|
| Traces | `POST https://<instance>/api/otel/v1/traces` |
| Metrics | `POST https://<instance>/api/otel/v1/metrics` |
| Logs | `POST https://<instance>/api/otel/v1/logs` |
| Auth | `Authorization: Bearer <project-token>` header on every request |
| Encoding | OTLP/HTTP — both `application/x-protobuf` and `application/json` are accepted. OTLP/gRPC is NOT supported. |
| Compression | `Content-Encoding: gzip` is supported (exporters default to no compression; enabling `compression: gzip` is fine). Max request body: 10 MB. |

## What the Dashboard Shows

Traceway turns OTel spans into five distinct concepts, each with its own dashboard page:

| Concept | Dashboard page | Built from |
|---|---|---|
| **Endpoint** | Endpoints (P50/P95/P99, error rates, Apdex) | Root spans that look like HTTP requests |
| **Task** | Tasks (background jobs, cron, consumers) | `CONSUMER`-kind spans |
| **Span** | Endpoint/Task detail → Spans tab (waterfall) | Child spans (DB queries, outgoing calls, custom work) |
| **Issue** | Issues (grouped errors with stack traces) | `exception` events on spans |
| **AI Trace** | AI Traces (tokens, cost, model) | Spans with `gen_ai.*` attributes |

## Span Classification Rules (exact)

For every incoming span, Traceway applies these rules in order:

1. **Endpoint** — `SpanKind` is `SERVER` or `INTERNAL`, the span has at least one HTTP attribute (`http.request.method`, `http.method`, `http.route`, or `url.path`), AND it is either a root span (no parent) or its parent span is not present in the same export batch (cross-process tracing, e.g. behind a proxy that injects `traceparent`).
2. **Task** — `SpanKind` is `CONSUMER`. This applies to ANY consumer span, root or not.
3. **Task** — a root `INTERNAL` span with a `console.command` attribute (CLI command instrumentation, e.g. Laravel/Symfony console).
4. **AI Trace** — the span has any attribute starting with `gen_ai.`.
5. **Child span** — anything else with a parent: stored as a generic span, attached to the nearest promoted ancestor (Endpoint/Task/AI Trace) by walking up the parent chain.
6. **Dropped** — anything else that is a root span. A root span that matches none of the rules above is silently discarded, along with any exception events on it.

The consequences of rule 6 are the most common integration bug: a custom root span created with `tracer.startActiveSpan("my-job")` and default `SpanKind.INTERNAL` produces NOTHING in Traceway. Background work must use `SpanKind.CONSUMER` (see Tasks below).

## Endpoints: Name and Route Parameters

This is where most integrations go wrong. The endpoint name displayed and grouped in the dashboard is built as:

```
<method> <route>
```

e.g. `GET /api/users/:id`. The pieces come from span attributes, with these exact rules:

- **Method**: `http.request.method` (current semconv), falling back to `http.method` (old semconv).
- **Route**: `http.route`, falling back to `url.path`. A `http.route` value that does not start with `/` is ignored entirely (treated as absent).
- If the method is present but no route can be resolved, the span name is used as the route. If neither is present, the raw span name becomes the endpoint name.

### The cardinality quirk — `http.route` is mandatory

`http.route` must contain the **route pattern with parameter placeholders**, never the concrete URL:

```
correct: http.route = /api/users/:id        -> one endpoint: "GET /api/users/:id"
correct: http.route = /api/users/{id}       -> one endpoint: "GET /api/users/{id}"
wrong:   http.route missing, url.path only  -> thousands of endpoints: "GET /api/users/1", "GET /api/users/2", ...
```

The placeholder style (`:id`, `{id}`, `<id>`) does not matter — Traceway uses the route string as-is. What matters is that the string is **identical for every request hitting that route**. If `http.route` is missing, Traceway falls back to `url.path`, which contains real IDs, and the Endpoints page explodes into one row per unique URL. Express, Hono (via `@hono/otel`), and Django instrumentations set `http.route` to the path template automatically — but verify it in the dashboard after setup; it's the #1 thing to check.

Symfony caveat: the stock `open-telemetry/opentelemetry-auto-symfony` package sets `http.route` to the Symfony route *name* (e.g. `app_user_show`), not a path template. Traceway deliberately ignores `http.route` values that don't start with `/` and falls back to `url.path` — so endpoint grouping for raw Symfony auto-instrumentation degrades to literal paths. Use the Traceway Symfony integration (see the Symfony guide) instead.

### Other endpoint attributes Traceway reads

| Attribute (current semconv) | Legacy fallback also read | Used for |
|---|---|---|
| `http.response.status_code` | `http.status_code` | Status, error rate, 4xx/5xx breakdown |
| `http.response.body.size` | `http.response_content_length` | Response size |
| `client.address` | `net.peer.ip` | Client IP |

Quirks:

- **404 responses are renamed to `UNMATCHED`.** Any endpoint span with status 404 is grouped under a single `UNMATCHED` endpoint regardless of its route — bot scans and typo'd URLs don't pollute the endpoint list. Don't be surprised when a legitimate 404-returning route doesn't appear under its own name.
- **Missing status code becomes `0`.** Without `http.response.status_code`, error tracking and Apdex for that endpoint are meaningless. Make sure the instrumentation sets it.
- **Streaming endpoints** (long-lived responses that would otherwise look like terrible P99s) are detected when: status is `101` (WebSocket upgrade), the captured response header `http.response.header.content-type` contains `text/event-stream` (SSE), or the vendor attribute `traceway.is_stream` (boolean) is `true`. OTel clients don't capture response headers by default — for SSE endpoints either enable capture of `content-type` or set `traceway.is_stream` manually. Streaming endpoints keep their request count and error rate, but latency percentiles and Apdex are zeroed (connection lifetime is not request latency).
- **Health-check endpoints are filtered at ingestion** according to the project's health-check configuration, so `GET /health`-style routes don't pollute the endpoint list or stats.
- **Apdex thresholds are fixed**: satisfied at duration up to 750 ms, tolerating up to 1.5 s, bad above that or on any 5xx response.

## Tasks: Scheduled Jobs, Cron, Queue Consumers

Background work appears on the **Tasks** page only when the span has `SpanKind.CONSUMER`:

```typescript
import { trace, SpanKind, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-app");

async function runScheduledJob() {
  await tracer.startActiveSpan(
    "cleanup-expired-sessions",          // becomes the Task name, must be stable
    { kind: SpanKind.CONSUMER },         // without this the span is DROPPED
    async (span) => {
      try {
        await doWork();
        span.setStatus({ code: SpanStatusCode.OK });
      } catch (error) {
        span.recordException(error);     // becomes an Issue, linked to this Task
        span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
        throw error;
      } finally {
        span.end();
      }
    }
  );
}
```

Quirks:

- **The span name IS the task name, and Traceway groups tasks by name.** Use a stable identifier like `cleanup-expired-sessions` or `process-email-queue`. Never put job IDs, timestamps, or user IDs in the span name — each unique name becomes a separate task group.
- **Every `CONSUMER` span becomes a Task, even non-root ones.** If a queue library's auto-instrumentation already emits `CONSUMER` spans (e.g. Kafka/RabbitMQ consumers, Symfony Messenger), do NOT wrap them in another `CONSUMER` span — you'd get duplicate Task entries.
- A root `INTERNAL` span with `SpanKind.INTERNAL` (the default!) is **dropped silently** — this is the most common reason "my cron job doesn't show up". The kind must be `CONSUMER`.
- Per-job context (job ID, batch size) belongs in span **attributes**, where it shows on the task detail page without affecting grouping.

## Database Queries and Child Spans

Work inside an endpoint or task — DB queries, cache hits, outgoing HTTP calls, custom business logic — should be **child spans** of the entity's root span. They render in the waterfall on the Endpoint/Task detail page.

How Traceway handles them:

- **The span name is replaced by the SQL text when present**: if the span has `db.query.text` (current semconv) or `db.statement` (old semconv), that value is displayed instead of the span name. So a `pg.query` span carrying `db.statement = "SELECT * FROM users WHERE id = $1"` shows the query itself in the waterfall — this is what makes auto-instrumented DB clients (`pg`, `mysql2`, `mongodb`, `ioredis`, Prisma via `@prisma/instrumentation`) useful out of the box.
- **Child spans attach to the nearest promoted ancestor.** The parent chain is walked until an Endpoint/Task/AI Trace is found. This means DB spans must be created **inside the active context** of the request/task span (`startActiveSpan`, or the framework middleware's context). A DB span created outside any active context becomes a root `INTERNAL` span — and is dropped.
- Spans whose parent lives in another process/batch and never resolves to a promoted entity are kept but linked by the raw OTel trace ID instead.
- For databases without auto-instrumentation (e.g. SQLite), create a manual child span and set `db.system` + `db.statement` attributes so it renders like the auto-instrumented ones.

## Issues: Exceptions

Errors become **Issues** when recorded as a span **event named `exception`** (`span.recordException(error)` does exactly this) with the standard attributes:

- `exception.type` — error class, e.g. `TypeError`
- `exception.message` — error message
- `exception.stacktrace` — full stack trace text

Quirks:

- **Exceptions on dropped spans are dropped too.** An exception recorded on an unpromoted root span (see classification rule 6) never reaches the Issues page. Record exceptions on spans that live inside an Endpoint/Task, or fix the span kind.
- Traceway also understands Honeycomb-style structured stack traces (`exception.structured_stacktrace.urls` / `.functions` / `.lines` / `.columns` array attributes) — used by some browser/edge SDKs.
- **Grouping is automatic.** Before hashing (SHA-256, truncated to 16 hex chars), Traceway normalizes the stack trace: error messages are stripped (only the error type is kept, including on `Caused by:` lines), JS function-name lines collapse to `<fn>`, absolute paths reduce to `filename:line`, dependency version suffixes (`@v1.2.3`) are removed, and runtime values are replaced with placeholders — hex addresses → `<hex>`, UUIDs → `<uuid>`, numbers of 5+ digits → `<id>`, emails → `<email>`, IPs → `<ip>`, goroutine IDs → `goroutine <n>`. JVM frames additionally lose their line numbers (`(Foo.java:123)` → `(Foo.java)`) and `... N more` becomes `... more`. The same logical error therefore groups into one Issue even when runtime values differ — don't try to pre-group errors client-side.
- **JS projects get source-map symbolication.** Triggered when the `telemetry.sdk.language` resource attribute is a JS value (`nodejs`, `webjs`, `javascript`, `typescript` — OTel SDKs set this automatically), or when the instrumentation scope name is npm-scoped (`@scope/pkg`) or `next.js`. Minified frames are resolved server-side **before** grouping, so uploading source maps improves grouping too.
- **Source maps are matched by filename (and debug ID when embedded), not by version.** Upload via `POST /api/sourcemaps/upload` (multipart `files` field, `.map`/`.js`/`.cjs`/`.mjs`, max 50 MB per file) using a token from `POST /api/projects/source-map-token`. A frame referencing `app.min.js` resolves against the uploaded `app.min.js.map` by name — keep bundle filenames unique per release (content hashes do this) or rely on debug IDs.

## Metrics

OTLP metrics sent to `/api/otel/v1/metrics` are stored under their **original names** — there is no renaming or namespacing. Every incoming metric is auto-registered (name, unit, type) in the metric registry, becomes discoverable in the dashboard's metric explorer, and can be charted in custom widgets or queried via `traceway metrics query --name <name>`.

Conversion rules per OTLP metric type:

| OTLP type | Stored as | Registered type |
|---|---|---|
| Gauge | `<name>`, value as-is | `gauge` |
| Sum (monotonic) | `<name>`, value as-is | `counter` |
| Sum (non-monotonic) | `<name>`, value as-is | `gauge` |
| Histogram | TWO series: `<name>.avg` (sum/count per export) and `<name>.count` | `gauge` + `counter` |
| ExponentialHistogram, Summary | **silently dropped** | — |

Quirks:

- **Histograms lose their buckets.** Only the average and count survive — you cannot compute true percentiles from an OTLP histogram in Traceway.
- **No percentile aggregations on metrics at all.** Metric queries support `avg`, `sum`, `count`, `min`, and `max` only; anything else silently falls back to `avg`. Latency percentiles (P50/P95/P99) exist for Endpoints and Tasks because those are computed from raw span durations — if you need percentiles on a measurement, model it as a span, not a metric.
- **Tags come from data-point attributes only.** Resource attributes are NOT copied onto metric points, with two exceptions: `service.name` becomes the `server_name` tag, and the process-scraper allowlist (`process.pid`, `process.executable.name`, `process.command_line`, `process.owner`) is lifted so hostmetrics per-process series stay distinguishable. Any per-series dimension must be a data-point attribute, and keep its cardinality low.

### Built-in metric names

The dashboard's built-in system charts (Metrics page, default widget suggestions) read these **exact hardcoded names** — they are emitted natively by the Traceway Go SDK:

| Name | Meaning | Unit |
|---|---|---|
| `cpu.used_pcnt` | CPU usage | percent (0–100) |
| `mem.used` | Memory used | MB |
| `mem.total` | Memory total | MB |
| `go.go_routines` | Goroutine count | count |
| `go.heap_objects` | Heap objects | count |
| `go.num_gc` | Total GC cycles | count |
| `go.gc_pause` | Last GC pause | nanoseconds |

An OTel project's metrics (e.g. hostmetrics' `system.cpu.utilization`) do NOT populate those built-in charts — they appear as custom metrics under their own names, chartable via custom widgets. To fill the built-in CPU/memory charts from an OTel pipeline, the metrics must be emitted under the exact names and units above.

## Logs

OTLP logs sent to `/api/otel/v1/logs` appear on the **Logs** page. From each LogRecord, Traceway reads: timestamp (falling back to observed timestamp), severity number and text, body, trace ID and span ID, and three separate attribute maps — resource, scope, and log-record attributes — each independently filterable.

What the Logs page (and `traceway logs query`) can filter by: minimum severity, service name (from the `service.name` resource attribute), trace ID, free-text body search, and attribute key/value filters scoped to resource, scope, or log attributes.

Quirks:

- **Emit logs inside the active span context.** The trace ID on a log record is what links it to the Endpoint/Task that produced it — OTel log bridges do this automatically when a span is active.
- **Body search over ranges longer than 24 hours requires at least one additional filter** (service, severity, trace, or attribute) — unbounded full-text scans are rejected.
- A JS `exception.stacktrace` attribute on a log record is source-map symbolicated, same as span exceptions.
- Logs are retained for 30 days (database TTL); other telemetry has no fixed expiry on ClickHouse deployments.

## Resource Attributes

Set these once on the OTel `Resource` — they tag everything the service exports:

| Resource attribute | Shown in Traceway as | Notes |
|---|---|---|
| `service.name` | Server Name | Set via `OTEL_SERVICE_NAME` or SDK config. Distinguishes instances/services within a project. |
| `service.version` | App Version | Enables release-over-release comparison. On Cloudflare Workers, derived from `cloudflare.script_version.id` automatically. |
| `telemetry.sdk.language` | — | Drives JS symbolication; set automatically by every OTel SDK. |

## Vendor Extension Attributes

Traceway recognizes these non-standard span attributes:

| Attribute | Type | Purpose |
|---|---|---|
| `traceway.is_stream` | boolean | Mark an endpoint span as streaming (SSE/long-poll) so its duration is excluded from latency percentiles. |
| `traceway.distributed_trace_id` | string (UUID) | Override the distributed trace ID used to link entities across services. Rarely needed — the OTel trace ID is used by default. |

## Verification Checklist

After wiring up any project, confirm in the dashboard (or via `traceway` CLI):

1. **Endpoints page** — routes are grouped by pattern (`GET /api/users/:id`), NOT one row per concrete URL. If you see raw IDs, `http.route` is not being set.
2. **Endpoints page** — status codes are non-zero.
3. **Tasks page** — each scheduled job appears under one stable name after triggering it. (Dashboard only — the CLI has no `tasks` command.)
4. **Issues page** — a deliberately thrown error shows up with a readable stack trace.
5. **Endpoint detail → Spans tab** — DB queries appear as children showing the SQL text.

With the `traceway` CLI (`--since` takes relative ranges like `1h`, `24h`, `7d`):

```bash
traceway endpoints list --since 1h
traceway exceptions list --since 1h
traceway logs query --since 1h --min-severity 17
traceway metrics query --name <metric-name> --since 1h
```
