# How Traceway Interprets OpenTelemetry Data

This is the framework-agnostic reference for connecting any OpenTelemetry project to Traceway. It explains what Traceway shows in the dashboard, how it classifies incoming OTel spans, and the quirks the instrumentation MUST respect for the data to display correctly. Read this before following any framework-specific guide.

## Sending Data

| What | Value |
|---|---|
| Traces | `POST https://<instance>/api/otel/v1/traces` |
| Metrics | `POST https://<instance>/api/otel/v1/metrics` |
| Logs | `POST https://<instance>/api/otel/v1/logs` |
| Profiles | `POST https://<instance>/api/otel/v1development/profiles` (OTLP profiles, the development signal) |
| Auth | `Authorization: Bearer <project-token>` header on every request |
| Encoding | OTLP/HTTP: both `application/x-protobuf` and `application/json` are accepted. OTLP/gRPC is NOT supported. |
| Compression | `Content-Encoding: gzip` is supported (exporters default to no compression; enabling `compression: gzip` is fine). Max request body: 10 MB. |

Anything whose `Content-Type` is not `application/x-protobuf` or `application/protobuf` is parsed as OTLP/JSON, so a missing `Content-Type` header still works for JSON exporters. When ingest is saturated the endpoints answer `503` with a `Retry-After` header. That is a back-pressure signal to retry, not a misconfiguration.

## What the Dashboard Shows

Traceway turns OTel spans into five distinct concepts, each with its own dashboard page:

| Concept | Dashboard page | Built from |
|---|---|---|
| **Endpoint** | Endpoints (P50/P95/P99, error rates, Apdex) | Root spans that look like HTTP requests |
| **Task** | Tasks (background jobs, cron, consumers) | `CONSUMER`-kind spans |
| **Span** | Endpoint/Task detail -> Spans tab (waterfall) | Child spans (DB queries, outgoing calls, custom work) |
| **Issue** | Issues (grouped errors with stack traces) | `exception` events on spans |
| **AI Trace** | AI Traces (tokens, cost, model) | Spans with `gen_ai.*` attributes |

## Span Classification Rules (exact)

For every incoming span, Traceway applies these rules in order:

1. **Endpoint**: `SpanKind` is `SERVER` or `INTERNAL`, the span has at least one HTTP attribute (`http.request.method`, `http.method`, `http.route`, or `url.path`), AND it is either a root span (no parent) or its parent span is not present in the same export batch (cross-process tracing, e.g. behind a proxy that injects `traceparent`). See the carve-out below.
2. **Task**: `SpanKind` is `CONSUMER`. This applies to ANY consumer span, root or not.
3. **Task**: a root `INTERNAL` span with a `console.command` attribute (CLI command instrumentation, e.g. Laravel/Symfony console).
4. **AI Trace**: the span has any attribute starting with `gen_ai.`.
5. **Child span**: anything else with a parent, stored as a generic span and attached to the nearest promoted ancestor (Endpoint/Task/AI Trace) by walking up the parent chain.
6. **Dropped**: anything else that is a root span. A root span that matches none of the rules above is silently discarded: no entity row, no span row. One exception: if the span carries an exception signal (an `exception` event or `exception.*` attributes), the exception is still captured as an Issue, linked by the raw OTel trace ID. Only the span itself is lost.

One carve-out is checked before all six rules: an `INTERNAL` span carrying `exception.*` **attributes** (`exception.type`, `exception.message` or `exception.stacktrace` set as span attributes, not as an `exception` event) is never promoted to anything. Not an Endpoint, not an AI Trace, not a `console.command` Task. The reason is browser SDKs (e.g. Honeycomb's), which stamp page context like `url.path` onto their zero-duration error spans, and those must become Issues, not endpoints. The consequence beyond browsers: do not set `exception.*` attributes on an `INTERNAL` span you also want promoted, or you lose the entity row and keep only the Issue. An agent that stamps `exception.type` onto its own `INTERNAL` `gen_ai.*` span loses the AI Trace row this way and gets only an Issue. Record the error as an `exception` event instead (`span.recordException(error)`), which does not suppress promotion.

A note on "the same export batch": Traceway resolves parents inside one `ResourceSpans` block, not across the whole request. Spans from one service normally share a Resource and land in one block, so this is invisible. But if an exporter splits a trace across Resource blocks, the children in the other block are treated as cross-process. They link by raw OTel trace ID, and a `SERVER`/`INTERNAL` child carrying HTTP attributes is promoted to a second Endpoint.

The consequences of rule 6 are the most common integration bug: a custom root span created with `tracer.startActiveSpan("my-job")` and default `SpanKind.INTERNAL` produces nothing on the Endpoints or Tasks pages (errors recorded on it do still reach Issues, but with no Endpoint/Task to link back to). Background work must use `SpanKind.CONSUMER` (see Tasks below).

One project-level gate sits above all of these rules: a project whose framework is one of the five browser SDK types (`react`, `svelte`, `vuejs`, `jquery`, `react-native`) never promotes OTel spans to Endpoints/Tasks/AI Traces, and stores no child span rows either. Only exceptions are extracted. Server-side JS frameworks (`nextjs`, `nestjs`, `express`, `remix`) and the mobile frameworks (`ios`, `android`, `flutter`) are NOT gated. Backend OTel exporters must point at a backend-framework project token.

## Endpoints: Name and Route Parameters

This is where most integrations go wrong. The endpoint name displayed and grouped in the dashboard is built as:

```
<method> <route>
```

e.g. `GET /api/users/:id`. The pieces come from span attributes, with these exact rules:

- **Method**: `http.request.method` (current semconv), falling back to `http.method` (old semconv).
- **Route**: `http.route`, falling back to `url.path`. A `http.route` value that does not start with `/` is ignored entirely (treated as absent).
- If the method is present but no route can be resolved, the span name is used as the route. If the method is missing, the route is discarded too and the raw span name alone becomes the endpoint name, even when `http.route` is set correctly. A method attribute is required for the route to be used at all.

### The cardinality quirk: `http.route` is mandatory

`http.route` must contain the **route pattern with parameter placeholders**, never the concrete URL:

```
correct: http.route = /api/users/:id        -> one endpoint: "GET /api/users/:id"
correct: http.route = /api/users/{id}       -> one endpoint: "GET /api/users/{id}"
wrong:   http.route missing, url.path only  -> thousands of endpoints: "GET /api/users/1", "GET /api/users/2", ...
```

The placeholder style (`:id`, `{id}`, `<id>`) does not matter. Traceway uses the route string as-is. What matters is that the string is **identical for every request hitting that route**. If `http.route` is missing, Traceway falls back to `url.path`, which contains real IDs, and the Endpoints page explodes into one row per unique URL. Express and Hono (via `@hono/otel`) set `http.route` to the path template automatically, but verify it in the dashboard after setup; it's the #1 thing to check.

A route that does not start with `/` is discarded outright, as if the attribute were absent. Two real instrumentations trip over this rule:

Symfony caveat: the stock `open-telemetry/opentelemetry-auto-symfony` package sets `http.route` to the Symfony route *name* (e.g. `app_user_show`), not a path template. Traceway ignores it and falls back to `url.path`, so endpoint grouping for raw Symfony auto-instrumentation degrades to literal paths. Use the Traceway Symfony integration (see the Symfony guide) instead.

Django caveat: `opentelemetry-instrumentation-django` copies `http.route` straight from Django's URL pattern, which never has a leading slash (e.g. `api/users/<int:user_id>/`). Traceway discards it, and the default Django instrumentation emits no `url.path` either (it is still on the old HTTP conventions, which use `http.target` and `http.url`). With no method-and-route pair to build from, the endpoint name falls back to the span name, which already starts with the method. Rows then arrive as `GET GET api/users/<int:user_id>/`, one per concrete URL, and never group. See the Django guide for the fix.

### Other endpoint attributes Traceway reads

| Attribute (current semconv) | Legacy fallback also read | Used for |
|---|---|---|
| `http.response.status_code` | `http.status_code` | Status, error rate, 4xx/5xx breakdown |
| `http.response.body.size` | `http.response_content_length` | Response size |
| `client.address` | `net.peer.ip` | Client IP |

Note that `url.path` is the **only** route fallback Traceway reads. An instrumentation still on the old HTTP semantic conventions emits `http.target` and `http.url` instead of `url.path`, so it has no usable fallback at all: if its `http.route` is missing or unusable, there is nothing left to build a route from.

Quirks:

- **404s collapse to `UNMATCHED` only when no real route matched.** A span with a proper `http.route` (starts with `/`, and is not a catch-all made only of slashes and wildcards like `/`, `/*`, `/**`) keeps its own endpoint name even on a 404, because a deliberate "not found" response on a real route is worth seeing under that route. The rewrite to a single `UNMATCHED` row happens when `http.route` is missing or unusable (only `url.path` available), or when the framework reported a catch-all route, which is what Express middleware, Spring resource handlers and not-found handlers emit for unmatched requests. That is what keeps bot scans and typo'd URLs out of the endpoint list.
- **Missing status code becomes `0`, and so does a wrongly typed one.** Traceway reads `http.response.status_code` only when it is an OTLP **integer** value. A status sent as a double or as a string is ignored and the endpoint is stored with status `0`, even though the attribute still shows "200" on the row. The same strictness applies everywhere: `http.route`, `http.request.method` and `db.statement` are read only as strings, and `traceway.is_stream` only as a real boolean (the string `"true"` does nothing). Standard OTel SDKs get the types right; hand-rolled OTLP/JSON exporters are where this goes wrong. Without a usable status code, error tracking and Apdex for that endpoint are meaningless.
- **Array- and map-valued attributes are dropped from the stored attribute map.** Only string, int, double and bool attribute values are kept (non-strings are stringified). An attribute whose value is an array or a nested key/value list never appears on the endpoint, task, span or exception row. Flatten it into scalar attributes if you need it in the dashboard.
- **Streaming endpoints** (long-lived responses that would otherwise look like terrible P99s) are detected when: status is `101` (WebSocket upgrade), the vendor attribute `traceway.is_stream` is a real boolean `true`, or the captured response header attribute (exactly the lowercase key `http.response.header.content-type`) has a value that *starts with* `text/event-stream`. So `text/event-stream; charset=utf-8` matches, but a combined `application/json, text/event-stream` does not. OTel clients don't capture response headers by default, and some instrumentations normalize the key differently (`Content-Type`, `content_type`), which Traceway will not read. For SSE endpoints, setting `traceway.is_stream` manually is the reliable path. Streaming endpoints keep their request count and error rate, but latency percentiles and Apdex are zeroed (connection lifetime is not request latency).
- **Successful health checks are dropped at ingestion, and the rule is broader than `/health`.** Projects are created with this on (`drop_healthy_healthchecks`, toggleable in project settings). An endpoint span is discarded, together with its child spans, when all of these hold: the method is `GET` or `HEAD`, the status code is below 400, and the path matches one of the built-in names (`/health`, `/healthz`, `/healthcheck`, `/health-check`, `/health_check`, `/ping`, `/livez`, `/readyz`, `/live`, `/ready`, `/alive`, `/up`, `/heartbeat`, `/status`, `/ht`, `/actuator/health`), starts with `/actuator/health/`, **ends with `/health`**, or matches a custom pattern configured on the project (`*` allowed at either end). Trailing slashes and case are ignored. Two consequences worth checking: a real application route named `/status` or `/up` will never appear on the Endpoints page, and a health check that fails (4xx/5xx) or that recorded an exception is always kept, which is what keeps outages visible.
- **Apdex thresholds are hardcoded, and they differ by page.** The Endpoints list buckets requests as satisfied up to 750 ms, tolerating up to 1.5 s, bad above that or on any 5xx. Both boundaries shift upward by the endpoint's slow-endpoint offset when one is configured (Endpoints page -> slow endpoint threshold). The endpoint detail page computes its displayed Apdex score with 500 ms / 2 s boundaries instead (no offset), and the apdex-drop notification rule uses flat 750 ms / 1.5 s. None of the thresholds are configurable.

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

- **The span name IS the task name, and Traceway groups tasks by name.** Use a stable identifier like `cleanup-expired-sessions` or `process-email-queue`. Never put job IDs, timestamps, or user IDs in the span name. Each unique name becomes a separate task group.
- **Every `CONSUMER` span becomes a Task, even non-root ones.** If a queue library's auto-instrumentation already emits `CONSUMER` spans (e.g. Kafka/RabbitMQ consumers, Symfony Messenger), do NOT wrap them in another `CONSUMER` span. You'd get duplicate Task entries.
- A root span with `SpanKind.INTERNAL` (the default!) is **dropped silently**. This is the most common reason "my cron job doesn't show up". The kind must be `CONSUMER`. (An exception recorded on the dropped span still reaches Issues, but the task run itself is lost: name, duration, history.)
- **Tasks are timestamped when they finish, endpoints when they start.** A task run lands in the time bucket of its end time, so a nightly job triggered at 01:00 and finishing at 03:00 shows up at 03:00. Widen the time range when a long-running job seems missing.
- Per-job context (job ID, batch size) belongs in span **attributes**, where it shows on the task detail page without affecting grouping.

## Database Queries and Child Spans

Work inside an endpoint or task (DB queries, cache hits, outgoing HTTP calls, custom business logic) should be **child spans** of the entity's root span. They render in the waterfall on the Endpoint/Task detail page.

How Traceway handles them:

- **The span name is replaced by the SQL text when present**: if the span has `db.query.text` (current semconv) or `db.statement` (old semconv), that value is displayed instead of the span name. So a `pg.query` span carrying `db.statement = "SELECT * FROM users WHERE id = $1"` shows the query itself in the waterfall. This is what makes auto-instrumented DB clients (`pg`, `mysql2`, `mongodb`, `ioredis`, Prisma via `@prisma/instrumentation`) useful out of the box.
- **Child spans attach to the nearest promoted ancestor.** The parent chain is walked until an Endpoint/Task/AI Trace is found. This means DB spans must be created **inside the active context** of the request/task span (`startActiveSpan`, or the framework middleware's context). A DB span created outside any active context becomes a root `INTERNAL` span, which is dropped.
- **A promoted span is never also a child span.** Once a span becomes an Endpoint, Task or AI Trace it leaves the waterfall of whatever contains it, and its own children re-root under it. So a `CONSUMER` span created inside a request handler (an inline queue publish, a Symfony Messenger sync dispatch) disappears from that endpoint's Spans tab and shows up on the Tasks page instead, with its sub-spans attached to the Task. If you want the work to stay in the request waterfall, use a plain child span (default `INTERNAL`), not `CONSUMER`.
- Spans whose parent lives in another process/batch and never resolves to a promoted entity are kept but linked by the raw OTel trace ID instead.
- For databases without auto-instrumentation (e.g. SQLite), create a manual child span and set `db.system` + `db.statement` attributes so it renders like the auto-instrumented ones.

## Issues: Exceptions

Errors become **Issues** when recorded as a span **event named `exception`** (`span.recordException(error)` does exactly this) with the standard attributes:

- `exception.type`: error class, e.g. `TypeError`
- `exception.message`: error message
- `exception.stacktrace`: full stack trace text

Setting the same `exception.*` keys as plain span **attributes** (instead of an event) also works. Traceway builds an Issue from the attributes when no `exception` event is present on the span.

Every span event named `exception` produces its own Issue occurrence, so a span that records three exceptions writes three occurrences. The attribute path is used only when the span has no `exception` event at all.

The stack-trace text that gets hashed is picked in this order:

1. Honeycomb-style structured frames, when `exception.structured_stacktrace.urls` is present (with `.functions` / `.lines` / `.columns`). Used by some browser and edge SDKs.
2. `exception.stacktrace`.
3. Android/JVM structured frames, when `exception.structured_stacktrace.classes` is present (with `.methods` / `.source_files` / `.lines`).

An `exception.type: exception.message` header is prepended unless the stack trace already starts with the type, which JVM agents do.

Quirks:

- **Exceptions survive span-dropping, but their context doesn't.** An exception recorded on an unpromoted root span (see classification rule 6) still becomes an Issue, but there is no Endpoint/Task for it to link to. The occurrence carries only the raw trace ID. Such an occurrence is stored with trace type `task`, which is why orphaned errors look task-shaped in the UI. Record exceptions on spans that live inside an Endpoint/Task, or fix the span kind, so Issues stay correlated with the request or job that raised them.
- **`exception.stacktrace` is stripped from every stored attribute map.** The raw blob lives in the Issue's stack trace field only, never duplicated onto the endpoint, task, span or exception rows. Don't re-add it under another key. For JS and Android telemetry the exception's attribute map additionally carries `telemetry.sdk.language`, copied down from the Resource.
- **Grouping is automatic.** Before hashing (SHA-256, truncated to 16 hex chars), Traceway normalizes the stack trace: error messages are stripped (only the error type is kept, including on `Caused by:` lines), JS function-name lines collapse to `<fn>`, URL origins are removed so the same bundle served from different hosts or CDNs groups together, absolute paths reduce to `filename:line`, column numbers are dropped from frames on line 2 or later (the column survives only on line 1, where minified bundles put everything and it is the only thing telling frames apart), dependency version suffixes (`@v1.2.3`) are removed, and runtime values are replaced with placeholders: hex addresses -> `<hex>`, UUIDs -> `<uuid>`, standalone numbers of 5+ digits -> `<id>` (never a number preceded by `:`, so `handler.go:123456` line numbers stay significant), emails -> `<email>`, IPs -> `<ip>`, goroutine IDs -> `goroutine <n>`. JVM frames additionally lose their line numbers (`(Foo.java:123)` -> `(Foo.java)`, same for `.kt` and `.scala`) and `... N more` becomes `... more`. The same logical error therefore groups into one Issue even when runtime values differ, so don't try to pre-group errors client-side.
- **JS projects get source-map symbolication, once the project has a source-map token.** Symbolication is triggered when the `telemetry.sdk.language` resource attribute is a JS value (`nodejs`, `webjs`, `javascript` or `typescript`, all set automatically by OTel SDKs), or when the instrumentation scope name is npm-scoped (`@scope/pkg`) or `next.js`. It is a silent no-op until the project has a source-map token: the trace is canonicalized and stored unresolved, with no error and no warning, and uploaded maps are never consulted. Only iOS projects get a token automatically, so for an OpenTelemetry project generate one with `POST /api/projects/source-map-token` first. Minified frames are resolved server-side **before** grouping, so uploading source maps improves grouping too.
- **Source maps are matched by filename (and debug ID when embedded), not by version.** Upload via `POST /api/sourcemaps/upload` (multipart `files` field, `.map`/`.js`/`.cjs`/`.mjs`, max 50 MB per file) using a token from `POST /api/projects/source-map-token`. A frame referencing `app.min.js` resolves against the uploaded `app.min.js.map` by name. Keep bundle filenames unique per release (content hashes do this) or rely on debug IDs.

## Metrics

OTLP metrics sent to `/api/otel/v1/metrics` are stored under their **original names**. There is no renaming or namespacing. Every incoming metric is auto-registered (name, unit, type) in the metric registry, becomes discoverable in the dashboard's metric explorer, and can be charted in custom widgets or queried via `traceway metrics query --name <name>`.

Conversion rules per OTLP metric type:

| OTLP type | Stored as | Registered type |
|---|---|---|
| Gauge | `<name>`, value as-is | `gauge` |
| Sum (monotonic) | `<name>`, value as-is | `counter` |
| Sum (non-monotonic) | `<name>`, value as-is | `gauge` |
| Histogram | `<name>.count` always; `<name>.avg` (sum/count per export) only when the data point has a non-zero count and a sum | `gauge` for `.avg` (keeps the source unit) + `counter` for `.count` (unit forced to `count`) |
| ExponentialHistogram, Summary | **silently dropped** | - |

Quirks:

- **Histograms lose their buckets.** Only the average and count survive. You cannot compute true percentiles from an OTLP histogram in Traceway. An idle export interval (count 0) writes only the `.count` series, so `<name>.avg` has gaps rather than zeros.
- **No percentile aggregations on metrics at all.** Metric queries support `avg`, `sum`, `count`, `min`, `max` and `last` only; anything else silently falls back to `avg`. `last` is special: it always reads the raw table, so it only works inside the 7-day raw retention window. Latency percentiles (P50/P95/P99) exist for Endpoints and Tasks because those are computed from raw span durations. If you need percentiles on a measurement, model it as a span, not a metric.
- **Tags come from data-point attributes plus a small resource allowlist.** Resource attributes are not copied onto metric points, with these exceptions. `service.name` becomes the `server_name` tag (and overwrites a data-point attribute of that name). And this fixed allowlist is lifted so per-process, per-container and per-pod series stay distinguishable when the receiver puts identity on the Resource: `process.pid`, `process.executable.name`, `process.command_line`, `process.owner`, `container.name`, `container.image.name`, `k8s.pod.name`, `k8s.namespace.name`, `k8s.node.name`, `k8s.deployment.name`, `k8s.container.name`, `postgresql.database.name`. Every other resource attribute (`host.name`, `deployment.environment`, and so on) is dropped. A data-point attribute wins over the resource value when both use the same key. Any other per-series dimension must be a data-point attribute, and keep its cardinality low.
- **Raw metric points expire after 7 days** (ClickHouse TTL). Queries over wider ranges read pre-aggregated rollups instead (1-minute rollups are kept 30 days, 1-hour rollups 1 year, 1-day rollups indefinitely), so older metrics lose granularity rather than disappearing.

### Built-in metric names

The dashboard's built-in system charts (Dashboards page templates, default widget suggestions) read these **exact hardcoded names**:

| Name | Meaning | Unit |
|---|---|---|
| `cpu.used_pcnt` | CPU usage | percent (0–100) |
| `mem.used` | Memory used | MB |
| `mem.total` | Memory total | MB |
| `go.go_routines` | Goroutine count | count |
| `go.heap_objects` | Heap objects | count |
| `go.num_gc` | Total GC cycles | count |
| `go.gc_pause` | Last GC pause | nanoseconds |

An OTel project's metrics (e.g. hostmetrics' `system.cpu.utilization`) do NOT populate those built-in charts. They appear as custom metrics under their own names, chartable via custom widgets. To fill the built-in CPU/memory charts from an OTel pipeline, the metrics must be emitted under the exact names and units above.

## Logs

OTLP logs sent to `/api/otel/v1/logs` appear on the **Logs** page. From each LogRecord, Traceway reads: timestamp (falling back to `observedTimeUnixNano` when `timeUnixNano` is unset), severity number and text, body, trace ID and span ID (stored as hex), and three separate attribute maps (resource, scope, and log-record attributes), each independently filterable.

What the Logs page (and `traceway logs query`) can filter by: minimum severity, service name (from the `service.name` resource attribute), trace ID, span ID, instrumentation scope name, free-text body search, and attribute key/value filters scoped to resource, scope, or log attributes.

Quirks:

- **Emit logs inside the active span context.** The trace ID on a log record is what links it to the Endpoint/Task that produced it. OTel log bridges do this automatically when a span is active.
- **Severity text is derived and uppercased.** `severityText` is stored upper-cased. When the record carries none, Traceway derives it from the severity number: `0` gives an empty string, `1-4` TRACE, `5-8` DEBUG, `9-12` INFO, `13-16` WARN, `17-20` ERROR, `21` and above FATAL. Those buckets do not match the OTel names one for one (WARN2 through WARN4 all become WARN), so filter by severity *number* when you need precision.
- **Non-string bodies are coerced to text.** Numbers, booleans and bytes are stringified; arrays and key/value maps are JSON-encoded into the body string. A structured log body is therefore searchable as its JSON text.
- **Body search over ranges longer than 24 hours requires at least one additional filter** (service, severity, trace, or attribute). Unbounded full-text scans are rejected.
- A JS `exception.stacktrace` attribute on a log record is source-map symbolicated, same as span exceptions.
- Logs are retained for 90 days on ClickHouse deployments (table TTL). On self-hosted SQLite/DuckDB deployments a retention worker prunes them, defaulting to 30 days and configurable. Spans, endpoints, tasks, and exceptions have no fixed expiry on ClickHouse deployments; metrics roll up with their own TTLs (see Metrics above).

## Resource Attributes

Set these once on the OTel `Resource`. They tag everything the service exports:

| Resource attribute | Shown in Traceway as | Notes |
|---|---|---|
| `service.name` | Server Name | Set via `OTEL_SERVICE_NAME` or SDK config. Distinguishes instances/services within a project. |
| `service.version` | App Version | Enables release-over-release comparison. On Cloudflare Workers, derived from `cloudflare.script_version.id` automatically. |
| `telemetry.sdk.language` | - | Drives JS symbolication; set automatically by every OTel SDK. |

## Vendor Extension Attributes

Traceway recognizes these non-standard span attributes:

| Attribute | Type | Purpose |
|---|---|---|
| `traceway.is_stream` | boolean | Mark an endpoint span as streaming (SSE/long-poll) so its duration is excluded from latency percentiles. |
| `traceway.distributed_trace_id` | string (UUID) | Override the distributed trace ID used to link entities across services. Rarely needed. The OTel trace ID is used by default. |

## Verification Checklist

After wiring up any project, confirm in the dashboard (or via `traceway` CLI):

1. **Endpoints page**: routes are grouped by pattern (`GET /api/users/:id`), NOT one row per concrete URL. If you see raw IDs, `http.route` is not being set.
2. **Endpoints page**: status codes are non-zero.
3. **Tasks page**: each scheduled job appears under one stable name after triggering it. (Dashboard only for the grouped list. The CLI has `traceway tasks show <taskId> --recorded-at <ts>` for a single run, but no task list command.)
4. **Issues page**: a deliberately thrown error shows up with a readable stack trace.
5. **Endpoint detail -> Spans tab**: DB queries appear as children showing the SQL text.

With the `traceway` CLI (`--since` takes relative ranges like `1h`, `24h`, `7d`):

```bash
traceway endpoints list --since 1h
traceway exceptions list --since 1h
traceway logs query --since 1h --min-severity 17
traceway metrics query --name <metric-name> --since 1h
```
