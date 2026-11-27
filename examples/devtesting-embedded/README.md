# devtesting-embedded

Embedded Traceway backend + OTel-instrumented Go service + jQuery frontend, for local development and feature testing.

## Quick Start

```bash
go run .
```

Then:
- App: http://localhost:8080/cdn (no build step) or http://localhost:8080 (requires `cd frontend && npm install && npm run build` first)
- AI chat: http://localhost:8080/chat (needs `ORCAROUTER_API_KEY`, `CHEAPER_INFERENCE_API_KEY`, `UBERLLM_API_KEY` or `OPENROUTER_API_KEY` in `.env`)
- Dashboard: http://localhost:8082 — login `admin@localhost.com` / `admin`

## Ports and storage

| Variable | Default | What listens there |
|---|---|---|
| `DEVTESTING_APP_PORT` | `8080` | the Go demo app (`/`, `/cdn`, `/chat`, `/api/test-*`) |
| `DEVTESTING_TRACEWAY_PORT` | `8082` | the embedded Traceway backend and its dashboard; the demo services' OTLP exporters, the self-monitoring URL and the SDK URLs in the served pages and `static/app.js` follow it |

Everything is stored in `./storage/` under the working directory, and the user, projects and tokens below are
seeded only when that database has no user yet. A second instance next to a running one needs other ports and
another working directory:

```bash
go build -o /path/to/devtesting-embedded .
mkdir -p /path/to/fresh-dir && cd /path/to/fresh-dir
DEVTESTING_APP_PORT=18180 DEVTESTING_TRACEWAY_PORT=18182 /path/to/devtesting-embedded
```

A dashboard from `../../frontend` with `vite dev` reaches that backend through its `/api` proxy:
`TRACEWAY_API_PROXY=http://localhost:18182 npx vite dev --port 15183` in `frontend/`.
The example's own `frontend/` dev server (`npm run dev:serve`, port 5180) still proxies to the app on 8080.

## Seeded projects and tokens

| Project | Framework | Ingest token | Source-map / symbol upload token |
|---|---|---|---|
| Backend API | opentelemetry | `backend-dev-token` | – |
| jQuery Frontend | jquery | `frontend-dev-token` | – |
| Shoply Storefront | react | `shop-dev-token` | – |
| Traceway Monitoring | opentelemetry | `monitoring-dev-token` (the backend reports on itself here) | – |
| Flutter App | flutter | `flutter-dev-token` | `flutter-upload-token` |
| iOS App | ios | `ios-dev-token` | `ios-upload-token` |
| Android App | android | `android-dev-token` | `android-upload-token` |

The mobile SDKs connect with `<token>@http://<host>:8082` (for example `ios-dev-token@http://192.168.1.20:8082`
from a phone on the same network). The SDK wireframe galleries upload their recordings into the Flutter, iOS and
Android projects as sessions in their checkers' dashboard mode (`CI/wireframe/run.py --dashboard …` in
traceway-ios, `tools/wireframe/run.py --dashboard …` in traceway-android).

## AI chat (conversations, tool calls, sub-agents)

`/chat` is a no-build chat UI backed by an OpenAI-compatible gateway, built to exercise the AI conversation analytics end to end. Configuration lives in `.env` (gitignored). Uncomment the settings for the provider you want to use:

```
# OrcaRouter (https://www.orcarouter.ai)
# ORCAROUTER_API_KEY=sk-orca-...
# ORCAROUTER_MODEL=orcarouter/auto

# Cheaper Inference (https://cheaperinference.com)
# CHEAPER_INFERENCE_API_KEY=ci_live_...
# CHEAPER_INFERENCE_MODEL=gpt-5.4-mini

# UberLLM (https://uberllm.dev)
# UBERLLM_API_KEY=ull_...
# UBERLLM_MODEL=openai/gpt-5.4-mini

# OpenRouter (https://openrouter.ai)
# OPENROUTER_API_KEY=sk-or-...
# OPENROUTER_MODEL=anthropic/claude-sonnet-5
```

Set one provider API key. If multiple keys are set, the example checks them in this order: `ORCAROUTER_API_KEY`, `CHEAPER_INFERENCE_API_KEY`, `UBERLLM_API_KEY`, then `OPENROUTER_API_KEY`.

How it maps to Traceway:

- Every LLM call emits a `gen_ai.*` span into the `Backend API` project, with real token counts and cost from the gateway's usage accounting.
- The chat session id becomes `gen_ai.conversation.id`, so each browser conversation shows up on the Conversations tab. The persona picker sets `user.id` for the Users tab.
- The main **Support Chat Agent** has fake tools (`get_weather`, `lookup_order`, `get_server_time`) plus two delegation tools that run sub-agents in the same conversation: **Research Sub-Agent** (with `search_knowledge_base`) and **Math Sub-Agent** (with a real `calculate` evaluator). Each agent is a separate trace name on the AI Traces tab.
- Tool executions run in plain child spans (no `gen_ai.*` attributes) so they appear in the trace waterfall without inflating conversation turn counts; the tool calls themselves are parsed from the completion payloads.

Suggestion buttons in the UI trigger each tool and sub-agent. To test content flagging, swear at the bot or add custom terms in the project settings AI tab. The API is also curl-able:

```bash
curl -s http://localhost:8080/api/ai/chat -H 'Content-Type: application/json' -d '{
  "sessionId": "conv-1", "userId": "alice@example.com",
  "messages": [{"role": "user", "content": "Weather in Paris, and ask the math agent for 12*(3+4)"}]
}'
```

## What runs

- **Traceway backend**, embedded on port 8082 (`DEVTESTING_TRACEWAY_PORT`), SQLite storage in `./storage/`.
- **Go app** on port 8080 (`DEVTESTING_APP_PORT`), instrumented with OpenTelemetry:
  - Traces via OTLP/HTTP → `http://localhost:8082/api/otel/v1/traces`
  - **Logs via OTLP/HTTP** → `http://localhost:8082/api/otel/v1/logs`
  - Two OTel providers in the same process to exercise the distributed-logs flow: `backend-service` and `worker-service`, both reporting to the `Backend API` project.
- A **Shoply Storefront** React project (token `shop-dev-token`) for `testing/shop`'s storefront, which exercises remote data collection and the browser performance metrics; see that README.
- **Flutter App**, **iOS App** and **Android App** projects for the mobile SDKs (tokens above).
- **jQuery frontend** pages (`/` and `/cdn`) — unchanged; useful for exercising the jQuery SDK independently.

## Testing the logs feature

Five endpoints exercise different parts of the logs feature end-to-end:

| Endpoint | What it exercises |
|---|---|
| `GET /api/test-error` | Error log + exception recording on the root span |
| `GET /api/test-success` | DEBUG + INFO logs on a simple successful request |
| `GET /api/test-log-levels` | One log at each severity (TRACE/DEBUG/INFO/WARN/ERROR/FATAL) — validates `SeverityBadge` rendering |
| `GET /api/test-spans-with-logs` | Nested child spans (`auth.verify` → `db.query` → `cache.lookup`) with logs emitted inside each and attributes on every child span. Exercises `parent_span_id` capture, the span tree + attribute popover on the trace detail page, and span-chip linking |
| `GET /api/test-distributed-logs` | Two services (`backend-service` + `worker-service`) emit logs under a shared W3C trace ID. Both services appear in the trace log panel |

Trigger them all at once:

```bash
for path in test-error test-success test-log-levels test-spans-with-logs test-distributed-logs; do
  curl -s "http://localhost:8080/api/$path" > /dev/null
done
```

Then in the dashboard (logged in, **Backend API** project selected):

1. **Logs list** — open `/logs`. You should see six severity badges and every request's logs listed with a Message, Service (`backend-service` or `worker-service`), and trace ID. The severity dropdown and search (by Message / Service / Trace ID) should all work.
2. **Trace detail + chip linking** — open `/endpoints`, click one of the test endpoints, click a row. Under the Spans card you should see a **Logs** card listing that request's logs, each chipped with the span that emitted it. For `/api/test-spans-with-logs` specifically, verify logs emitted inside `db.query` chip as `db.query`, logs from `cache.lookup` chip as `cache.lookup`, and logs from handler code (between the child-span blocks) chip as the endpoint name.
3. **Distributed logs button** — on a trace from `/api/test-distributed-logs`, the Logs card shows a `Load logs from other traces` button. Clicking it loads logs from the `worker-service` trace under a divider.
4. **Span tree + attributes** — on a trace from `/api/test-spans-with-logs`, the Spans card renders a tree: `auth.verify` and `db.query` at the top level, `cache.lookup` indented under `db.query` with a chevron on `db.query` to collapse it. Clicking a span name opens a popover listing its attributes (e.g. `db.system`, `cache.hit`) with per-value copy buttons.
5. **`parent_span_id` + `attributes`** — verify in SQLite:
   ```bash
   sqlite3 storage/traceway_telemetry.db "SELECT substr(id,1,8), name, substr(parent_span_id,1,8), attributes FROM spans ORDER BY start_time DESC LIMIT 10;"
   ```
   Non-root spans should show a non-null `parent_span_id` matching their parent span's `id` and a JSON `attributes` column.

## Starting from a clean slate

To re-run migrations from empty (useful after any of these migrations ship: `0045_create_log_records`, `0046_add_parent_span_id_to_spans`, `0047_add_span_id_to_endpoints`, `0048_add_span_id_to_tasks`, or their SQLite counterparts `0003_create_log_records`, `0004_add_parent_span_id_to_spans`, `0005_add_span_id_to_root_traces`):

```bash
rm -f storage/traceway.db* storage/traceway_telemetry.db*
```

The backend will re-create the schema on next start.

## Pages (legacy jQuery setup — unchanged)

| URL | Description | Build required? |
|-----|-------------|-----------------|
| http://localhost:8080 | Node-built version — `@tracewayapp/jquery` bundled via Vite into `static/app.js` | Yes (`npm install && npm run build` in `frontend/`) |
| http://localhost:8080/cdn | CDN version — loads `@tracewayapp/jquery` IIFE from jsdelivr | No |
| http://localhost:8082 | Traceway dashboard | No |
