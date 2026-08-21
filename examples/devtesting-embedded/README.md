# devtesting-embedded

Embedded Traceway backend + OTel-instrumented Go service + jQuery frontend, for local development and feature testing.

## Quick Start

```bash
go run .
```

Then:
- App: http://localhost:8080/cdn (no build step) or http://localhost:8080 (requires `cd frontend && npm install && npm run build` first)
- AI chat: http://localhost:8080/chat (needs `ORCAROUTER_API_KEY` or `OPENROUTER_API_KEY` in `.env`)
- Dashboard: http://localhost:8082 — login `admin@localhost.com` / `admin`

## AI chat (conversations, tool calls, sub-agents)

`/chat` is a no-build chat UI backed by an OpenAI-compatible gateway, built to exercise the AI conversation analytics end to end. Configuration lives in `.env` (gitignored):

```
# OrcaRouter (https://www.orcarouter.ai) — preferred; routing, fallback, guardrails
ORCAROUTER_API_KEY=sk-orca-...
ORCAROUTER_MODEL=orcarouter/auto

# OpenRouter — fallback provider
# OPENROUTER_API_KEY=sk-or-...
# OPENROUTER_MODEL=anthropic/claude-sonnet-5
```

When `ORCAROUTER_API_KEY` is set the chat routes through [OrcaRouter](https://www.orcarouter.ai) (`https://api.orcarouter.ai/v1`); otherwise it falls back to OpenRouter. It also runs gateway-level, zero-trust security for AI agents on the same endpoint — screening every prompt/response and governing every tool call on a default-deny basis, with no application code changes.

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

- **Traceway backend**, embedded on port 8082, SQLite storage in `./storage/`.
- **Go app** on port 8080, instrumented with OpenTelemetry:
  - Traces via OTLP/HTTP → `http://localhost:8082/api/otel/v1/traces`
  - **Logs via OTLP/HTTP** → `http://localhost:8082/api/otel/v1/logs`
  - Two OTel providers in the same process to exercise the distributed-logs flow: `backend-service` and `worker-service`, both reporting to the `Backend API` project.
- **jQuery frontend** pages (`/` and `/cdn`) — unchanged; useful for exercising the jQuery SDK independently.

## Testing the logs feature

Five endpoints exercise different parts of the logs feature end-to-end:

| Endpoint | What it exercises |
|---|---|
| `GET /api/test-error` | Error log + exception recording on the root span |
| `GET /api/test-success` | DEBUG + INFO logs on a simple successful request |
| `GET /api/test-log-levels` | One log at each severity (TRACE/DEBUG/INFO/WARN/ERROR/FATAL) — validates `SeverityBadge` rendering |
| `GET /api/test-spans-with-logs` | Nested child spans (`auth.verify` → `db.query` → `cache.lookup`) with logs emitted inside each and attributes on every child span. Exercises `parent_span_id` capture, the span tree + attribute popover on the trace detail page, and span-chip linking |
| `GET /api/test-distributed-logs` | Two services (`backend-service` + `worker-service`) emit logs under a shared `traceway.distributed_trace_id`. Powers the "Load logs from other traces" button on the trace detail page |

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
