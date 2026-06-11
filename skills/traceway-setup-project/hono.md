# Add Traceway to a Hono Project

Add OpenTelemetry tracing to an existing Hono (Node.js) project so it reports endpoints, spans, and errors to a Traceway instance.

## Prerequisites

- Hono app running on Node.js via `@hono/node-server`
- A Traceway instance URL and project token (get these from your Traceway dashboard → Connection page)

## Step 1: Install Dependencies

```bash
npm install @hono/otel \
  @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/exporter-metrics-otlp-http \
  @opentelemetry/api
```

`hono` and `@hono/node-server` should already be in the project.

## Step 2: Create the Instrumentation File

Create `instrumentation.js` (or `.ts`) at the project root — next to `package.json`, NOT inside `src/`:

```javascript
import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";

const tracewayUrl = process.env.TRACEWAY_URL || "https://your-traceway-instance.com";
const tracewayToken = process.env.TRACEWAY_TOKEN || "your-project-token";

const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter({
    url: `${tracewayUrl}/api/otel/v1/traces`,
    headers: { Authorization: `Bearer ${tracewayToken}` },
  }),

  metricReader: new PeriodicExportingMetricReader({
    exporter: new OTLPMetricExporter({
      url: `${tracewayUrl}/api/otel/v1/metrics`,
      headers: { Authorization: `Bearer ${tracewayToken}` },
    }),
    exportIntervalMillis: 30_000,
  }),

  // IMPORTANT: Disable instrumentation-http.
  // @hono/node-server imports Node's `http` via ESM. On Node 22+, OTel's
  // import-in-the-middle hook cannot intercept ESM imports of built-in modules,
  // so instrumentation-http never patches the server. @hono/otel handles HTTP
  // spans at the middleware level instead.
  instrumentations: [
    getNodeAutoInstrumentations({
      "@opentelemetry/instrumentation-http": { enabled: false },
    }),
  ],
});

sdk.start();
```

### Key decisions in this file

- **No `serviceName` / `serviceVersion` in SDK config.** Set these via `OTEL_SERVICE_NAME` env var or pass them to `otel()` in step 3.
- **`instrumentation-http` is disabled.** `@hono/otel` replaces it. Without this, you get either duplicate spans or zero spans (depending on Node version).
- **Other auto-instrumentations stay enabled.** CJS npm packages (`pg`, `mysql2`, `mongodb`, `ioredis`) are automatically traced. Only ESM-imported Node.js built-in modules have the patching issue.

## Step 3: Add the OTel Middleware

In your main server file, add two lines — import `otel` and register it as middleware **before** your routes:

```javascript
import { otel } from "@hono/otel";

// Add BEFORE route definitions
app.use(otel());
```

Full example:

```javascript
import { serve } from "@hono/node-server";
import { Hono } from "hono";
import { otel } from "@hono/otel";

const app = new Hono();

app.use(otel()); // ← this is the only change to your app code

// ... existing routes unchanged ...
app.get("/api/users", (c) => c.json({ users: [] }));
app.get("/api/users/:id", (c) => c.json({ id: c.req.param("id") }));

serve({ fetch: app.fetch, port: 3000 });
```

### What `otel()` does automatically

- Creates a root SERVER span for every request
- Sets `http.route` to the matched route pattern (e.g., `/api/users/:id`) so Traceway groups endpoints correctly
- Sets `http.request.method`, `http.response.status_code`, `url.full`
- Records thrown exceptions as span events with `exception.type`, `exception.message`, `exception.stacktrace` — these appear as Issues in Traceway
- Sets span status to ERROR on exceptions

### Optional: pass config to `otel()`

```javascript
app.use(otel({
  serviceName: "my-app",
  serviceVersion: "1.0.0",
  captureRequestHeaders: ["user-agent", "x-request-id"],
  captureResponseHeaders: ["x-trace-id"],
}));
```

## Step 4: Update the Start Script

The instrumentation file must load **before** the app code. Update `package.json`:

```json
{
  "scripts": {
    "start": "node --import ./instrumentation.js server.js"
  }
}
```

If using CommonJS:

```json
{
  "scripts": {
    "start": "node --require ./instrumentation.js server.js"
  }
}
```

Or set via environment variable:

```bash
export NODE_OPTIONS="--import ./instrumentation.js"
```

## Step 5: Set Environment Variables

```bash
TRACEWAY_URL=https://your-traceway-instance.com
TRACEWAY_TOKEN=your-project-token
```

## What Gets Traced Automatically

After completing steps 1-5, the following produces traces with zero additional code:

| What | How | Span type in Traceway |
|------|-----|----------------------|
| Every incoming HTTP request | `@hono/otel` middleware | **Endpoint** (root SERVER span) |
| Route grouping (`/users/1` + `/users/2` → `/users/:id`) | `@hono/otel` sets `http.route` | Endpoint attribute |
| Thrown errors | `@hono/otel` calls `span.recordException()` | **Issue** |
| Outgoing `fetch()` calls | `instrumentation-undici` (auto) | **Span** (child) |
| PostgreSQL queries (`pg`) | `instrumentation-pg` (auto) | **Span** (child) |
| MySQL queries (`mysql2`) | `instrumentation-mysql2` (auto) | **Span** (child) |
| MongoDB queries | `instrumentation-mongodb` (auto) | **Span** (child) |
| Redis operations (`ioredis`) | `instrumentation-ioredis` (auto) | **Span** (child) |
| DNS lookups | `instrumentation-dns` (auto) | **Span** (child) |

## Adding Manual Spans (Optional)

For operations without auto-instrumentation (SQLite, custom business logic, external APIs via non-fetch clients):

```javascript
import { trace, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-app");

// Wrap any operation in a span
function dbSpan(name, query, fn) {
  return tracer.startActiveSpan(name, (span) => {
    span.setAttribute("db.system", "sqlite");
    span.setAttribute("db.statement", query);
    try {
      const result = fn();
      span.end();
      return result;
    } catch (error) {
      span.recordException(error);
      span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
      span.end();
      throw error;
    }
  });
}

// Use in a route handler — automatically becomes a child of the @hono/otel root span
app.get("/api/users", (c) => {
  const users = dbSpan("db.query", "SELECT * FROM users", () =>
    db.prepare("SELECT * FROM users").all()
  );
  return c.json(users);
});
```

## Instrumenting Background Tasks (Optional)

Hono has no built-in scheduler. If your app runs cron jobs or queue consumers (via `node-cron`, `bullmq`, etc.), wrap them with `SpanKind.CONSUMER` so they appear as **Tasks** in Traceway:

```javascript
import { trace, SpanKind, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-app");

// In your cron/queue handler
async function processQueueJob(job) {
  await tracer.startActiveSpan(
    "process-email-queue",
    { kind: SpanKind.CONSUMER },
    async (span) => {
      try {
        span.setAttribute("job.id", job.id);
        await sendEmail(job.data);
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

Without `SpanKind.CONSUMER`, background work would be dropped or misclassified as an Endpoint.

## Recording Caught Exceptions (Optional)

`@hono/otel` auto-records thrown errors. For errors you catch and handle:

```javascript
import { trace, SpanStatusCode } from "@opentelemetry/api";

app.get("/api/checkout", async (c) => {
  const span = trace.getActiveSpan();
  try {
    await processPayment();
  } catch (error) {
    if (span) {
      span.recordException(error);
      span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
    }
    return c.json({ error: "Payment failed" }, 500);
  }
  return c.json({ status: "ok" });
});
```

## Verification

After starting your app, hit a few endpoints and check Traceway:

1. **Endpoints page** — routes should appear grouped by pattern (e.g., `GET /api/users/:id`), not by literal URL
2. **Issues page** — any thrown errors should appear with stack traces
3. **Endpoint detail → Spans tab** — database queries, outgoing fetch calls, and manual spans should appear as children of the HTTP request

## Checklist

- [ ] `@hono/otel` and OTel packages installed
- [ ] `instrumentation.js` created at project root with `instrumentation-http` disabled
- [ ] `app.use(otel())` added before route definitions
- [ ] Start script uses `--import ./instrumentation.js` (ESM) or `--require` (CJS)
- [ ] `TRACEWAY_URL` and `TRACEWAY_TOKEN` env vars set
- [ ] Verified endpoints appear in Traceway dashboard
