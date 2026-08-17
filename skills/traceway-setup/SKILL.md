---
name: traceway-setup
description: Analyze a repository's backend, browser, mobile, AI, background-work, existing-observability, build, and deployment architecture; propose and explain the correct Traceway project topology; guide the user through creating projects and credentials in the Traceway dashboard; then instrument and verify every selected component. Every backend uses OpenTelemetry over OTLP/HTTP regardless of language or framework, keeping endpoints, tasks, AI traces, logs, application metrics, and host metrics in one backend project. Browser frontends and independently released mobile apps use separate Traceway projects with the Traceway SDKs, plus source map or symbol uploads where applicable. Use when the user wants to plan, add, migrate, or complete Traceway or OpenTelemetry monitoring for a backend, frontend, full-stack, mobile, iOS/Swift, or AI agent/chatbot repository. Accepts a project token and instance URL in the invocation, e.g. "/traceway-setup with token abc123 and url https://traceway.example.com".
---

# Set Up Traceway

Analyze the application first, agree on its Traceway project structure with the user, help them create that structure in the dashboard, and only then integrate and verify it.

## Operating Contract

- Inspect before asking for credentials or changing code.
- Explain what was detected, what is already tracked, and how each production component should report to Traceway.
- Present a proposed project map and wait for the user to confirm it before implementation.
- Never print token values found in files or the environment. Report only that a value exists and the variable name that carries it.
- Do not require the user to paste secrets into chat. Implement against environment variables and let the user set real values in their local, deployment, or CI secret store. Real values are required only for live verification.
- Never commit tokens or filled connection strings.

**Every backend integrates with OpenTelemetry.** There is one backend path, not one per language or web framework: Go, Node, Python, PHP, Java, .NET, Ruby, and everything else export OTLP/HTTP to `<instance>/api/otel/*`. The Traceway project is created with framework **OpenTelemetry**, which is the only backend option in the dashboard's framework picker. The native Traceway Go SDK is a deliberate exception used only when the user explicitly asks for it.

## Fast Path

When all of the following hold, skip Steps 2 and 3 and go straight to the integration steps:

- the repository has exactly one deployable component, and
- the user supplied (or the environment already carries) an instance URL and a project token, and
- the token's project already matches that component.

Still run Step 1, because it decides *how* to instrument. Collapse "Propose and Confirm the Project Map" to a single confirmation line ("This is a Go API; it reports to your existing `<name>` project over OpenTelemetry") and skip the dashboard walkthrough entirely. The full map-and-dashboard ceremony exists for repositories with more than one deployable component, or where no project exists yet. Do not make a user who handed you a token sit through a project-map review for a single service.

## Step 1: Analyze the Architecture

Before changing anything, build a picture of what is deployed and what is already instrumented:

1. **Frameworks and languages**: detect them from `package.json`, `go.mod`, `composer.json`, `requirements.txt`/`pyproject.toml`, `pubspec.yaml`, `build.gradle`(`.kts`), `Package.swift`, `*.xcodeproj`/`*.xcworkspace`, `Podfile`, and source extensions. For iOS/Apple targets, note whether the sources are Swift or Objective-C. That choice picks the path in "Frontend and Mobile" (Swift gets the native SDK; Objective-C-only has none and falls back to OTel).
2. **Deployable components and entry points**: inventory APIs, SSR servers, browser apps, workers, schedulers, CLIs, and mobile applications. Treat code organization as evidence, not proof that something is deployed.
3. **Existing observability**: find OpenTelemetry configuration, Traceway SDKs, Sentry, Datadog, New Relic, Honeycomb, logging exporters, tracing middleware, source map or symbol upload steps, and observability environment-variable names. Explain whether Traceway will replace, coexist with, or extend each integration. Never display discovered credential values.
4. **Production role of JS meta-frameworks** (Next.js, SvelteKit, Remix, Nuxt): never assume full-stack.
   - **Frontend-only signals**: `output: 'export'` in `next.config.*` (static export, so there is no server in production); no API routes (`app/api/**/route.*`, `pages/api/*`) or only trivial ones; no `'use server'` server actions; `rewrites`/proxy config or a `NEXT_PUBLIC_API_URL`-style env var pointing the browser at an external API; a separate backend service in this repo, another repo, or another language; static hosting in the deploy config (S3/CloudFront, GitHub Pages, nginx serving `out/`).
   - **Server signals**: API routes with real logic, `'use server'` server actions, database clients imported in server code, SSR reading its own data layer, `next start` or standalone output in the deploy config.
   - **Mixed or unclear**: ask how the application is deployed and whether the framework server does production work worth tracking.

   Frontend-only means integrate ONLY the browser side with the frontend SDK; do NOT add OTel to, or otherwise instrument, the framework's server side. A separate backend is its own component with its own Traceway project.
5. **Background work**: find cron jobs, queue consumers, schedulers, CLI commands, and long-running workers. Record whether libraries already emit `CONSUMER` spans.
6. **AI/LLM usage**: check dependencies for `openai`, `@anthropic-ai/sdk`, `anthropic`, `langchain` / `@langchain/*`, `ai` (Vercel AI SDK), `litellm`, `google-generativeai`, `cohere`, `openrouter`, and agent frameworks (`langgraph`, `crewai`, `autogen` / `ag2`, `pydantic-ai`, `@openai/agents`, `@mastra/*`, `llamaindex` / `llama-index`, `semantic-kernel`). Then decide whether this is a **conversational product** (chatbot, assistant, agent) rather than one-shot LLM calls: look for chat or streaming-chat routes (`/chat`, `/completions`, SSE or websocket handlers feeding an LLM), message-history persistence (a `messages`/`conversations` table or thread ids), tool/function-calling definitions passed to the model, or any agent framework above. If conversational or tool-calling, read `ai-agent.md` in this skill directory before proposing the AI integration.
7. **Browser build and release flow**: identify the bundler, output directory, source-map settings, deploy pipeline, and existing artifact uploads.
8. **Mobile build and release flow**: determine whether Flutter is obfuscated, Android uses R8/minification, iOS produces dSYMs, and whether multiple platform directories are one cross-platform product or independently released apps.
9. **Deployment and host metrics**: inspect Dockerfiles, Compose, Kubernetes, Helm, Terraform, Ansible, cloud-init, PaaS configs, and CI/CD. Determine whether the backend runs on a host where the Traceway OTel Agent is applicable.

Present the findings before asking questions:

| Component | Repository evidence | Production role | Current tracking | Proposed Traceway project | Integration | Credentials needed |
|---|---|---|---|---|---|---|
| `apps/api` | Go module + Gin routes | Backend API | OTel / none / vendor | `Product Backend` | OTel | Runtime token |
| `apps/web` | Svelte + Vite | Browser app | Existing SDK / none | `Product Web` | Traceway Svelte SDK | Runtime + upload token |

Use actual paths and findings. Include unresolved deployment assumptions explicitly.

## Step 2: Propose and Confirm the Project Map

Explain these default boundaries:

| Application boundary | Traceway project | What belongs in it |
|---|---|---|
| **Backend system** | One project with framework **OpenTelemetry** | API endpoints, child spans, issues, background tasks, AI traces, logs, application/runtime metrics, and host metrics. APIs, workers, schedulers, and the host agent share the project token; distinguish them with stable `service.name` values. |
| **Browser frontend** | One separate project per deployed browser application, using **React**, **Svelte**, **Vue.js**, or **jQuery** | Browser errors, web vitals, session replay, distributed-trace linkage, source maps, and browser release metadata. |
| **Mobile app** | One separate project per independently released application, using **Flutter**, **React Native**, **Android**, or **iOS** | Mobile errors/crashes, replay where supported, and build symbols or mappings. A single Flutter product targeting Android and iOS normally uses one project; separate native apps use separate projects. |
| **Full-stack JS app** | Two projects when its server runs in production | Server-side work goes to the OpenTelemetry backend project; browser-side work goes to the browser project. |

The dashboard's framework picker offers exactly these nine options: OpenTelemetry for every backend, React / Svelte / Vue.js / jQuery for browsers, and Flutter / React Native / Android / iOS for mobile. There is no Gin, Django, Laravel, Next.js, or Remix entry, and none is needed. A Next.js or Remix browser project selects **React**, and its server side selects **OpenTelemetry** like any other backend. The framework-specific setup for a backend lives on the project's Connection page, which asks for the language and web framework after the project exists.

Do not create one project per backend process by default. Keeping the API, workers, AI calls, and server metrics together preserves their operational context and distributed traces. Split backend projects only for a real product, ownership, access-control, compliance, or data-isolation boundary.

Ask the user to confirm only what the repository cannot establish safely:

1. Which detected components are deployed and should be tracked?
2. Traceway Cloud or self-hosted, and which organization should own the projects?
3. Are the proposed names and project boundaries correct?
4. Does the backend run on a VM/host, Kubernetes, or serverless/PaaS, and should host metrics be collected?
5. Do mobile directories represent one cross-platform product or separate released apps?

Do not modify code until the user confirms this map.

## Step 3: Guide Dashboard Project Creation

Read `dashboard-project-setup.md` and give the user a tailored, ordered checklist containing exactly the projects from the confirmed map. Explain why each project exists, which framework to select, which signals it will receive, and which credentials it needs. Include the relevant Traceway documentation links.

For every credential, define an environment variable rather than asking the user to paste the value. Runtime credentials take component-specific names (`TRACEWAY_BACKEND_TOKEN`, `PUBLIC_TRACEWAY_WEB_CONNECTION_STRING`, `TRACEWAY_MOBILE_CONNECTION_STRING`), respecting the framework's public-environment prefix rules.

**Upload tokens are different: the uploaders read fixed variable names.** `traceway-sourcemaps` reads `TRACEWAY_SOURCEMAP_TOKEN`, and `dart run traceway:upload_symbols` and the iOS dSYM script read `TRACEWAY_UPLOAD_TOKEN` (all three also read `TRACEWAY_URL`). Either name the CI secret exactly that, or keep a component-specific secret and pass it explicitly:

```bash
traceway-sourcemaps --url "$TRACEWAY_URL" --token "$TRACEWAY_WEB_UPLOAD_TOKEN" --directory ./dist
```

Inventing a name like `TRACEWAY_WEB_UPLOAD_TOKEN` and then calling the uploader with no `--token` fails at release time, not at setup time, so make the choice explicit in the CI step.

Proceed when the user confirms the required projects exist and the variables will be available. Existing correctly mapped Traceway projects may be reused.

## Integration Paths

Pick the path by project type, and pick the project type from the "Analyze the Architecture" findings, never from the framework name alone (a Next.js repo can be a full-stack app or just the frontend of a separate backend). Per path, this is not negotiable per framework; it is how Traceway is designed to receive data:

| Project type | Path |
|---|---|
| **Backend** (any language) | OpenTelemetry, exporting OTLP/HTTP to `<instance>/api/otel/v1/*`. Always, including Go. |
| **Frontend** (browser SPA, or a JS meta-framework running frontend-only in production) | Traceway `@tracewayapp/<framework>` SDK + bundler plugin + source map upload (see "Frontend and Mobile" below). |
| **Full-stack JS** (Next.js, SvelteKit, Remix, actually serving its API/SSR in production per "Analyze the Architecture") | BOTH sides, each under its own Traceway project: server side via OpenTelemetry AND browser side via the frontend SDK. |
| **Mobile** (Flutter, React Native, Android, native Swift iOS) | The Traceway platform SDK. Never OTel. Sole exception: a non-Swift iOS/Apple app has no native SDK, so it uses an OTel library (e.g. Honeycomb) exporting to Traceway like a backend (see "Frontend and Mobile"). |

The rules every backend integration must satisfy, and the table of what each span becomes, are in Step 4 where they are applied.

## Step 4: Backend OTel Setup

The same shape in every language:

1. Install the language's OpenTelemetry SDK plus the auto-instrumentation for the web framework and database clients.
2. Point the OTLP/HTTP exporter at Traceway with the project token as a Bearer header.
3. Set `service.name` (becomes the Server Name in Traceway) and `service.version` (enables release comparison) on the resource.
4. Verify endpoint grouping before anything else.

### Three rules that are not negotiable

1. **Endpoints MUST arrive parametrized.** `http.route` must be the route pattern (`/api/users/:id`), never the concrete URL. Traceway uses the value as-is, but only when it starts with `/`: a route *name* like `app_user_show` is discarded exactly like a missing value. With no usable route it falls back to `url.path`, and the Endpoints page explodes into one row per unique URL.
2. **Background work MUST use `SpanKind.CONSUMER`.** A root span with the default `INTERNAL` kind and no HTTP attributes is silently dropped (exceptions recorded on it still reach Issues, but the task run itself is lost).
3. **The exporter MUST use an OpenTelemetry project's token.** If OTLP arrives with a browser project's token (framework React, Svelte, Vue.js, jQuery, or React Native), Traceway keeps only the Issues and discards every endpoint, task, child span, and AI trace. The symptom is "errors appear but the Endpoints page is empty", and the cause is almost always two projects whose tokens got swapped.

### How Traceway classifies spans

The rules are evaluated in this order and the first match wins.

| # | Span | Condition | Becomes |
|---|---|---|---|
| 1 | Any span | `SpanKind = INTERNAL` **and** carries `exception.*` attributes | **Issue.** Never promoted, even with HTTP or `gen_ai.*` attributes present. A child span keeps its ordinary Span row, a root span gets only the Issue |
| 2 | Root span, or a span whose parent is not in the same export batch | `SpanKind = SERVER` or `INTERNAL` with HTTP attributes | **Endpoint** |
| 3 | Any span | `SpanKind = CONSUMER` | **Task** |
| 4 | Root span | `SpanKind = INTERNAL` with a `console.command` attribute | **Task** (CLI command) |
| 5 | Any span | Has any `gen_ai.*` attribute | **AI Trace** |
| 6 | Non-root span | Has a parent span id | **Span** (child) |
| 7 | Root span | Nothing above matched | **Dropped** (exceptions recorded on it still become Issues, unlinked) |

Because the order is fixed, a `CONSUMER` span carrying `gen_ai.*` is a Task, and a `SERVER` request span carrying `gen_ai.*` is an Endpoint. Put the model call on its own child span (Step 6).

Two more rules that do not depend on span kind:

- An `exception` event, or `exception.*` attributes, on any span produces an **Issue**, whatever the span itself became.
- An endpoint that returns **404** with no real route matched (`http.route` missing, or a catch-all like `/`, `/*`) is renamed to the literal endpoint `UNMATCHED`, so bot scans and typo'd URLs collapse into one row. A concrete matched route returning 404 keeps its own name.

For the exact rules, endpoint naming, metric conversion, and the remaining quirks, read `data-model.md` in this skill directory. It is the authoritative reference.

### Exporter configuration

Where the SDK supports the standard env vars, prefer them; they work identically across languages:

```bash
OTEL_SERVICE_NAME=my-service
OTEL_RESOURCE_ATTRIBUTES=service.version=1.2.3   # there is no OTEL_SERVICE_VERSION variable
OTEL_EXPORTER_OTLP_ENDPOINT=https://<instance>/api/otel
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf        # required, several SDKs default to gRPC
OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer <project-token>"
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
```

Two of those lines are load-bearing in a way that fails silently:

- **`OTEL_EXPORTER_OTLP_PROTOCOL` is not optional.** Left unset, the Python SDK and the Java agent resolve `otlp` to **gRPC**, and Traceway has no gRPC listener. The app starts, serves traffic, exits 0, prints no warning, and every export is lost. (PHP is the one exception to the value: it needs `http/json`.)
- **There is no `OTEL_SERVICE_VERSION`.** `service.version` only reaches Traceway through `OTEL_RESOURCE_ATTRIBUTES`. Without it, every endpoint row comes back with an empty App Version and release comparison does nothing.

SDKs append `/v1/traces`, `/v1/metrics`, `/v1/logs` to the endpoint automatically, so set the **base** URL only. A full signal path in `OTEL_EXPORTER_OTLP_ENDPOINT` produces `/v1/traces/v1/traces`, and the signal-specific variables (`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`) are used verbatim with nothing appended. When configuring in code instead, the full URLs are `https://<instance>/api/otel/v1/traces` (and `/v1/metrics`, `/v1/logs`) with header `Authorization: Bearer <project-token>`.

Constraints: OTLP/HTTP only (protobuf or JSON). OTLP/gRPC is not supported, there is no listener on port 4317. `Content-Encoding: gzip` is fine. The body is read up to 10 MB and the rest is truncated, so an oversized batch answers `400 failed to unmarshal` rather than `413`. A wrong or missing token answers `401`, and any other path answers `404`. All three are invisible from the application: most SDKs log exporter failures at debug level only, so when nothing arrives, turn the SDK's own diagnostic logging on first.

### Node.js example

```bash
npm install @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node \
  @opentelemetry/instrumentation @opentelemetry/api @opentelemetry/sdk-metrics \
  @opentelemetry/exporter-trace-otlp-http @opentelemetry/exporter-metrics-otlp-http
```

Declare `@opentelemetry/api` yourself even though it also arrives transitively. Relying on hoisting breaks under pnpm's strict `node_modules` and under Yarn PnP.

Create `instrumentation.mjs` at the project root and load it before the app: `node --import ./instrumentation.mjs server.js`. Use the `.mjs` extension. A `.js` file holding `import` syntax fails to load in a CommonJS project, which is what most projects still are.

```javascript
import { register } from "node:module";
register("@opentelemetry/instrumentation/hook.mjs", import.meta.url);

import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";

const url = process.env.TRACEWAY_URL;
const headers = { Authorization: `Bearer ${process.env.TRACEWAY_BACKEND_TOKEN}` };

const sdk = new NodeSDK({
  serviceName: process.env.OTEL_SERVICE_NAME ?? "my-service",
  traceExporter: new OTLPTraceExporter({ url: `${url}/api/otel/v1/traces`, headers }),
  metricReaders: [
    new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter({ url: `${url}/api/otel/v1/metrics`, headers }),
      exportIntervalMillis: 30_000,
    }),
  ],
  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();
```

Three parts of that snippet are load-bearing:

- **The two `register` lines install the ESM loader hook.** Without them an ESM app (`"type": "module"`, `import express from "express"`) still gets HTTP spans but no `http.route`, so every URL becomes its own endpoint row. A CommonJS app (`require("express")`) is patched without the hook, and the lines are harmless there, so keep them either way.
- **`serviceName`** becomes the Server Name in Traceway. Leave it out and every span reports `unknown_service:node`. For release comparison also set `OTEL_RESOURCE_ATTRIBUTES=service.version=1.2.3`.
- **`metricReaders` is a list.** The older singular `metricReader` option is deprecated.

Auto-instrumentation covers Express routes (sets `http.route`), status codes, errors, and database clients (`pg`, `mysql2`, `mongodb`, `ioredis`). SQLite and custom business logic need manual `tracer.startActiveSpan()` child spans.

### Per-language notes

- **Go**: use the framework's OTel middleware. `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin`, `github.com/riandyrn/otelchi` (a community package, and pass `otelchi.WithChiRoutes(r)` or it reports raw URLs), `github.com/gofiber/contrib/otelfiber/v2`. All three set `http.route` from the matched route pattern. Exporter: `otlptracehttp.WithEndpointURL(...)` + `WithHeaders`. Two Go-only traps:
  - **Set the propagator.** Go's global propagator is a no-op by default, so `traceparent` is never sent or read and cross-service traces never link. One line, right after `otel.SetTracerProvider(tp)`:

    ```go
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{}, propagation.Baggage{},
    ))
    ```

  - **stdlib `net/http` does not report its route.** `otelhttp` reads the route from `r.Pattern`, which `ServeMux` fills in only after it has matched, so the usual top-level `otelhttp.NewHandler(mux, "server")` records no `http.route` at all and every URL becomes its own endpoint row. Either wrap each registered handler, which puts the span start inside the mux:

    ```go
    mux.Handle("GET /api/users/{id}", otelhttp.NewHandler(usersHandler, "users"))
    ```

    or keep the single top-level wrap and stamp the route yourself:

    ```go
    // imports: strings, go.opentelemetry.io/otel/trace,
    //          semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
    func withRoute(mux *http.ServeMux) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if _, pattern := mux.Handler(r); pattern != "" {
                if i := strings.IndexByte(pattern, '/'); i >= 0 {
                    route := pattern[i:] // drop the "GET " prefix, http.route must start with "/"
                    trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
                }
            }
            mux.ServeHTTP(w, r)
        })
    }

    handler := otelhttp.NewHandler(withRoute(mux), "server")
    ```

- **Python**: `pip install opentelemetry-distro opentelemetry-exporter-otlp`, then `opentelemetry-bootstrap -a install`, then run the app under `opentelemetry-instrument` with the env vars above (`opentelemetry-instrument uvicorn app:app`, `opentelemetry-instrument gunicorn wsgi:app`, `opentelemetry-instrument python worker.py`). Starting the server without that prefix sends nothing. FastAPI and Flask instrumentation sets `http.route` correctly with zero application code, in the framework's own syntax (`GET /users/{user_id}`, `GET /orders/<order_id>`), and both frameworks already answer `500` on an unhandled exception, so the endpoint status and the Issue line up without extra work. Three Python-only gotchas:
  - **Logs need two more variables.** `OTEL_LOGS_EXPORTER=otlp` attaches the OTel handler to the **root** logger, whose level defaults to `WARNING`, so every `logger.info(...)` is filtered out before the bridge sees it and only WARN and above reach Traceway. Attaching the handler also replaces the root handler list, so the app's own records stop appearing on stdout. Add both:

    ```bash
    OTEL_PYTHON_LOG_CORRELATION=true   # restores console output, stamps otelTraceID/otelSpanID
    OTEL_PYTHON_LOG_LEVEL=info         # only read alongside the line above
    ```

    `OTEL_PYTHON_LOG_LEVEL` on its own does nothing. Do **not** set `OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true`; on current versions the bridge is already on and that variable switches it to the SDK's deprecated handler.
  - **Metrics take a minute.** The SDK default for `OTEL_METRIC_EXPORT_INTERVAL` is 60000 ms, so a correct pipeline looks empty while you are checking it. Set `OTEL_METRIC_EXPORT_INTERVAL=10000` during verification.
  - **Django needs two extra things**: export `DJANGO_SETTINGS_MODULE` or `opentelemetry-instrument` fails to start Django at all, and add the route middleware from the Django guide, because `opentelemetry-instrumentation-django` reports `http.route` as Django's own pattern (`api/users/<int:user_id>/`), which has no leading `/` and is therefore discarded.

  Guides: https://docs.tracewayapp.com/client/otel/python and https://docs.tracewayapp.com/client/otel/django
- **PHP**: Laravel via `composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter`; Symfony via `composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter` (the stock Symfony auto-instrumentation sets `http.route` to the route NAME, which Traceway discards; see `data-model.md`). PHP does not take `OTEL_PHP_AUTOLOAD_ENABLED` as a plain env var next to the block above. Laravel needs nothing extra, the keepsuit service provider starts the SDK. Symfony starts the SDK through `open_telemetry.sdk.autoload_enabled: true` in `config/packages/open_telemetry.yaml`, or through a real process environment variable set in php-fpm, the Dockerfile, or Apache. Never put `OTEL_PHP_AUTOLOAD_ENABLED` in `.env`: Dotenv reads it after Composer autoload, the bundle then skips starting the SDK, and every signal silently becomes a no-op. PHP also prefers `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`, though `http/protobuf` works too and is only slower without `ext-protobuf`.
- **Java / .NET / anything else**: the standard OTel agent or SDK with the env vars above works as-is.
- Full per-framework docs: https://docs.tracewayapp.com/client/otel

### Verify endpoint grouping (do this first)

Hit a parametrized route a few times with different IDs and check the Traceway Endpoints page: you must see ONE row (`GET /api/users/:id`), not one row per ID. If you see raw IDs, the instrumentation is not setting `http.route`; fix that before continuing. Last resort, set it manually in a middleware that knows the matched route pattern (the value must start with `/`, or it is discarded):

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

**Return HTTP 500 when an exception happens.** A request that throws or panics must respond with `500` and set the span status to `ERROR`. Never let it fall through to a `200`/`2xx`. The common trap is a recover/panic middleware that reports the exception but lets the response writer keep its default `200`: Traceway then records the endpoint transaction as a *success*, so the Endpoints page looks healthy while Issues fills up, and the exception becomes an unlinked island instead of correlating to a failed request (and, with distributed tracing, to the frontend call that triggered it). Always pair "an exception was recorded" with "respond `500` and set span status `ERROR`". Genuine client errors that are NOT bugs (validation `422`, auth `401`, not-found `404`) keep their real 4xx status and should not be recorded as exceptions in the first place.

Go has a sharper version of the same trap. `otelgin` records the status only after `c.Next()` returns, which never happens during a panic unwind, so an unrecovered panic leaves the endpoint row at status **0** even though Gin's own recovery already returned 500 to the client, and the Issue arrives with a title but no stack trace. Register a recovery middleware **after** `otelgin.Middleware` so it runs inside the server span:

```go
// imports: fmt, net/http, go.opentelemetry.io/otel/codes, go.opentelemetry.io/otel/trace
func otelRecovery() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if r := recover(); r != nil {
                err, ok := r.(error)
                if !ok {
                    err = fmt.Errorf("%v", r)
                }
                span := trace.SpanFromContext(c.Request.Context())
                span.RecordError(err, trace.WithStackTrace(true))
                span.SetStatus(codes.Error, err.Error())
                c.AbortWithStatus(http.StatusInternalServerError)
            }
        }()
        c.Next()
    }
}

router.Use(otelgin.Middleware(serviceName))
router.Use(otelRecovery())
```

### Logs

`OTEL_LOGS_EXPORTER=otlp` wires the exporter only. The application's own log calls still need a bridge, or the Logs page stays empty while traces and metrics look perfectly healthy.

- **Node**: add `@opentelemetry/winston-transport`, or the Pino or Bunyan instrumentation, so each record picks up `trace_id` and `span_id`.
- **Python, PHP, Java, .NET**: `opentelemetry-instrument` and the language agents bridge the standard logger automatically.
- **Go**: build an `sdklog.LoggerProvider` with `otlploghttp.WithEndpointURL(...)`, register it with `global.SetLoggerProvider(lp)`, and `defer lp.Shutdown(ctx)` or the last batch is lost on exit.

Emit from the **request** context (`c.Request.Context()` in Gin, `r.Context()` in net/http, the active context in Node). A line emitted from a background context carries no trace id and cannot be opened from the endpoint it belongs to. Traceway reads `severity_text` (or derives it from `severity_number`), the body, `service.name`, and the trace and span ids.

### Go uses OpenTelemetry

There is no separate Traceway Go SDK. It was retired, and Go instruments with `opentelemetry-go` exactly like every other backend: `go get go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`, then the same env vars as the block above. Host CPU and memory come from the Traceway OTel Agent (see "Deployment and Server Metrics"), not from the app.

## Step 5: Background Tasks (Boundaries and Labeling)

If "Analyze the Architecture" found background work, instrument it as Tasks. The rules:

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
        const err = error instanceof Error ? error : new Error(String(error));
        span.recordException(err);
        span.setStatus({ code: SpanStatusCode.ERROR, message: err.message });
        throw error;
      } finally {
        span.end();
      }
    }
  );
}
```

(Go: `tracer.Start(ctx, "cleanup-expired-sessions", trace.WithSpanKind(trace.SpanKindConsumer))`.)

## Step 6: AI Traces

If "Analyze the Architecture" found AI/LLM dependencies, instrument the model calls. A span becomes an **AI Trace** when it carries at least one `gen_ai.*` attribute *and* matched none of the earlier classification rules. In practice that means: keep `gen_ai.*` off the HTTP route span (it becomes an Endpoint), keep it off `CONSUMER` spans (they become Tasks), and give the model call its own child span with `SpanKind.CLIENT` rather than `INTERNAL`, so a failed call that records `exception.*` attributes still produces its AI Trace. Since calls happen inside a request or task, that child span stays linked to its Endpoint or Task by trace ID.

Boundaries: **one span per model call** (one provider API request = one AI Trace row). A multi-step agent run is multiple spans sharing a stable `trace.name` (same labeling discipline as task names: no IDs or timestamps). For streaming, end the span when the stream finishes.

**Conversational product?** If "Analyze the Architecture" flagged a chatbot, assistant, or tool-calling agent, follow `ai-agent.md` in this skill directory instead of stopping at the table below. It covers `gen_ai.conversation.id` (multi-turn grouping), what value belongs in `user.id` (a stable end-customer id, never a session id), tool-call payload shapes, and sub-agents. Without those the Users tab stays empty, and the Conversations tab fills with one throwaway single-turn conversation per request, because Traceway falls back to the OTel trace id. That is worse than empty: it looks like the feature is working.

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
| `gen_ai.conversation.id` | Multi-turn conversation grouping (falls back to `session.id`, then the OTel trace id) |
| `user.id` | End-user attribution: a stable customer id (account id, tenant id, or email), the same value across all of that user's conversations |
| `gen_ai.response.finish_reason` | Why generation stopped |
| `gen_ai.prompt` / `gen_ai.completion` | Conversation content, shown on the trace detail page (skip if content must not leave the app) |

Conversation content is also read from `trace.input`/`trace.output` (or `span.input`/`span.output`) when `gen_ai.prompt`/`gen_ai.completion` are absent, and missing `total_tokens`/`total_cost` are computed from the input + output values. Token counts must be integer attributes and costs must be double or integer attributes. A number sent as a string is silently dropped and stored as 0, with no error on the export.

```typescript
import { SpanKind } from "@opentelemetry/api";

return tracer.startActiveSpan("chat-completion", { kind: SpanKind.CLIENT }, async (span) => {
  const response = await openai.chat.completions.create({ model: "gpt-4o", messages });
  span.setAttributes({
    "gen_ai.system": "openai",
    "gen_ai.request.model": "gpt-4o",
    "gen_ai.usage.input_tokens": response.usage?.prompt_tokens ?? 0,
    "gen_ai.usage.output_tokens": response.usage?.completion_tokens ?? 0,
    "trace.name": "support-agent",
  });
  span.end();
  return response;
});
```

Zero-code path for OpenRouter users: in OpenRouter Settings -> Observability, add an OpenTelemetry Collector destination pointing at `https://<instance>/api/otel/v1/traces` with header `{"Authorization": "Bearer <project-token>"}`. Docs: https://docs.tracewayapp.com/client/openrouter

## Step 7: Frontend and Mobile

Frontend and mobile projects do NOT use OTel; they use the Traceway SDKs reporting to `/api/report` with connection string `<project-token>@https://<instance>/api/report`.

**Browser** (React / Vue / Svelte / jQuery / plain JS), three pieces, all expected:

1. **SDK**: `npm install @tracewayapp/react` (or `vue`, `svelte`, `jquery`, `frontend` for plain JS) and initialize with the connection string (React: wrap the app in `<TracewayProvider connectionString="...">`). Captures errors, web vitals, and session replay.
2. **Bundler plugin**: `npm install -D @tracewayapp/bundler-plugin`, then add `tracewayDebugIds()` from `@tracewayapp/bundler-plugin/vite` (or `/rollup`, or `TracewayDebugIdsWebpackPlugin` from `/webpack`) to the bundler config, with source maps enabled (`build.sourcemap: true` / `devtool: "source-map"`).
3. **Source map upload**: `npm install -D @tracewayapp/sourcemap-upload`, then run `traceway-sourcemaps --url <instance> --token <source-map-upload-token> --directory ./dist` as a postbuild or CI step (env vars: `TRACEWAY_URL`, `TRACEWAY_SOURCEMAP_TOKEN`). The upload token comes from the browser project and is a CI secret, never committed.

For the per-framework init code (plain JS, React, Vue, Svelte/SvelteKit, jQuery), the shared SDK options, error filtering, custom attributes, distributed tracing, and the full debug-ID + source map pipeline, read `frontend-js.md` in this skill directory. Online docs: https://docs.tracewayapp.com/client/react (or `vue`, `svelte`, `jquery`, `js-sdk`).

**Full-stack JS** (Next.js, SvelteKit, Remix): only when "Analyze the Architecture" confirmed the framework's server actually serves the app in production. A frontend-only deployment gets just the browser pieces above; a separate backend follows "Backend OTel Setup". When it is genuinely full-stack, integrate both sides under the two confirmed projects: server side follows "Backend OTel Setup" with the backend project's token, and browser side follows the three pieces above with the frontend project's token.

**Mobile**, always the platform SDK, never OTel:

- **Flutter**: `flutter pub add traceway`, then `Traceway.run(connectionString: '<token>@https://<instance>/api/report', child: MyApp())`. Then check whether the release build is obfuscated (`--obfuscate --split-debug-info`): if it is, production crash stack traces arrive obfuscated and stay unreadable until the build's `.symbols` files are uploaded, so use the mobile project's upload token and wire up the symbol upload. For options, platform permissions, the navigator observer, screen recording, privacy masking, the obfuscation check and symbol upload, and the Flutter web caveat, read `flutter.md` in this skill directory. Docs: https://docs.tracewayapp.com/client/flutter
- **React Native**: `npm install @tracewayapp/react-native`, wrap the app in `TracewayProvider`. Docs: https://docs.tracewayapp.com/client/react-native
- **Android** (native Kotlin/Java): add `implementation("com.tracewayapp:traceway:1.0.1")` from Maven Central and call `Traceway.init(application = this, connectionString = "<token>@https://<instance>/api/report", options = TracewayOptions(version = "1.0.0"))` from `Application.onCreate()` (register the `Application` class in the manifest; no permission entry is needed, the AAR's own manifest declares `INTERNET` and `ACCESS_NETWORK_STATE` and the merger folds them in). It captures every uncaught Java/Kotlin exception on every thread plus manual `Traceway.captureException(...)`; errors and crashes only, no session replay. Release builds run R8, so production crashes arrive with renamed classes and rewritten line numbers and stay unreadable until the build's `mapping.txt` is uploaded: apply the `com.tracewayapp.symbols` Gradle plugin (`version "1.0.1"`, resolved from `mavenCentral()`), which injects `BuildConfig.TRACEWAY_PROGUARD_UUID` and uploads the mapping with the mobile project's upload token; pass that UUID into `TracewayOptions(proguardUuid = ...)` so each crash matches its mapping. For init code, options, the manifest, and the full Gradle plugin setup, read `android.md` in this skill directory. Docs: https://docs.tracewayapp.com/client/android
- **iOS / Swift** (native SwiftUI or UIKit): add the Traceway iOS SDK via Swift Package Manager (`https://github.com/tracewayapp/traceway-ios.git`) and call `Traceway.start(connectionString: "<token>@https://<instance>/api/report", options: TracewayOptions(version: "1.0.0"))` as early as possible. It captures uncaught `NSException`s and fatal signals (hard crashes upload on the next launch) plus manual `Traceway.capture(...)`; it reports errors and crashes only (no session replay). Release crashes arrive as bare addresses until the build's dSYMs are uploaded, so set up dSYM upload with the mobile project's upload token. For init code, options, the debugger caveat, and dSYM upload, read `ios.md` in this skill directory. **If the app is NOT a Swift app** (Objective-C only, a cross-platform stack with no Traceway mobile SDK, or a team standardized on OpenTelemetry), there is no native SDK: use an OTel distribution like Honeycomb with its exporter pointed at `<instance>/api/otel` and a `Authorization: Bearer <project-token>` header, exactly like a backend (see "Backend OTel Setup"). The non-Swift path is also in `ios.md`.

## Step 8: Deployment and Server Metrics

Use the deployment and host-metrics choice confirmed in "Propose and Confirm the Project Map". If it remains unresolved, ask before changing deployment configuration:

1. **How is this project deployed?** Docker on a VM / directly on a VM or bare metal / Kubernetes / serverless or PaaS.
2. **Do you want server (host) metrics tracked in Traceway?** CPU, memory, disk, filesystem, network of the machine running the app.

| Deployment | Wants host metrics | What to do |
|---|---|---|
| Docker on a VM, or directly on a VM/host | Yes | Install the **Traceway OTel Agent** on the host (below). For Docker deploys this is the default; the agent goes on the host, not in a container. |
| Kubernetes | Any | Agent not applicable (host service, no Docker image or K8s manifests by design). In-process app metrics still flow via the OTLP metrics exporter from "Backend OTel Setup". |
| Serverless / PaaS | Any | No host to install on; skip. |
| Anything | No | Skip. |

The agent is a tiny pre-configured OTel Collector that scrapes host metrics every 60s. It MUST use the same backend project token as the API, workers, tasks, and AI traces. Install it on the host (Linux systemd / macOS launchd; PowerShell installer exists for Windows):

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

Metrics arrive within ~60s under their hostmetrics names (`system.cpu.utilization`, `system.memory.usage`, ...). The host is identified by the `server_name` tag, which comes from `TRACEWAY_SERVICE_NAME`. Resource attributes such as `host.name` are not stored as tags at all, so give every host a distinct `TRACEWAY_SERVICE_NAME` or the hosts cannot be told apart. The metrics do NOT populate the built-in CPU/memory charts (those read the Go SDK's exact names). Install the **OTelemetry Server Agent** dashboard template instead: open the command palette with Cmd K, search for it, and it installs a dashboard already wired to `system.cpu.utilization`, `system.memory.*`, `system.filesystem.*`, `system.disk.*` and `system.network.*`. Custom widgets cover anything the template misses. Agent repo: https://github.com/tracewayapp/traceway-otel-agent

## Step 9: Verify

Every dashboard page named below is at `<instance>/<page>`: `/endpoints`, `/issues`, `/tasks`, `/ai-traces`, `/logs`, `/dashboards`. Set the time picker to the last 15 minutes before reading any of them. Verify each project independently and finish with a project-to-component summary.

1. **Backend project**
   - Run `curl <app>/api/users/1`, then `/2`, then `/3`, and open `/endpoints`. Exactly one row, `GET /api/users/:id`, with non-zero status codes. Three rows means `http.route` is not being set. Fix that before checking anything else.
   - Two results that look broken but are not: a request to a URL matching no route arrives as `UNMATCHED`, not as the URL you typed; and if the project has "drop healthy healthchecks" enabled, successful `/health` requests are discarded on ingest, so never use a healthcheck route as the smoke test.
   - Trigger a handled or test exception, then open `/issues`. The exception is listed with its endpoint or task context, and that request's endpoint row shows status 500.
   - Open one endpoint row: database queries and outgoing HTTP calls appear as child spans.
   - Run each background job once, then open `/tasks`. One row per job, under a stable name with no ids or timestamps in it.
   - If logs were wired, emit one INFO and one ERROR line inside a request, then open `/logs`. Both carry the same trace id as that endpoint.
   - If "AI Traces" applied, make one model call, then open `/ai-traces`. The call is listed with model, tokens, and cost where available.
   - Open `/dashboards` and confirm application/runtime metrics arrive. If the OTel Agent was installed, host metrics reach this same backend project within about 60 seconds, each host under its own `server_name` tag.
2. **Browser project**
   - Trigger a test browser error and confirm it appears in this project, not the backend project.
   - Confirm the stack resolves to original source files and lines after the matching source maps and bundles are uploaded.
   - If distributed tracing was configured, confirm a browser issue or request links to its backend trace.
3. **Each mobile project**
   - Trigger the platform's safe test error or crash flow and confirm it appears only in the matching mobile project.
   - For obfuscated Flutter, minified Android, or Release iOS builds, confirm the matching symbols, R8 mapping, or dSYM resolves the production stack.
4. **Credential and topology audit**
   - Confirm browser and mobile integrations do not use the backend token, and that the OTLP exporter does not use a browser project's token (rule 3 in Step 4).
   - Confirm APIs, workers, schedulers, AI calls, and host metrics use the backend project unless the user approved an isolation boundary.
   - Report any verification that could not run because a local, deployment, or CI variable is not yet populated.
5. If the `traceway` CLI is installed, run `traceway login`, select the backend project with `traceway projects use <project-id>` (or pass `--project <project-id>` on every call), then:
   ```bash
   traceway endpoints list --since 15m
   traceway exceptions list --since 15m
   traceway logs query --since 15m
   traceway metrics query --name system.cpu.utilization --since 15m
   ```
   Tasks and AI traces have no `list` subcommand, only `show <id>`, so check those two on `/tasks` and `/ai-traces` in the dashboard.
