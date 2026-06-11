---
name: traceway-setup
description: Connect a project to a Traceway instance so it reports endpoints, spans, errors, and metrics. Use when the user wants to add Traceway (or OpenTelemetry tracing that exports to Traceway) to a backend, frontend, or mobile project. Accepts a project token and instance URL, e.g. "/traceway-setup with token abc123".
---

# Set Up Traceway in a Project

Connect an existing project to a Traceway instance so it reports endpoints, spans, errors, and metrics.

## Step 0: Gather Connection Info

Two values are required:

| Value | Example | Where to find it |
|---|---|---|
| **Instance URL** | `https://traceway.example.com` | The URL of the Traceway dashboard |
| **Project token** | `abc123...` | Traceway dashboard → Connection page |
| **Source map upload token** (optional) | `def456...` | Traceway dashboard → Connection page → Source Map Upload |

Both may be provided in the invocation (e.g. `/traceway-setup with token abc123 and url https://traceway.example.com`). If either is missing, check for existing `TRACEWAY_URL` / `TRACEWAY_TOKEN` environment variables or `.env` entries in the project — otherwise ask the user before proceeding. Never invent placeholder values in committed code; wire everything through environment variables.

If a **source map upload token** is provided and the project produces minified/bundled JavaScript, add source map upload to the build or deploy step so stack traces resolve to original files and lines:

```bash
npx @tracewayapp/sourcemap-upload --url <instance-url> --token <source-map-upload-token> --version <release-version> --directory <build-output, e.g. dist/assets>
```

Do not commit the upload token; keep it in CI secrets or an untracked env file.

## What Traceway Needs

For the integration to work correctly, the instrumentation MUST capture:

1. **Endpoints grouped by route pattern** — `GET /api/users/1` and `GET /api/users/2` must appear as a single `GET /api/users/:id` endpoint, NOT as separate entries. This requires `http.route` to be set on the root span. Without it, the Traceway dashboard explodes with thousands of unique URL entries.

2. **Status codes** — `http.response.status_code` must be set on spans so Traceway can track error rates, 4xx/5xx breakdowns, and Apdex scores.

3. **Exceptions with stack traces** — Thrown errors must be recorded as span events with `exception.type`, `exception.message`, and `exception.stacktrace` attributes. These appear as **Issues** in Traceway.

4. **Scheduled/long-running tasks** — Background jobs (cron, queues, consumers) must create root spans with `SpanKind.CONSUMER`. This is how Traceway distinguishes Tasks from Endpoints. Without the correct span kind, background work either gets misclassified as an Endpoint or dropped entirely.

### How Traceway classifies spans

| OTel Span | Condition | Traceway Concept |
|---|---|---|
| Root span | `SpanKind = SERVER` or `INTERNAL` with HTTP attributes | **Endpoint** |
| Root span | `SpanKind = CONSUMER` | **Task** |
| Non-root span | Has a parent span ID | **Span** |
| Exception event | Event named `"exception"` on any span | **Issue** |

## Step 1: Analyze the Architecture

Before changing anything, build a picture of what needs instrumenting:

1. **Frameworks and languages**: detect them by reading `package.json` (Node.js), `go.mod` (Go), `composer.json` (PHP), `requirements.txt`/`pyproject.toml` (Python), `pubspec.yaml` (Flutter), `build.gradle` (Android), or asking the user.
2. **Services and entry points**: in a monorepo, list each deployable service and its entry point. Each service that should report to Traceway needs its own integration, and usually its own project token (ask the user before reusing one token across services).
3. **Background work**: find cron jobs, queue consumers, schedulers, and long-running workers. These must be instrumented as Tasks (root spans with `SpanKind.CONSUMER`), not Endpoints.

Then follow the framework-specific guide for each service that needs instrumenting.

## Step 2: Follow the Framework-Specific Guide

### Hono (Node.js)
Follow `hono.md` in this skill directory. Uses `@hono/otel` middleware — do NOT use `@opentelemetry/instrumentation-http` (it doesn't work with Hono's ESM imports on Node 22+).
- Endpoints: `@hono/otel` sets `http.route` automatically
- Status codes: `@hono/otel` sets them automatically
- Exceptions: `@hono/otel` records thrown errors automatically
- Tasks: No built-in scheduler — use `SpanKind.CONSUMER` manually for background work

### NestJS (Node.js)
Follow `nestjs.md` in this skill directory. Simplest integration — Express auto-instrumentation handles everything.
- Endpoints: `instrumentation-express` sets `http.route` automatically
- Status codes: `instrumentation-http` sets them automatically
- Exceptions: Express error handling records them automatically
- Tasks: Wrap `@nestjs/schedule` cron jobs and `@nestjs/bull` queue consumers with `SpanKind.CONSUMER` spans

### Next.js (Node.js)
Follow `nextjs.md` in this skill directory. Requires `withRoute()` wrapper for API routes and `@prisma/instrumentation` for database tracing.
- Endpoints: `withRoute()` helper must be added manually to every API route handler
- Status codes: Set by the HTTP instrumentation
- Exceptions: `withRoute()` catches and records thrown errors
- Tasks: No built-in scheduler — use `SpanKind.CONSUMER` manually for background work

### Express (Node.js)
- Install: `@opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @opentelemetry/exporter-trace-otlp-http @opentelemetry/exporter-metrics-otlp-http @opentelemetry/api`
- Create `instrumentation.js` at project root with `NodeSDK` + `getNodeAutoInstrumentations()`
- No app code changes needed — auto-instrumentation captures routes, status codes, errors
- Start with `node --import ./instrumentation.js server.js`
- Tasks: Use `SpanKind.CONSUMER` manually for background work
- Full docs: https://docs.tracewayapp.com/client/node-sdk

### Gin / Chi / Fiber / FastHTTP / stdlib (Go)
- Install the framework-specific middleware: `go get go.tracewayapp.com/tracewaygin` (or `tracewaychi`, `tracewayfiber`, `tracewayfasthttp`, `tracewayhttp`)
- Add middleware: `r.Use(tracewaygin.New("token@http://traceway:8082/api/report"))`
- Reports via Traceway's native protocol (`/api/report`), not OTel
- Endpoints, status codes, exceptions, and tasks are all handled by the Go SDK automatically
- Full docs: https://docs.tracewayapp.com/client/gin-middleware (or the corresponding framework page)

### Django (Python)
- Uses OTel auto-instrumentation for Django
- Full docs: https://docs.tracewayapp.com/client/django

### Symfony (PHP)
- Install: `composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter`
- Configure via `.env` with `OTEL_*` variables
- Add `\OpenTelemetry\SDK\SdkAutoloader::autoload()` to `public/index.php`
- Endpoints and status codes: handled by Symfony OTel auto-instrumentation
- Tasks: Symfony Messenger consumers are auto-instrumented as Tasks
- Full docs: https://docs.tracewayapp.com/client/symfony

### Laravel (PHP)
- Full docs: https://docs.tracewayapp.com/client/laravel

### React / Vue / Svelte / jQuery / plain JS (Frontend)
- Install the framework-specific Traceway SDK: `npm install @tracewayapp/react` (or `@tracewayapp/vue`, `@tracewayapp/svelte`, `@tracewayapp/jquery`)
- These are client-side SDKs that report to `/api/report`, not OTel
- They capture JS errors (as Issues), page loads, and web vitals; upload source maps for readable stack traces from minified bundles
- Full docs: https://docs.tracewayapp.com/client/react (or `vue`, `svelte`, `jquery`, `js-sdk` for plain JavaScript)

### React Native / Flutter / Android (Mobile)
- Install the platform Traceway SDK and initialize it with the instance URL + project token at app startup
- Full docs: https://docs.tracewayapp.com/client/react-native, https://docs.tracewayapp.com/client/flutter, https://docs.tracewayapp.com/client/android

### Cloudflare Workers
- Uses Cloudflare's built-in OTLP export, not the Node SDK
- Scheduled handlers (`scheduled` event) create root spans automatically
- Full docs: https://docs.tracewayapp.com/client/cloudflare

### Any Other Language (Generic OTel)
- Use any OpenTelemetry SDK for the language
- Export via OTLP/HTTP to `https://<traceway-instance>/api/otel/v1/traces` and `/v1/metrics`
- Set `Authorization: Bearer <project-token>` header
- Ensure `http.route` is set on root SERVER spans (not just `url.path`)
- Use `SpanKind.CONSUMER` for background/scheduled work
- Full docs: https://docs.tracewayapp.com/client/otel

## Instrumenting Background Tasks (All Frameworks)

For any framework, background work (cron jobs, queue consumers, scheduled tasks) must create a root span with `SpanKind.CONSUMER` to appear as a **Task** in Traceway:

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

Without `SpanKind.CONSUMER`, the span would either be classified as an Endpoint (wrong) or dropped.

## Common Across All Node.js Frameworks

- **Traceway URL**: `https://<instance>/api/otel/v1/traces` and `/v1/metrics`
- **Auth header**: `Authorization: Bearer <project-token>`
- **Environment variables**: `TRACEWAY_URL` and `TRACEWAY_TOKEN` (or standard `OTEL_*` vars)
- **Auto-instrumented child spans** (CJS packages only): `pg`, `mysql2`, `mongodb`, `ioredis`, `redis`, Prisma (with `@prisma/instrumentation`), outgoing `fetch()` via `instrumentation-undici`
- **Not auto-instrumented**: SQLite (`better-sqlite3`), custom business logic — use `tracer.startActiveSpan()` manually

## Step 3: Verify

1. Start the app and hit a few endpoints (or trigger an error on purpose).
2. Check the Traceway dashboard:
   - **Endpoints page** — routes appear grouped by pattern (e.g. `GET /api/users/:id`), not by literal URL
   - **Issues page** — thrown errors appear with stack traces
   - **Endpoint detail → Spans tab** — database queries and outgoing calls appear as children
3. If the `traceway` CLI is installed and authenticated, verify from the terminal instead:
   ```bash
   traceway endpoints list --since 15m
   traceway exceptions list --since 15m
   ```
