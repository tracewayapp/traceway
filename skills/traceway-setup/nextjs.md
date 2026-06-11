# Add Traceway to a Next.js Project

Add OpenTelemetry tracing to an existing Next.js (App Router) project so it reports endpoints, spans, and errors to a Traceway instance.

## Prerequisites

- Next.js 13.4+ with App Router
- A Traceway instance URL and project token (get these from your Traceway dashboard → Connection page)

## Step 1: Install Dependencies

```bash
npm install @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/exporter-metrics-otlp-http \
  @opentelemetry/api
```

If the project uses Prisma, also install its OTel instrumentation for automatic query tracing:

```bash
npm install @prisma/instrumentation
```

## Step 2: Create the Instrumentation File

Create `instrumentation.ts` at the project root — next to `next.config.js`, NOT inside `src/` or `app/`. Next.js loads this file automatically during server startup. No `--require` or `--import` flag needed.

```typescript
export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    const { NodeSDK } = await import("@opentelemetry/sdk-node");
    const { OTLPTraceExporter } = await import(
      "@opentelemetry/exporter-trace-otlp-http"
    );
    const { OTLPMetricExporter } = await import(
      "@opentelemetry/exporter-metrics-otlp-http"
    );
    const { PeriodicExportingMetricReader } = await import(
      "@opentelemetry/sdk-metrics"
    );
    const { getNodeAutoInstrumentations } = await import(
      "@opentelemetry/auto-instrumentations-node"
    );
    const { Resource } = await import("@opentelemetry/resources");
    const { PrismaInstrumentation } = await import(
      "@prisma/instrumentation"
    );

    const tracewayUrl = process.env.TRACEWAY_URL || "https://your-traceway-instance.com";
    const tracewayToken = process.env.TRACEWAY_TOKEN || "your-project-token";

    const sdk = new NodeSDK({
      resource: new Resource({
        "service.name": "my-nextjs-app",
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

      instrumentations: [
        getNodeAutoInstrumentations(),
        new PrismaInstrumentation(),
      ],
    });

    sdk.start();
  }
}
```

### Key decisions in this file

- **`process.env.NEXT_RUNTIME === "nodejs"` guard is required.** Next.js runs `instrumentation.ts` in both Node.js and Edge runtimes. The OTel Node SDK only works in Node.js.
- **All imports are dynamic `import()`.** This prevents OTel packages from being bundled into the Edge runtime.
- **`PrismaInstrumentation` is included.** If the project doesn't use Prisma, remove the import and the `new PrismaInstrumentation()` line.
- **`getNodeAutoInstrumentations()` is kept with defaults.** Unlike Hono, `instrumentation-http` is NOT disabled — it may capture some spans depending on the Node.js version.

## Step 3: Create the Route Helper

Next.js (App Router) does not use Express or Fastify, so OTel auto-instrumentation cannot set `http.route` automatically. Without it, Traceway creates a separate endpoint for every unique URL path instead of grouping them.

Create `lib/with-route.ts`:

```typescript
import { trace, SpanStatusCode } from "@opentelemetry/api";

type RouteHandler = (
  req: Request,
  context: { params: Promise<Record<string, string>> }
) => Response | Promise<Response>;

export function withRoute(route: string, handler: RouteHandler): RouteHandler {
  return async (req, context) => {
    const span = trace.getActiveSpan();
    if (span) {
      span.setAttribute("http.route", route);
    }
    try {
      return await handler(req, context);
    } catch (error) {
      if (span) {
        span.recordException(error as Error);
        span.setStatus({
          code: SpanStatusCode.ERROR,
          message: (error as Error).message,
        });
      }
      throw error;
    }
  };
}
```

### What `withRoute` does

- Sets `http.route` to the parameterized route pattern so Traceway groups endpoints correctly (e.g., `/api/users/[id]` instead of `/api/users/1`, `/api/users/2`)
- Catches thrown exceptions, records them as span events (appear as Issues in Traceway), then re-throws

## Step 4: Wrap Every API Route Handler

Wrap each exported handler with `withRoute`, passing the route pattern that matches the file path:

```typescript
// app/api/users/route.ts
import { withRoute } from "@/lib/with-route";
import { prisma } from "@/lib/db";

export const GET = withRoute("/api/users", async () => {
  const users = await prisma.user.findMany();
  return Response.json(users);
});

export const POST = withRoute("/api/users", async (req) => {
  const body = await req.json();
  const user = await prisma.user.create({ data: body });
  return Response.json(user, { status: 201 });
});
```

```typescript
// app/api/users/[id]/route.ts
import { withRoute } from "@/lib/with-route";
import { prisma } from "@/lib/db";

export const GET = withRoute("/api/users/[id]", async (req, { params }) => {
  const { id } = await params;
  const user = await prisma.user.findUnique({ where: { id: parseInt(id) } });
  if (!user) {
    return Response.json({ error: "User not found" }, { status: 404 });
  }
  return Response.json(user);
});
```

### Route pattern convention

The `route` string passed to `withRoute` should match the file-system path with Next.js bracket notation:

| File path | Route string |
|---|---|
| `app/api/users/route.ts` | `"/api/users"` |
| `app/api/users/[id]/route.ts` | `"/api/users/[id]"` |
| `app/api/posts/[slug]/comments/route.ts` | `"/api/posts/[slug]/comments"` |

## Step 5: Enable Instrumentation Hook (Next.js 13.4–14.x only)

For Next.js versions before 15, add to `next.config.js`:

```javascript
/** @type {import('next').NextConfig} */
const nextConfig = {
  experimental: {
    instrumentationHook: true,
  },
};

module.exports = nextConfig;
```

Not needed for Next.js 15+.

## Step 6: Set Environment Variables

```bash
TRACEWAY_URL=https://your-traceway-instance.com
TRACEWAY_TOKEN=your-project-token
```

## What Gets Traced Automatically

After completing steps 1-6, the following produces traces with zero additional code:

| What | How | Span type in Traceway |
|------|-----|----------------------|
| Every incoming HTTP request | `withRoute` sets `http.route` | **Endpoint** (root SERVER span) |
| Route grouping (`/users/1` + `/users/2` → `/users/[id]`) | `withRoute` sets `http.route` | Endpoint attribute |
| Thrown errors | `withRoute` calls `span.recordException()` | **Issue** |
| Prisma queries | `@prisma/instrumentation` (auto) | **Span** (child) |
| Outgoing `fetch()` calls | `instrumentation-undici` (auto) | **Span** (child) |
| PostgreSQL queries (`pg`) | `instrumentation-pg` (auto) | **Span** (child) |
| MySQL queries (`mysql2`) | `instrumentation-mysql2` (auto) | **Span** (child) |
| MongoDB queries | `instrumentation-mongodb` (auto) | **Span** (child) |
| Redis operations (`ioredis`) | `instrumentation-ioredis` (auto) | **Span** (child) |

## Instrumenting Background Tasks (Optional)

Next.js has no built-in scheduler. If your app runs cron jobs or queue consumers (via `node-cron`, `bullmq`, or triggered by an external scheduler like Vercel Cron), wrap them with `SpanKind.CONSUMER` so they appear as **Tasks** in Traceway:

```typescript
import { trace, SpanKind, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-nextjs-app");

// e.g., in an API route triggered by a cron scheduler
export const POST = withRoute("/api/cron/cleanup", async () => {
  return tracer.startActiveSpan(
    "cleanup-expired-sessions",
    { kind: SpanKind.CONSUMER },
    async (span) => {
      try {
        await prisma.session.deleteMany({
          where: { expiresAt: { lt: new Date() } },
        });
        span.setStatus({ code: SpanStatusCode.OK });
        span.end();
        return Response.json({ status: "ok" });
      } catch (error) {
        span.recordException(error as Error);
        span.setStatus({ code: SpanStatusCode.ERROR, message: (error as Error).message });
        span.end();
        throw error;
      }
    }
  );
});
```

Without `SpanKind.CONSUMER`, background work would be classified as an Endpoint or dropped.

## Adding Manual Spans (Optional)

For operations without auto-instrumentation (SQLite, Server Components, custom business logic):

```typescript
import { trace, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-nextjs-app");

// In an API route handler
export const GET = withRoute("/api/report", async () => {
  return tracer.startActiveSpan("generate-report", async (span) => {
    try {
      span.setAttribute("report.type", "monthly");
      const data = await generateReport();
      span.end();
      return Response.json(data);
    } catch (error) {
      span.recordException(error as Error);
      span.setStatus({ code: SpanStatusCode.ERROR, message: (error as Error).message });
      span.end();
      throw error;
    }
  });
});

// In a Server Component
export default async function DashboardPage() {
  return tracer.startActiveSpan("DashboardPage.render", async (span) => {
    try {
      const stats = await fetchStats();
      span.setAttribute("stats.count", stats.length);
      return <Dashboard stats={stats} />;
    } finally {
      span.end();
    }
  });
}
```

## Recording Caught Exceptions (Optional)

`withRoute` auto-records thrown errors. For errors you catch and handle:

```typescript
import { trace, SpanStatusCode } from "@opentelemetry/api";

export const POST = withRoute("/api/checkout", async (req) => {
  const span = trace.getActiveSpan();
  try {
    await processPayment(await req.json());
  } catch (error) {
    if (span) {
      span.recordException(error as Error);
      span.setStatus({ code: SpanStatusCode.ERROR, message: (error as Error).message });
    }
    return Response.json({ error: "Payment failed" }, { status: 500 });
  }
  return Response.json({ status: "ok" });
});
```

## Verification

After starting your app, hit a few endpoints and check Traceway:

1. **Endpoints page** — routes should appear grouped by pattern (e.g., `GET /api/users/[id]`), not by literal URL
2. **Issues page** — any thrown errors should appear with stack traces
3. **Endpoint detail → Spans tab** — Prisma queries, outgoing fetch calls, and manual spans should appear as children of the HTTP request

## Checklist

- [ ] OTel packages installed (+ `@prisma/instrumentation` if using Prisma)
- [ ] `instrumentation.ts` created at project root with `register()` function and runtime guard
- [ ] `lib/with-route.ts` created
- [ ] Every API route handler wrapped with `withRoute("/api/path/[param]", handler)`
- [ ] `experimental.instrumentationHook: true` added (Next.js 13.4-14.x only)
- [ ] `TRACEWAY_URL` and `TRACEWAY_TOKEN` env vars set
- [ ] Verified endpoints appear in Traceway dashboard
