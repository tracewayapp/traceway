---
name: traceway-cli
description: How to use the `traceway` CLI to query a Traceway observability instance (exceptions, logs, distributed traces, metrics, endpoints, sessions). Use whenever the user asks about errors, exceptions, crashes, logs, latency, slow endpoints, performance, traces, spans, metrics, sessions, or "what's broken in prod". Assumes the user is already authenticated and a default project is configured — do not run `traceway login` unprompted. Read operations are safe to run freely; write operations require explicit user instruction.
---

# Using the traceway CLI

`traceway` is a command-line client for [Traceway](https://github.com/tracewayapp/traceway), an open-source observability platform (OpenTelemetry-native: logs, metrics, distributed traces, exception tracking, session replay). This skill teaches you how to drive the CLI productively.

> The CLI is under active development. If a documented flag here doesn't appear in `traceway <command> --help`, trust the binary, not this doc, and tell the user the doc is stale.

## When to invoke this skill

Invoke whenever the user's request involves looking *into* their running systems for diagnostic information:

- "What's erroring in prod right now?"
- "Show me the stack trace for the last 5 occurrences of X"
- "Why is `/api/checkout` slow?"
- "Find logs around trace `abc...`"
- "Did the deploy at 14:00 break anything?"
- "Pull the trace for request `xyz`"
- "What metrics are above their normal range?"

Do **not** invoke for:
- Generic Go/programming questions
- Questions about Traceway as a product (architecture, pricing) — those aren't CLI work
- Setting up credentials or first-time login

## Read vs write — default posture

**Read operations are safe.** You may run any of the query/list/show subcommands without asking, including with broad time windows or pagination. They never mutate server state.

**Write operations require explicit user instruction.** Subcommands that archive, resolve, mute, acknowledge, or otherwise change server state must be requested by name. Do not infer "the user wants this archived" from context. If the user says "look at this exception", they want to read it, not resolve it.

## Top-level invocation

```
traceway [global-flags] <subcommand> [subcommand-flags] [args]
```

### Global flags (apply everywhere)

| Flag | Purpose |
|---|---|
| `--profile <name>` | Select a credential/base-url profile from `~/.config/traceway/config.json` (singular file — both auth and profile metadata live there). Omit to use the current profile. |
| `--project <id>` | Override the configured project. Most commands need a project; assume the current profile has one set. |
| `--output json\|table\|yaml` | Output format. Default is **table when stdout is a TTY, JSON otherwise.** When you pipe through `jq` or capture output, JSON is automatic — you rarely need to set this flag. |
| `--fields <a,b>` | Comma-separated field projection for table/JSON output (e.g. `--fields id,name`). |
| `--yes` | Skip confirmation for mutating commands. Don't pass this unless the user explicitly asked you to mutate. |
| `--help` / `-h` | Print help. |

> The CLI currently has **no** `--base-url` or `--verbose`/`-v` global flag. If you need a different server, switch profiles. If a call returns empty, widen the time window or re-check the project — the CLI doesn't expose HTTP-level tracing.

### Output format guidance for LLM use

- **Always prefer JSON.** Pipe it into `jq` and pull only the fields you need. This keeps context small.
- Avoid `--output table` when capturing output programmatically — column widths and headers will inflate your context window.
- Use `--output yaml` only when the user explicitly asks; it's for humans.

## Subcommand reference

The CLI is organized by resource. Most resources have `list` (paginated search) and `show <id>` (detail) verbs.

### `projects`

List the projects available on the server. Useful for discovering project IDs when the user has more than one project or when no default is configured.

```
traceway projects list
```

The JSON form returns a **bare array** (no `data`/`pagination` envelope, unlike most other list endpoints). Currently observed fields: `id` (uuid), `name`. Other metadata you might expect (`slug`, `createdAt`, etc.) is not exposed by this subcommand yet.

You usually do **not** need this — the default profile has a project already. Only run it when:
- The user says "what projects are there?"
- A query fails with "no project configured" and you need to pick one.

### `exceptions` — the most-used subcommand

Grouped exception/error tracking. Each "group" is identified by a stack-trace hash. The list endpoint returns groups; the show endpoint returns occurrences within a group.

```
traceway exceptions list [--from RFC3339] [--to RFC3339] [--since 1h]
                          [--search "pattern"] [--search-type text|regex]
                          [--include-archived]
                          [--order-by lastSeen|firstSeen|count]
                          [--page 1] [--page-size 50]
```

Time window:
- Prefer `--since 1h` / `--since 24h` for relative windows. Default to `--since 1h` if the user says "right now" or "recently".
- `--since` accepts `s`, `m`, `h`, and `Nd` units (`7d` works and is parsed client-side as `7 * 24h`). Compound forms like `7d2h` are not accepted. Capital `D` (`7D`) exits 2 with `invalid_time_range`.
- Use `--from` / `--to` for absolute windows. Both are RFC3339 (`2026-05-15T10:00:00Z`).

Pagination: `--page` defaults to `1` and `--page-size` to `50`. Bare invocations like `exceptions list --since 24h` succeed without any pagination flags. Passing `--page 0` is still rejected server-side with HTTP 400 (the CLI passes the bad value through unchanged).

Result shape (JSON):
```jsonc
{
  "data": [
    {
      "exceptionHash": "9a8b...",
      "stackTrace": "...",          // top of stack; full trace via `show`
      "count": 142,
      "firstSeen": "2026-05-13T...",
      "lastSeen":  "2026-05-14T...",
      "hourlyTrend": [{ "timestamp": "...", "count": 7 }]
    }
  ],
  "pagination": { "page": 1, "pageSize": 50, "total": 312, "totalPages": 7 }
}
```

```
traceway exceptions show <exceptionHash> [--page 1] [--page-size 20]
```

Returns the exception group plus an array of recent `occurrences`. Each occurrence has at least `recordedAt`, `attributes`, and optional `distributedTraceId` / `sessionId` (verify the exact shape against your build — schema is still settling). A bogus hash exits with code `5` and `not_found`.

**Forbidden without explicit instruction:** `exceptions archive` and `exceptions unarchive`. These are the only mutating exception subcommands in the current CLI — there is no `resolve` / `mute` yet.

### `logs`

Full-text + structured search across logs.

```
traceway logs query [--from RFC3339] [--to RFC3339] [--since 1h]
                    [--search "pattern"] [--search-type body|attribute]
                    [--service <name>]
                    [--min-severity <uint8>]    # OTel severity number, see table
                    [--trace-id <uuid>]
                    [--order-by timestamp] [--sort-direction asc|desc]
                    [--page 1] [--page-size 50]
```

**Severity is a number, not a name.** The flag is `--min-severity`, not `--severity`, and it takes an OpenTelemetry severity number. Mapping:

| Severity   | Number |
|------------|--------|
| `TRACE`    | 1      |
| `DEBUG`    | 5      |
| `INFO`     | 9      |
| `WARN`     | 13     |
| `ERROR`    | 17     |
| `FATAL`    | 21     |

So "errors and above" is `--min-severity 17`. Do **not** write `--severity error` — that flag does not exist and the CLI will refuse it.

The current CLI does not have a `--attr key=value` filter — attribute-based filtering is done via `--search ... --search-type attribute`.

Result shape: `{data: [...] | null, pagination: {page, pageSize, total, totalPages}}`. When data is present, rows have at least `timestamp`, OTel severity fields, body, and a `traceId` if attached. Empty results come back as `data: null`, not `[]`.

Recipe — pull logs around an exception:
```bash
traceway exceptions show $HASH --output json | jq -r '.occurrences[0].distributedTraceId' \
  | xargs -I{} traceway logs query --trace-id {} --page 1 --output json
```

### `endpoints` — HTTP endpoint performance

```
traceway endpoints list [--since 24h]
                        [--search "/api/"]
                        [--order-by impact|count|p95|lastSeen]
                        [--sort-direction asc|desc]
                        [--page 1] [--page-size 50]
```

Returns endpoints with latency percentiles and request counts. Best for "what's slow?" questions.

There is **no `endpoints show` subcommand** in the current CLI — only `list`. If a user asks for a per-endpoint breakdown, use `--search "<endpoint-name>"` against `list`.

Note `--order-by` does not include `p99`, `errorRate`, or `requests` as choices — the only sort fields right now are `impact`, `count`, `p95`, `lastSeen`. The default (`impact`) is usually what you want.

Recipe — find the worst endpoint right now:
```bash
traceway endpoints list --since 1h --order-by p95 --page 1 --page-size 1 --output json \
  | jq '.data[0]'
```

### `metrics`

Custom metric queries. Traceway does not have a query DSL like PromQL — instead, you specify metric name + aggregation + filters + groupBy as flags.

```
traceway metrics query --name <metric.name>
                       [--aggregation avg|sum|count|min|max|p50|p95|p99]
                       [--from RFC3339] [--to RFC3339] [--since 1h]
                       [--interval-minutes <n>]
                       [--tag key=value]   # repeatable, exact match filter
                       [--group-by <tag>]  # one tag only
```

`--name` is required; omitting it exits with code 2 and `usage_error`.

**Percentile caveat:** the CLI accepts `p50`/`p95`/`p99` but the server has no quantile aggregation for metric points — it silently computes `avg` for them. Do not present percentile results from `metrics query` to a user; latency percentiles come from `traceway endpoints list` (computed from raw request durations).

**There is no `metrics list` / `metrics discover` subcommand.** The CLI cannot enumerate available metric names — the user has to provide one, or you have to guess from OpenTelemetry semantic conventions. Names observed live: `system.cpu.utilization`, `system.network.io`, `system.network.errors`, `system.network.dropped`. If the user is checking system telemetry, those four are the likeliest starting points; if they're checking app-level metrics, ask.

A bogus metric name returns exit 0 with `series: {}` (clean empty, not an error). Use that to probe whether a name exists.

`--aggregation <invalid-value>` exits 2 with `usage_error` (client-side enum validation). The other enum flags — `--search-type`, `--order-by`, `--sort-direction` — behave the same way: invalid values exit 2 before the request leaves the client.

Result shape — `series` is a **map keyed by group tag**, not an array:
```jsonc
{
  "results": [
    {
      "name": "system.network.io",
      "unit": "By",
      "series": {
        "__all__": [{ "timestamp": "2026-05-14T10:00:00-04:00", "value": 88645480308.55 }]
      }
    }
  ]
}
```

With `--group-by direction` on a network metric, the `series` keys become `"receive"` and `"transmit"` instead of `"__all__"`.

### Subcommands the CLI does not currently have

These are commonly requested but **not implemented** in the current binary. If a user asks for them, say so — don't fabricate flags.

- `traces show <traceId>` — span waterfall view. Trace IDs in `logs query` / `exceptions show` output are currently link-only.
- `sessions list` / `sessions show` — session replay listings.
- `ai-traces list` / `ai-traces show` — LLM observability.
- `endpoints show <name>` — per-endpoint detail (use `endpoints list --search`).
- `metrics list` / `metrics discover` — metric-name enumeration.

### `login` / `logout`

```
traceway login [--profile <name>]
traceway logout [--profile <name>]
traceway profiles use <name>
```

`login` prompts for base URL, email, password and stores a JWT in `~/.config/traceway/config.json`. `logout` removes a profile's stored credentials. `profiles use` switches the *local* current profile.

**Do not run any of these unprompted.** If a query returns 401 / "unauthorized" / "token expired", tell the user the session expired and ask them to run `traceway login`. Do not run it yourself. The same applies to `logout` and `profiles use` — even though `profiles use` only mutates local config, it can silently redirect every subsequent query to a different server.

## Common workflows

### "What's broken in prod right now?"

```bash
traceway exceptions list --since 1h --order-by lastSeen \
  --page 1 --page-size 10 --output json \
  | jq '.data[]? | {hash: .exceptionHash, count, lastSeen}'
```

Then drill into the top one:

```bash
traceway exceptions show $HASH --page 1 --output json | jq '.'
```

### "Why is this endpoint slow?"

```bash
# 1. confirm it's slow
traceway endpoints list --search "/api/checkout" --since 1h \
  --page 1 --output json | jq '.data[0]?'

# 2. find logs from that route. Filter by service if you know it; otherwise
#    use --search-type attribute to match on a log attribute key=value.
traceway logs query --since 15m --service api --min-severity 13 \
  --page 1 --output json | jq '.data[]? | {timestamp, body, traceId}'
```

There is no `traces show` subcommand yet — once a trace ID is captured, the waterfall view has to be inspected via the web UI.

### "Show me errors for service X in the last hour"

The `exceptions list --search` is free-text against the message body, not a service filter. Pivot to `logs query` for service-scoped queries:

```bash
traceway logs query --service X --min-severity 17 --since 1h \
  --page 1 --output json \
  | jq '.data[]? | {timestamp, body, traceId}'
```

(`--min-severity 17` = ERROR; see severity table above.)

### "Did anything new break since 13:00?"

```bash
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
SINCE="2026-05-14T13:00:00Z"
traceway exceptions list --from "$SINCE" --to "$NOW" --order-by firstSeen \
  --page 1 --output json \
  | jq '.data[]? | select(.firstSeen >= "'"$SINCE"'") | {hash: .exceptionHash, firstSeen, count}'
```

Filter is server-side time-window; the jq pass extracts groups that *first appeared* in the window (vs old groups that just had new occurrences).

### "What system metrics are unusual?"

```bash
for m in system.cpu.utilization system.network.io system.network.errors system.network.dropped; do
  echo "=== $m ==="
  traceway metrics query --name "$m" --since 1h --output json \
    | jq '.results[0].series | to_entries[] | {tag: .key, last: .value[-1].value}'
done
```

## Error handling

| Error | What it means | What to do |
|---|---|---|
| `401 Unauthorized` / "token expired" | JWT no longer valid | Tell the user; do **not** run `traceway login` yourself |
| `connection refused` / DNS errors | Wrong base URL or server down | Try `traceway --profile <name> projects list` to confirm reachability; ask the user to check `~/.config/traceway/config.json` |
| `no project configured` | Current profile lacks a project | Run `traceway projects list`, ask the user to pick one |
| HTTP 400 `'Page' failed on the 'min' tag` | You explicitly passed `--page 0` (or another value `< 1`); the server rejects it. The CLI default is `1`, so omitting the flag works. | Drop `--page` or pass `--page 1`. |
| `invalid_time_range: unknown unit "D"` (or similar) | You used a unit `--since` doesn't accept (capital `D`, compound `7d2h`, week `1w`) | Use `s`, `m`, `h`, or lowercase `Nd`. |
| `data: null` (empty) | Not an error | Confirm the time window; widen with `--since 24h` or `--since 7d` |
| `pagination.total` very large | Many results | Narrow with a search, service, severity, or smaller time window — don't page through 1000s |

If you see a panic, stack trace, or unhandled error in stderr, **stop** and report it verbatim to the user. That's a CLI bug, not an observability finding.

## Discipline for LLM operators

1. **Bound your time windows.** Default to `--since 1h`. Never run a query without a time window — Traceway can fall back to all-time and return huge results.
2. **Keep `--page-size` small.** 10–20 is usually enough for first-pass triage. Don't pull 500 rows into context.
3. **Capture identifiers, don't repeat list calls.** When you find a useful hash / trace id / session id, save it to a shell variable and reuse.
4. **Use `jq` aggressively.** Pull only the fields you need. A full exception group can be many KB; you usually want just `exceptionHash`, `count`, `lastSeen`, and the first line of the stack trace.
5. **Read before you act.** When the user describes a problem, read several signals (exceptions + endpoints + recent logs) before forming a hypothesis. Single-source diagnoses are usually wrong.
6. **Don't infer write actions.** "Look at this error" means show it, not archive it.
7. **One vertical workflow > many broad queries.** Pick a hypothesis, follow it down: exception → trace → spans → logs. Repeat with another hypothesis only if the first doesn't fit.
8. **Stop and ask when ambiguous.** If "the auth service" could be one of three services in the project, ask. Don't paper over the ambiguity with a broader query.

## Glossary

- **Project** — Traceway tenant boundary. All queries scope to one project.
- **Exception group / hash** — A bucket of exceptions sharing the same normalized stack trace, identified by a SHA-256 hash. The list endpoint returns groups; the show endpoint returns occurrences in a group.
- **Occurrence** — A single instance of an exception within a group.
- **Trace ID** — UUID identifying one distributed trace (a request that crosses services). Used to correlate logs, spans, exceptions.
- **Span** — One unit of work within a trace, with a parent span forming a tree. Span ID identifies it.
- **Session ID** — UUID for a user session; sessions can be replayed and linked to the exceptions/traces that occurred during them.
- **Distributed Trace** — Full collection of spans for one trace ID, typically shown as a waterfall.
- **Endpoint** — An HTTP route on a service, identified by name (often method + path).
- **Severity** — OpenTelemetry severity number on a log record. Common values: `1` TRACE, `5` DEBUG, `9` INFO, `13` WARN, `17` ERROR, `21` FATAL. The CLI's `--min-severity` flag takes the number, not the name.

## When this doc is wrong

The flag names, subcommand structure, and default behaviors documented here track the *intended* CLI surface. If `traceway <command> --help` shows different flags, trust the binary. If a documented subcommand doesn't exist, the CLI doesn't have it yet — say so to the user rather than fabricating a workaround.
