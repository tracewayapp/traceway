# Add Traceway to a NestJS Project

Add OpenTelemetry tracing to an existing NestJS project so it reports endpoints, spans, and errors to a Traceway instance.

## Prerequisites

- NestJS application (Express or Fastify adapter)
- A Traceway instance URL and project token (get these from your Traceway dashboard → Connection page)

## Step 1: Install Dependencies

```bash
npm install @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/exporter-metrics-otlp-http \
  @opentelemetry/api
```

If the project uses Prisma, also install its OTel instrumentation:

```bash
npm install @prisma/instrumentation
```

## Step 2: Create the Instrumentation File

Create `instrumentation.ts` at the project root — next to `package.json`, NOT inside `src/`:

```typescript
import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import { Resource } from "@opentelemetry/resources";

const tracewayUrl = process.env.TRACEWAY_URL || "https://your-traceway-instance.com";
const tracewayToken = process.env.TRACEWAY_TOKEN || "your-project-token";

const sdk = new NodeSDK({
  resource: new Resource({
    "service.name": "my-nestjs-app",
    "service.version": "1.0.0",
  }),

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

  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();
```

If using Prisma, add `import { PrismaInstrumentation } from "@prisma/instrumentation"` and include `new PrismaInstrumentation()` in the `instrumentations` array.

### Why NestJS is simpler than Hono/Next.js

- **No `instrumentation-http` disable needed.** NestJS uses Express (CJS) internally, so OTel patches it successfully.
- **No route helper needed.** `@opentelemetry/instrumentation-express` automatically sets `http.route` — endpoint grouping works out of the box.
- **No error handler needed.** Express instrumentation records exceptions automatically.

## Step 3: Update Start Scripts

The instrumentation file must load **before** NestJS boots. Update `package.json`:

```json
{
  "scripts": {
    "start": "node --require ./instrumentation.js dist/main.js",
    "start:dev": "node --require ts-node/register --require ./instrumentation.ts src/main.ts"
  }
}
```

Requires `ts-node` as a dev dependency for development mode:

```bash
npm install -D ts-node
```

If using the NestJS CLI (`nest start`), set via environment variable instead:

```bash
export NODE_OPTIONS="--require ./instrumentation.js"
npm run start
```

## Step 4: Set Environment Variables

```bash
TRACEWAY_URL=https://your-traceway-instance.com
TRACEWAY_TOKEN=your-project-token
```

## Step 5: Instrument Background Tasks (Optional)

NestJS doesn't have built-in scheduled tasks that auto-instrument. If your app uses `@nestjs/schedule` (cron jobs) or `@nestjs/bull` (queues), wrap them in manual spans with `SpanKind.CONSUMER` so they appear as **Tasks** in Traceway:

```typescript
import { trace, SpanKind, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-nestjs-app");

@Injectable()
export class TasksService {
  @Cron("0 * * * *")
  async hourlyCleanup() {
    await tracer.startActiveSpan(
      "hourly-cleanup",
      { kind: SpanKind.CONSUMER },
      async (span) => {
        try {
          await this.cleanupExpiredSessions();
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
}
```

The `SpanKind.CONSUMER` is what tells Traceway to classify this as a **Task** instead of an Endpoint.

## What Gets Traced Automatically

After completing steps 1-4, the following produces traces with zero additional code:

| What | How | Span type in Traceway |
|------|-----|----------------------|
| Every incoming HTTP request | `instrumentation-http` + `instrumentation-express` | **Endpoint** (root SERVER span) |
| Route grouping (`/users/1` + `/users/2` → `/users/:id`) | `instrumentation-express` sets `http.route` | Endpoint attribute |
| Status codes (200, 404, 500) | `instrumentation-http` sets `http.response.status_code` | Endpoint attribute |
| Thrown errors | Express error handling records exceptions | **Issue** |
| NestJS handler spans | `instrumentation-nestjs-core` | **Span** (child) |
| Prisma queries | `@prisma/instrumentation` (if installed) | **Span** (child) |
| Outgoing `fetch()` calls | `instrumentation-undici` (auto) | **Span** (child) |
| Outgoing HTTP (`axios`) | `instrumentation-http` (auto) | **Span** (child) |
| PostgreSQL queries (`pg`) | `instrumentation-pg` (auto) | **Span** (child) |
| MySQL queries (`mysql2`) | `instrumentation-mysql2` (auto) | **Span** (child) |
| MongoDB queries | `instrumentation-mongodb` (auto) | **Span** (child) |
| Redis operations (`ioredis`) | `instrumentation-ioredis` (auto) | **Span** (child) |
| Cron jobs / queue consumers | Manual with `SpanKind.CONSUMER` (step 5) | **Task** |

## Verification

After starting your app, hit a few endpoints and check Traceway:

1. **Endpoints page** — routes should appear grouped by pattern (e.g., `GET /api/users/:id`), not by literal URL
2. **Issues page** — any thrown errors should appear with stack traces
3. **Endpoint detail → Spans tab** — database queries, outgoing fetch calls, and NestJS handler spans should appear as children

## Checklist

- [ ] OTel packages installed (+ `@prisma/instrumentation` if using Prisma)
- [ ] `instrumentation.ts` created at project root
- [ ] Start scripts updated to use `--require ./instrumentation.ts` (dev) or `--require ./instrumentation.js` (prod)
- [ ] `TRACEWAY_URL` and `TRACEWAY_TOKEN` env vars set
- [ ] Cron jobs / queue consumers wrapped with `SpanKind.CONSUMER` spans (if applicable)
- [ ] Verified endpoints appear in Traceway dashboard
