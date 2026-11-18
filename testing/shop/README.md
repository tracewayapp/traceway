# Shoply — Traceway demo shop

A small e-commerce app used to generate realistic demo data for a Traceway instance:

- **Frontend**: React 18 + Vite + Tailwind, instrumented with `@tracewayapp/react` (session recording on, all sessions recorded). Reports to the **react** demo project via `/api/report`.
- **Backend**: Go + Gin + in-memory SQLite, instrumented with **OpenTelemetry** (traces, logs, metrics over OTLP/HTTP). Reports to the **opentelemetry** demo project via `/api/otel/v1/*`. The retired Traceway Go SDK is gone.
- Single origin: the built frontend is embedded into the Go binary and served on **:8090**.

The app ships intentional bugs and random slowness (see `BUGS.md`) so issues, slow endpoints,
logs, metrics, session replays and AI traces all light up. Frontend exceptions carry a
`traceway-trace-id` that links them to the backend trace (distributed tracing).

## Setup

1. `backend/.env` (gitignored, see `backend/.env.example`):

```
TRACEWAY_URL=https://cloud.tracewayapp.com
TRACEWAY_BACKEND_TOKEN=<opentelemetry project token>
TRACEWAY_SERVICE_NAME=shop-backend
```

2. `frontend/.env` (gitignored, see `frontend/.env.example`):

```
VITE_API_BASE=/api
VITE_TW_CONNECTION=<react project token>@https://cloud.tracewayapp.com/api/report
TRACEWAY_URL=https://cloud.tracewayapp.com
TRACEWAY_SOURCEMAP_TOKEN=<source map upload token>
```

The sourcemap token comes from `seed/seed.mjs` with `MINT_SOURCEMAP_TOKEN=1` (or the
project's Connection page). Uploading source maps before generating browser traffic makes
the frontend issues symbolicate at ingest.

## Run

```bash
./build-and-run.sh          # build frontend -> embed -> build backend -> run on :8090
```

Dev mode: `cd frontend && npm run dev` (:5175, proxies /api to :8090).

## Run against a local Traceway (remote config and page performance)

`examples/devtesting-embedded` seeds a React project, **Shoply Storefront** (token `shop-dev-token`), next to its
**Backend API** project. Point the shop at it:

```
# frontend/.env
VITE_API_BASE=/api
VITE_TW_CONNECTION=shop-dev-token@http://localhost:8082/api/report

# backend/.env
TRACEWAY_URL=http://localhost:8082
TRACEWAY_BACKEND_TOKEN=backend-dev-token
TRACEWAY_SERVICE_NAME=shop-backend
```

Remote config, long tasks and route timing need the 2.x browser SDK. Until a release carries them, install a local
build over the published one without touching `package.json`: in `js-client`, `npm run build` and `npm pack` the
`core`, `opamp`, `frontend` and `react` packages, then `npm install --no-save <the four tarballs>` in `frontend/`.

Run `go run .` in `backend/` (after sourcing `.env`) and `npm run dev` in `frontend/` (:5175), then

```bash
cd traffic && DASHBOARD_URL=http://localhost:5180 node data-collection-e2e.mjs ./screenshots
```

`client-metrics-e2e.mjs` checks the performance metrics end to end (start the backend with `HEALTH_DEEP_TOKEN`,
`GEOIP_COUNTRY_HEADER=X-Test-Country`, `NOTIFICATION_POLL_SECONDS=10` and `SLO_POLL_SECONDS=10`, and pass the same
`HEALTH_DEEP_TOKEN` to the script). It visits the storefront as two releases on desktops and phones from several
countries, with release 0.2.0 made slow on purpose, and then reads what the SDK uploaded, what the server stored, and
what the Web Performance dashboard, its filters, a percentile alert, a release regression alert and an SLO per release
make of it. It also uploads `app.*` metrics for the Flutter project and opens Mobile Performance.

`data-collection-e2e.mjs` drives the dashboard and the storefront in Chromium: it creates Data Collection rules (route
timing for everyone, long tasks for one user, a frozen-page trigger), checks on the wire that each user sends exactly
what its rules allow, that the events are stored as metrics and charted by the Web Performance dashboard, and that a
metric alert built from a preset fires to a local webhook. It deletes the project's rules, alert rules and channels
first, so it can be repeated. Serve the dashboard from a build (`npm run build && npx vite preview --port 5180` in
`frontend/`); headless Chromium does not load the unbundled dev server reliably.

## Seed platform entities (monitors, on-call, status page, dashboards...)

```bash
traceway login --profile demo          # device flow into the demo account
cd seed && MINT_SOURCEMAP_TOKEN=1 node seed.mjs
```

Idempotent; see `seed/README.md`.

## Generate traffic

```bash
cd traffic && npm install && npx playwright install chromium
./run-traffic.sh --seed                # ~50 varied browser sessions + http load + AI chats
./run-traffic.sh --drip                # endless low-rate mode (nohup it)
./run-traffic.sh --seed --no-browser   # http load + AI chats only
```

Flags: `--base <url>` `--sessions N` `--parallel N` `--headed`.

## API

| Method | Path | Notes |
|---|---|---|
| GET | `/api/health` | untraced |
| GET | `/api/products` | ~75% N+1 slow path |
| GET | `/api/products/:id` | ~75% slow (review N+1 + recommendations); no UI path, driven by http-load |
| GET/POST | `/api/cart`, DELETE `/api/cart/:id` | POST returns 201, inventory.check sleep |
| POST | `/api/coupon` | `SAVE10`/`HALF50` valid (~75% panic: nil map), `EXPIRED` 400, unknown 500 |
| POST | `/api/checkout` | empty cart 500; 1-in-6 payment declined 500 |
| POST | `/api/support/chat` | fake LLM support agent; emits gen_ai.* spans (AI Traces) |
