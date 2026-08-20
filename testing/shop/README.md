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
