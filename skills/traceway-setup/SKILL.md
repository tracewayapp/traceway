---
name: traceway-setup
description: Connect a project to a Traceway instance so it reports endpoints, spans, errors, background tasks, AI traces, and metrics. Backends use OpenTelemetry over OTLP/HTTP, frontends and mobile apps use the Traceway SDKs, and host metrics use the Traceway OTel Agent. Use when the user wants to add Traceway (or OpenTelemetry tracing that exports to Traceway) to a backend, frontend, full-stack, or mobile project. Accepts a project token and instance URL, e.g. "/traceway-setup with token abc123".
---

# Set Up Traceway in a Project

Connect an existing project to a Traceway instance so it reports endpoints, spans, errors, background tasks, AI traces, and metrics.

## Step 0: Gather Connection Info

| Value | Example | Where to find it |
|---|---|---|
| **Instance URL** | `https://traceway.example.com` | The URL of the Traceway dashboard |
| **Project token** | `abc123...` | Traceway dashboard → Connection page |
| **Source map upload token** (optional, frontend only) | `def456...` | Traceway dashboard → Connection page → Source Maps |

Instance URL and project token may be provided in the invocation (e.g. `/traceway-setup with token abc123 and url https://traceway.example.com`). If either is missing:

1. Check for existing `TRACEWAY_URL` / `TRACEWAY_TOKEN` environment variables or `.env` entries in the project.
2. Still missing: ask the user whether they already have a Traceway account.
   - **Yes**: ask them to open their Traceway dashboard, go to the project's **Connection** page, and paste the instance URL and project token here. If no project exists yet for this app, have them create one first, picking the framework that matches this codebase.
   - **No**: send them to the register page to create an account: https://cloud.tracewayapp.com/register (or `https://<their-instance>/register` if they are self-hosting). After registering and creating a project, the Connection page shows the token; ask them to paste the URL and token here.

Do not proceed without real values. Never invent placeholder values in committed code; wire everything through environment variables.

## Integration Paths

Pick the path by project type. This is not negotiable per framework; it is how Traceway is designed to receive data:

| Project type | Path |
|---|---|
| **Backend** (any language) | OpenTelemetry, exporting OTLP/HTTP to `<instance>/api/otel/v1/*`. Always, including Go. The native Traceway Go SDK is used only when the user explicitly asks for it. |
| **Frontend** (browser SPA) | Traceway `@tracewayapp/<framework>` SDK + bundler plugin + source map upload (see "Frontend and Mobile" below). |
| **Full-stack JS** (Next.js, SvelteKit, Remix) | BOTH: server side via OpenTelemetry AND browser side via the frontend SDK. |
| **Mobile** (Flutter, React Native, Android) | The Traceway platform SDK only. Never OTel. |

Two hard rules apply to every backend integration:

1. **Endpoints MUST arrive parametrized.** `http.route` must be set to the route pattern (`/api/users/:id`), never the concrete URL. Traceway uses the value as-is; when it is missing it falls back to `url.path`, and the Endpoints page explodes into one row per unique URL.
2. **Background work MUST use `SpanKind.CONSUMER`.** A root span with the default `INTERNAL` kind and no HTTP attributes is silently dropped.

### How Traceway classifies spans

| OTel Span | Condition | Traceway Concept |
|---|---|---|
| Root span (or span whose parent lives in another service) | `SpanKind = SERVER` or `INTERNAL` with HTTP attributes | **Endpoint** |
| Any span | `SpanKind = CONSUMER` | **Task** |
| Root span | `SpanKind = INTERNAL` with a `console.command` attribute | **Task** (CLI command) |
| Any span | Has any `gen_ai.*` attribute | **AI Trace** |
| Non-root span | Has a parent span ID, matched none of the above | **Span** (child) |
| Exception event | Event named `"exception"` on any span above | **Issue** |
| Root span | Matched none of the above | **Dropped silently** (including its exception events) |

For the exact classification rules, endpoint naming, metric conversion, and all the quirks, read `data-model.md` in this skill directory. It is the authoritative reference.

## Step 1: Analyze the Architecture

Before changing anything, build a picture of what needs instrumenting:

1. **Frameworks and languages**: detect them by reading `package.json` (Node.js), `go.mod` (Go), `composer.json` (PHP), `requirements.txt`/`pyproject.toml` (Python), `pubspec.yaml` (Flutter), `build.gradle` (Android), or asking the user.
2. **Services and entry points**: in a monorepo, list each deployable service and its entry point. Each service that should report to Traceway needs its own integration, and usually its own project token (ask the user before reusing one token across services).
3. **Background work**: find cron jobs, queue consumers, schedulers, CLI commands, and long-running workers. These must be instrumented as Tasks (Step 3).
4. **AI/LLM usage**: check dependencies for `openai`, `@anthropic-ai/sdk`, `anthropic`, `langchain` / `@langchain/*`, `ai` (Vercel AI SDK), `litellm`, `google-generativeai`, `cohere`, `openrouter`. If any are present, Step 4 applies.
5. **Deployment signals**: note Dockerfiles, `docker-compose.yml`, Kubernetes manifests, Helm charts, deploy/provisioning scripts, `fly.toml`, `vercel.json`, Procfiles. You will use these in Step 5 to set up server metrics.

## Step 2: Backend OTel Setup

The same shape in every language:

1. Install the language's OpenTelemetry SDK plus the auto-instrumentation for the web framework and database clients.
2. Point the OTLP/HTTP exporter at Traceway with the project token as a Bearer header.
3. Set `service.name` (becomes the Server Name in Traceway) and `service.version` (enables release comparison) on the resource.
4. Verify endpoint grouping (the hard rule above) before anything else.

### Exporter configuration

Where the SDK supports the standard env vars, prefer them; they work identically across languages:

```bash
OTEL_SERVICE_NAME=my-service
OTEL_EXPORTER_OTLP_ENDPOINT=https://<instance>/api/otel
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer <project-token>"
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
```

SDKs append `/v1/traces`, `/v1/metrics`, `/v1/logs` to the endpoint automatically. When configuring in code instead, the full URLs are `https://<instance>/api/otel/v1/traces` (and `/v1/metrics`, `/v1/logs`) with header `Authorization: Bearer <project-token>`.

Constraints: OTLP/HTTP only (protobuf or JSON), OTLP/gRPC is NOT supported; gzip is fine; max request body 10 MB.

### Node.js example

Create `instrumentation.js` at the project root and load it before the app (`node --import ./instrumentation.js server.js`):

```javascript
import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";

const url = process.env.TRACEWAY_URL;
const headers = { Authorization: `Bearer ${process.env.TRACEWAY_TOKEN}` };

const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter({ url: `${url}/api/otel/v1/traces`, headers }),
  metricReader: new PeriodicExportingMetricReader({
    exporter: new OTLPMetricExporter({ url: `${url}/api/otel/v1/metrics`, headers }),
    exportIntervalMillis: 30_000,
  }),
  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();
```

Auto-instrumentation covers Express routes (sets `http.route`), status codes, errors, and CJS database clients (`pg`, `mysql2`, `mongodb`, `ioredis`). SQLite and custom business logic need manual `tracer.startActiveSpan()` child spans.

### Per-language notes

- **Go**: use the contrib middleware for the framework: `otelgin`, `otelchi` (with `otelchi.WithChiRoutes(r)`), `otelfiber`; they set `http.route` from the route pattern. For stdlib `net/http`, current `otelhttp` derives `http.route` from Go 1.23+ `ServeMux` method+pattern routes (`GET /api/users/{id}`); for other routers set `http.route` manually on the active span. Exporter: `otlptracehttp.WithEndpointURL(...)` + `WithHeaders`.
- **Python**: `opentelemetry-distro` + `opentelemetry-bootstrap -a install`, run under `opentelemetry-instrument` with the env vars above. Django/Flask/FastAPI instrumentations set `http.route`.
- **PHP**: Symfony and Laravel via their OTel packages and `OTEL_*` env vars. Symfony caveat: the stock auto-instrumentation sets `http.route` to the route NAME, which Traceway ignores; use the Traceway Symfony integration (`composer require traceway/opentelemetry-symfony`). See `data-model.md`.
- **Java / .NET / anything else**: the standard OTel agent or SDK with the env vars above works as-is.
- Full per-framework docs: https://docs.tracewayapp.com/client/otel

### Verify endpoint grouping (do this first)

Hit a parametrized route a few times with different IDs and check the Traceway Endpoints page: you must see ONE row (`GET /api/users/:id`), not one row per ID. If you see raw IDs, the instrumentation is not setting `http.route`; fix that before continuing. Last resort, set it manually in a middleware that knows the matched route pattern:

```javascript
import { trace } from "@opentelemetry/api";

trace.getActiveSpan()?.setAttribute("http.route", matchedRoutePattern);
```

### Errors

Thrown errors must be recorded as exception events to appear as Issues. Auto-instrumentation handles uncaught errors; for caught-and-handled ones:

```javascript
import { trace, SpanStatusCode } from "@opentelemetry/api";

const span = trace.getActiveSpan();
span?.recordException(error);
span?.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
```

(Go: `span.RecordError(err, trace.WithStackTrace(true))`; the stack trace option is what produces the `exception.stacktrace` attribute.)

### If the user explicitly asks for the Traceway Go SDK

Only on explicit request, instead of OTel: `go get go.tracewayapp.com/tracewaygin` (or `tracewaychi`, `tracewayfiber`, `tracewayfasthttp`, `tracewayhttp`), add the middleware with connection string `<project-token>@https://<instance>/api/report`. Trade-off: the Go SDK natively emits the built-in system metric names (`cpu.used_pcnt`, `mem.used`, `go.*`) that populate the dashboard's built-in charts; on the OTel path those charts stay empty and host metrics come from the Traceway OTel Agent (Step 5).

## Step 3: Background Tasks (Boundaries and Labeling)

If Step 1 found background work, instrument it as Tasks. The rules:

- **Boundary: one Task = one execution of a unit of background work.** A whole cron job run is one task. One queue message or job is one task. One CLI command invocation is one task. Per-item work inside a run (each email in a batch, each row in an import) is a child span of the task span, never a separate `CONSUMER` span.
- **Do not double-wrap.** If a library's auto-instrumentation already emits `CONSUMER` spans (Kafka, RabbitMQ, Symfony Messenger consumers), wrapping them again creates duplicate Task entries.
- **Labeling: the span name IS the task name and the grouping key.** Use a stable identifier like `cleanup-expired-sessions` or `process-email-queue`. Never embed job IDs, timestamps, or user IDs in the name; each unique name becomes a separate task group. Dynamic context (job ID, batch size) belongs in span attributes, where it shows on the task detail page without affecting grouping.
- **The kind must be `CONSUMER`.** A root span with the default `SpanKind.INTERNAL` is dropped silently; this is the most common reason "my cron job doesn't show up". CLI commands may alternatively be root `INTERNAL` spans with a `console.command` attribute (Laravel/Symfony console instrumentation does this).

```typescript
import { trace, SpanKind, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-app");

async function runScheduledJob() {
  await tracer.startActiveSpan(
    "cleanup-expired-sessions",
    { kind: SpanKind.CONSUMER },
    async (span) => {
      try {
        await doWork();
        span.setStatus({ code: SpanStatusCode.OK });
      } catch (error) {
        span.recordException(error);
        span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
        throw error;
      } finally {
        span.end();
      }
    }
  );
}
```

(Go: `tracer.Start(ctx, "cleanup-expired-sessions", trace.WithSpanKind(trace.SpanKindConsumer))`.)

## Step 4: AI Traces

If Step 1 found AI/LLM dependencies, instrument the model calls. Any span carrying at least one `gen_ai.*` attribute is promoted to an **AI Trace**; since calls usually happen inside a request or task, the span is naturally a child and stays linked to its Endpoint/Task by trace ID.

Boundaries: **one span per model call** (one provider API request = one AI Trace row). A multi-step agent run is multiple spans sharing a stable `trace.name` (same labeling discipline as task names: no IDs or timestamps). For streaming, end the span when the stream finishes. Set `user.id` to break down usage and cost per user.

Attributes Traceway reads (all optional, set what is available):

| Attribute | Meaning |
|---|---|
| `gen_ai.request.model` / `gen_ai.response.model` | Requested / serving model |
| `gen_ai.system` or `gen_ai.provider.name` | Provider (`openai`, `anthropic`, ...) |
| `gen_ai.operation.name` | Operation (`chat`, `embeddings`, ...) |
| `gen_ai.usage.input_tokens` / `.output_tokens` / `.total_tokens` | Token counts |
| `gen_ai.usage.input_tokens.cached` / `gen_ai.usage.output_tokens.reasoning` | Cached / reasoning tokens |
| `gen_ai.usage.input_cost` / `.output_cost` / `.total_cost` | Cost, when you compute pricing |
| `trace.name` | Agent/workflow grouping name |
| `user.id` | End-user attribution |
| `gen_ai.response.finish_reason` | Why generation stopped |
| `gen_ai.prompt` / `gen_ai.completion` | Conversation content, shown on the trace detail page (skip if content must not leave the app) |

```typescript
return tracer.startActiveSpan("chat-completion", async (span) => {
  const response = await openai.chat.completions.create({ model: "gpt-4o", messages });
  span.setAttributes({
    "gen_ai.system": "openai",
    "gen_ai.request.model": "gpt-4o",
    "gen_ai.usage.input_tokens": response.usage.prompt_tokens,
    "gen_ai.usage.output_tokens": response.usage.completion_tokens,
    "trace.name": "support-agent",
  });
  span.end();
  return response;
});
```

Zero-code path for OpenRouter users: in OpenRouter Settings → Observability, add an OpenTelemetry Collector destination pointing at `https://<instance>/api/otel/v1/traces` with header `{"Authorization": "Bearer <project-token>"}`. Docs: https://docs.tracewayapp.com/client/openrouter

## Frontend and Mobile

Frontend and mobile projects do NOT use OTel; they use the Traceway SDKs reporting to `/api/report` with connection string `<project-token>@https://<instance>/api/report`.

**Browser** (React / Vue / Svelte / jQuery / plain JS), three pieces, all expected:

1. **SDK**: `npm install @tracewayapp/react` (or `vue`, `svelte`, `jquery`, `frontend` for plain JS) and initialize with the connection string (React: wrap the app in `<TracewayProvider connectionString="...">`). Captures errors, web vitals, and session replay.
2. **Bundler plugin**: `npm install -D @tracewayapp/bundler-plugin`, then add `tracewayDebugIds()` from `@tracewayapp/bundler-plugin/vite` (or `/rollup`, or `TracewayDebugIdsWebpackPlugin` from `/webpack`) to the bundler config, with source maps enabled (`build.sourcemap: true` / `devtool: "source-map"`).
3. **Source map upload**: `npm install -D @tracewayapp/sourcemap-upload`, then run `traceway-sourcemaps --url <instance> --token <source-map-upload-token> --directory ./dist` as a postbuild or CI step (env vars: `TRACEWAY_URL`, `TRACEWAY_SOURCEMAP_TOKEN`). The upload token comes from Step 0 and is a CI secret, never committed.

Full docs: https://docs.tracewayapp.com/client/react (or `vue`, `svelte`, `jquery`, `js-sdk`).

**Full-stack JS** (Next.js, SvelteKit, Remix): both sides. Server side follows Step 2 (verify `http.route` grouping; set it manually in a server hook where the auto-instrumentation does not know the router). Browser side follows the three pieces above.

**Mobile**, always the platform SDK, never OTel:

- **Flutter**: `flutter pub add traceway`, then `Traceway.run(connectionString: '<token>@https://<instance>/api/report', child: MyApp())`. Docs: https://docs.tracewayapp.com/client/flutter
- **React Native**: `npm install @tracewayapp/react-native`, wrap the app in `TracewayProvider`. Docs: https://docs.tracewayapp.com/client/react-native
- **Android**: `implementation("com.tracewayapp:traceway:<version>")`, call `Traceway.init(...)` at startup. Docs: https://docs.tracewayapp.com/client/android

## Step 5: Deployment and Server Metrics

After the code-side integration is in place, ask the user two questions (pre-fill your best guess from the deployment signals found in Step 1 and let them confirm):

1. **How is this project deployed?** Docker on a VM / directly on a VM or bare metal / Kubernetes / serverless or PaaS.
2. **Do you want server (host) metrics tracked in Traceway?** CPU, memory, disk, filesystem, network of the machine running the app.

| Deployment | Wants host metrics | What to do |
|---|---|---|
| Docker on a VM, or directly on a VM/host | Yes | Install the **Traceway OTel Agent** on the host (below). For Docker deploys this is the default; the agent goes on the host, not in a container. |
| Kubernetes | Any | Agent not applicable (host service, no Docker image or K8s manifests by design). In-process app metrics still flow via the OTLP metrics exporter from Step 2. |
| Serverless / PaaS | Any | No host to install on; skip. |
| Anything | No | Skip. |

The agent is a tiny pre-configured OTel Collector that scrapes host metrics every 60s and ships them with the project token. Install on the host (Linux systemd / macOS launchd; PowerShell installer exists for Windows):

```bash
curl -fsSL https://install.tracewayapp.com/install.sh | \
  TRACEWAY_TOKEN=<project-token> \
  TRACEWAY_ENDPOINT=https://<instance>/api/otel \
  TRACEWAY_SERVICE_NAME=<host-label, e.g. api-prod-eu-1> \
  bash
```

- `TRACEWAY_ENDPOINT` ends in `/api/otel` and is required for self-hosted instances; omit only for Traceway Cloud.
- Optional: `TRACEWAY_LOG_PATHS` (comma-separated globs to tail as logs), `TRACEWAY_PROCESS_NAMES` (per-process metrics).
- Re-running the installer upgrades in place, so the command is safe to keep in provisioning scripts.
- If the repo has host provisioning or deploy scripts (cloud-init, Ansible, Terraform `user_data`, `deploy.sh`), add the command there with the token referenced from a secret. Otherwise hand the operator the filled-in one-liner; do not modify the repo.

Metrics arrive within ~60s under their hostmetrics names (`system.cpu.utilization`, `system.memory.usage`, ...), tagged with `service.name` and `host.name`. They are chartable via custom widgets; they do NOT populate the built-in CPU/memory charts (those read the Go SDK's exact names). Agent repo: https://github.com/tracewayapp/traceway-otel-agent

## Step 6: Verify

1. Start the app and hit a few endpoints (or trigger an error on purpose).
2. Check the Traceway dashboard:
   - **Endpoints page**: routes appear grouped by pattern (e.g. `GET /api/users/:id`), not by literal URL, and status codes are non-zero.
   - **Issues page**: thrown errors appear with stack traces. For frontend projects, stack traces are symbolicated to original files and lines.
   - **Endpoint detail → Spans tab**: database queries and outgoing calls appear as children.
   - **Tasks page**: after triggering each background job once, it appears under one stable name.
   - **AI Traces page** (if Step 4 applied): model calls appear with token counts and cost.
   - **Metrics** (if the agent was installed): host metrics arrive within about 60 seconds.
3. If the `traceway` CLI is installed and authenticated, verify from the terminal instead:
   ```bash
   traceway endpoints list --since 15m
   traceway exceptions list --since 15m
   traceway metrics query --name system.cpu.utilization --since 15m
   ```
