---
name: traceway
description: 'Operate a Traceway observability instance through the traceway CLI: log in, query exceptions, logs, endpoints, background tasks, sessions, AI traces, and metrics, and debug production issues down to root cause. Use when the user invokes /traceway with a subcommand, e.g. "/traceway login", "/traceway debug issue <hash|url|title>", "/traceway what''s broken in prod", or whenever they want to investigate errors, crashes, slowness, logs, or task/LLM behavior from an app monitored by Traceway.'
---

# Traceway

Drive a Traceway instance from the terminal with the `traceway` CLI. The first word of the argument decides the flow:

| Invocation | Flow |
|---|---|
| `/traceway login` | **Login**: install the CLI if missing, authenticate, select a project |
| `/traceway debug <issue ref or bug description>` | **Debug**: resolve the issue and investigate to root cause |
| `/traceway perf <endpoint or symptom>` | **Performance**: diagnose latency/slowness to root cause against a checklist of common bottlenecks |
| `/traceway <anything else>` | **Query**: answer the observability question with CLI reads |
| `/traceway` (no argument) | Ask what they want: log in, debug an issue, or run a query |

> The CLI is under active development. If a flag documented here does not appear in `traceway <command> --help`, trust the binary.

## Ground Rules (All Flows)

- **Reads are safe**: any `list` / `show` / `query` subcommand may run freely; they never mutate server state.
- **Writes require explicit user instruction**: `exceptions archive` / `unarchive` are the only mutating data commands; only run them when the user asks by name, with `--yes` in non-interactive contexts. "Look at this error" means read it, not archive it.
- **Output**: piped output defaults to JSON (table on a TTY). Prefer JSON + `jq`, and `--fields a,b,c` to trim responses. Keep `--page-size` at 10 to 20 for triage.
- **Time windows**: always bound queries, default `--since 1h` for "now" questions, `--since 24h` otherwise. `--since` accepts `s`, `m`, `h`, lowercase `Nd` (no `1w`, no `7d2h`). Absolute windows via `--from` / `--to` (RFC3339). Exception: `metrics list` defaults to the server's 7-day discovery window, which is what you want.
- **Exit codes**: 0 ok, 1 generic/API, 2 usage, 3 connection, 4 auth, 5 not found, 6 rate limited, 7 server 5xx. Errors emit `{"error":"<stable_id>","message":"...","hint":"...","exit_code":N}` on stderr; branch on the `error` field.
- On exit code 4 (auth), do not run `traceway login` yourself; switch to the Login flow and let the user enter credentials.

## Resolving Dashboard URLs

Users paste dashboard URLs (`https://<instance>/<route>`) as references in any flow. Resolve by route family:

| URL path | Identifies | How to fetch it |
|---|---|---|
| `/issues/<hash>` and `/issues/<hash>/events` | Exception group (hash = 16 hex chars) | `traceway exceptions show <hash>` |
| `/issues/<hash>/<occurrenceId>` (UUID) | One occurrence within the group | `traceway exceptions occurrence <occurrenceId> --recorded-at <t>` where `t` is the URL's `?t=` param. Direct and fast; also returns the occurrence's `sessionId` and session recording. No URL? get `recordedAt` from `traceway exceptions show <hash>` occurrences |
| `/endpoints/<endpoint>` | Endpoint group; the segment is the URL-encoded endpoint name (`GET%20%2Fapi%2Fusers%2F%3Aid` is `GET /api/users/:id`) | Decode it, then `traceway endpoints list --search "<decoded name>"` (the group has no id; `endpoints show` is for one request — next row) |
| `/endpoints/<endpoint>/<endpointId>` | One request (transaction) of that endpoint | `traceway endpoints show <endpointId> --recorded-at <t>` (`t` = the URL's `?t=` param). Returns the request, its span waterfall, and any linked exception/messages |
| `/tasks/<task>` | Background task group; the segment is the URL-encoded task name | Decode it, then `traceway tasks runs --task "<decoded name>"` (runs + aggregate stats); `traceway tasks list --search "<decoded name>"` for the group row |
| `/tasks/<task>/<taskId>` | Single task run | `traceway tasks show <taskId> --recorded-at <t>` (`t` = the URL's `?t=` param) |
| `/sessions/<sessionId>` | Session (the exceptions that fired during it; replay stays dashboard-only) | `traceway sessions show <sessionId> --started-at <t>`. The URL has no `?t=`; use the session's start, the URL's `from=`, or a linked occurrence's `recordedAt` (it falls inside the window). Occurrences reference sessions via their `sessionId` |
| `/ai-traces/<traceName>` | AI trace group | `traceway ai-traces list --search "<decoded name>"` for the group's token/cost stats; for one trace use the next row |
| `/ai-traces/<traceName>/<traceId>` | Single AI trace | `traceway ai-traces show <traceId> --recorded-at <t>` (`t` = the URL's `?t=` param); returns token/cost stats + the conversation |
| `/logs` | Logs page (its filters are not stored in the URL) | `traceway logs query` with flags taken from the user's description |
| `/issues`, `/endpoints`, `/tasks`, `/sessions`, `/ai-traces`, `/metrics`, `/` | List and dashboard pages | The matching `list` / `query` command |

**Time window**: most dashboard URLs carry `?preset=<p>` or `?from=<iso>&to=<iso>` (sticky across pages); honor them instead of the default window.

- `preset` values `5m 30m 60m 3h 6h 12h 24h 3d 7d` map directly to `--since`; the CLI has no month unit, so map `1M` to `--since 30d` and `3M` to `--since 90d`.
- `from`/`to` are ISO timestamps; pass via `--from`/`--to`, appending `Z` (or the correct offset) when missing, since the CLI requires RFC3339.
- No time params means the page was on its default; pick `--since` per the ground rules.

`preset`/`from`/`to` set the *window* for list/group views. Detail URLs additionally carry **`?t=<iso>`** — the single record's timestamp, URL-encoded. That `t` value is exactly what the by-id commands need as `--recorded-at` (or `--started-at` for sessions). See "Fast by-id lookups" next.

## Fast by-id lookups (always pass the timestamp)

The by-id detail commands — `exceptions occurrence`, `endpoints show`, `tasks show`, `ai-traces show`, `sessions show`, `traces show` — **require** the record's timestamp (`--recorded-at`, or `--started-at` for sessions). Telemetry tables are partitioned by day: with the timestamp the lookup is bounded to a small window and ClickHouse prunes to a few partitions; without it the server scans every partition (slow cold load). The flag is mandatory for exactly this reason — never omit it. It can be approximate (within ±24h), and you can recover or estimate it when it isn't handed to you; see "When you don't have the timestamp" below.

Where the timestamp comes from, in order of preference:

1. **A dashboard URL** — the `?t=<iso>` param is the record's `recordedAt`; URL-decode it and pass it verbatim. (Sessions have no `t`; use the session start, `from=`, or a linked occurrence's `recordedAt`.)
2. **A list/group you already fetched** — every `exceptions show` occurrence carries `recordedAt`. Capture the id and its `recordedAt` together, then drill in.
3. **A notification** — see below.

Query order when you hold an id: resolve its `recordedAt` first (URL, group, or notification), then call the by-id command with it.

### When you don't have the timestamp

The flag is required, so you must supply *something* — but it can be approximate. The lookup window is **±24h** around what you pass (**±48h** for `traces show`), and if the record isn't in that window the server falls back to an unbounded scan. So a timestamp within a day of the truth stays fast; a wrong guess still returns the right record, just slower. Resolve it in this order:

1. **Recover it from an API.** For an occurrence whose hash you know (e.g. `/issues/<hash>/<occurrenceId>` pasted without `?t=`), run `traceway exceptions show <hash>` and read that occurrence's `recordedAt` — the hash endpoint needs no timestamp. A group's `firstSeen`/`lastSeen` from `exceptions list` bound when its occurrences happened (`lastSeen` ≈ the most recent one).
2. **Estimate from context.** A notification's send time, the issue's `firstSeen`/`lastSeen`, or the URL's `preset`/`from` window all put you inside ±24h — good enough for a fast lookup.
3. **Ask the user.** If nothing pins it down (e.g. a bare occurrence/endpoint id with no hash, no time, and no list to recover from), ask roughly when it happened — "around when did this fire? within a day is enough" — and pass that. Don't invent a placeholder like "now" when the issue is old; that defeats the pruning and can miss the ±24h window entirely.

## Resolving a notification

Traceway notifications (email / Slack / webhook) come in two shapes. Read the body to tell them apart: an **exception** notification carries a `Hash:` and `Exception ID:`; a **performance** notification (latency, apdex, impact) names an endpoint and a threshold and has no hash. Webhook payloads also carry a `ruleType` field that names the rule exactly. Resolve each differently.

### Exception notification

The body embeds everything for a direct, fast lookup. It contains:

- `Hash: <16-hex>` — the exception group → `traceway exceptions show <hash>`.
- `Exception ID: <uuid>` — the specific occurrence.
- `Occurred at: 2006-01-02 15:04:05 UTC` — the occurrence timestamp. **Convert to RFC3339**: replace the space with `T` and ` UTC` with `Z` (→ `2006-01-02T15:04:05Z`).
- `View details: /issues/<hash>` — the deep link.

So from a notification, go straight to the occurrence (fast), then pivot reusing the same timestamp:

```bash
traceway exceptions occurrence <Exception ID> --recorded-at <Occurred at → RFC3339> --output json
# the result carries distributedTraceId and sessionId → traces show / sessions show below
```

### Performance notification (an endpoint became slow or critical)

These fire from latency/throughput rules, not from an error. There is no hash, no exception id, no occurrence to fetch. The webhook `ruleType` (or the subject wording) tells you which:

| ruleType | Subject / body shape | Deep link |
|---|---|---|
| `endpoint_p95_threshold` / `endpoint_p99_threshold` | `P95 latency 1250ms on <endpoint>` · "reached 1250ms over the last N minutes (threshold: 1000ms)" | `/endpoints?from=<iso>&to=<iso>` |
| `apdex_drop` | `Apdex dropped to 0.62 (threshold: 0.80)` · "across N requests over the last N minutes" | `/endpoints?from=<iso>&to=<iso>` |
| `impact_score_critical\|high\|medium` | `Endpoint <endpoint> impact became critical` · "impact score: 0.82. Reason: <reason>" | `/endpoints?from=<iso>&to=<iso>` |
| `metric_threshold` | `Metric <name> is <value> (threshold: gt 100)` | `/metrics?preset=1h` |

What the body gives you: the **endpoint name** (parse it from the subject/body; it is not a separate field), the **metric/percentile**, the **current value**, the **threshold**, and the **lookback window**. The `/endpoints?from=&to=` link is only a ~3-minute window around the fire time, so treat the fire time as the onset hint, not the incident window.

To resolve, hand straight to the **Performance flow** scoped to that endpoint, using the fire time as the time anchor:

```bash
# 1. confirm the alert against current stats for that endpoint
traceway endpoints list --search "<endpoint>" --since 1h --output json | jq '.data[0] | {p50, p95, p99, count, impact, impactReason}'
# 2. is this an accepted baseline? check whether an operator marked it slow
traceway endpoints slow "<endpoint>" --output json    # {offsetMs, reason}; offsetMs 0 = not marked
# 3. find when it crossed the threshold, then a slow trace (see Flow: Performance)
traceway endpoints chart --metric-type p95 --since 6h --interval-minutes 15 --output json
```

Step 2 matters: an `impact_score_*` or `apdex_drop` alert is already offset-aware, but `endpoint_p95_threshold` / `endpoint_p99_threshold` fire on **raw** latency regardless of any slow-marking, so a marked-slow endpoint can alert on latency its operators have already accepted. See "Account for user-marked slow endpoints" in `performance.md`.

## Flow: Login

### 1. Check for an existing install

```bash
traceway version
```

If it prints a version, skip to authentication.

### 2. Install if missing

Prebuilt binaries are on the [tracewayapp/traceway releases page](https://github.com/tracewayapp/traceway/releases) under `cli/vX.Y.Z` tags (the latest release may be a Backend release, so filter for CLI tags):

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = "aarch64" ] && ARCH=arm64
URL=$(curl -s "https://api.github.com/repos/tracewayapp/traceway/releases?per_page=20" \
  | grep -o "https://[^\"]*traceway_[^\"]*_${OS}_${ARCH}\.tar\.gz" | head -1)
TMP=$(mktemp -d)
curl -sL "$URL" | tar -xz -C "$TMP"
install -m 755 "$TMP/traceway" ~/.local/bin/traceway && rm -rf "$TMP"
```

Make sure `~/.local/bin` is on `PATH` (or install to `/usr/local/bin`). Fallback, build from source (requires Go):

```bash
git clone https://github.com/tracewayapp/traceway && cd traceway/cli
go build -o bin/traceway ./cmd/traceway && install -m 755 bin/traceway ~/.local/bin/traceway
```

Verify with `traceway version`.

### 3. Authenticate

Login prompts for the password interactively, so ask the user to run it themselves (in Claude Code, suggest typing `! traceway login --url https://<instance>` so the output lands in the session):

```bash
traceway login --url https://<traceway-instance>
```

Non-interactive alternative when the password is in a secret store (never echo a password into the command line or shell history):

```bash
printf '%s' "$TRACEWAY_PASSWORD" | traceway login --url https://<instance> --username you@example.com --password-stdin
```

Multiple instances or accounts coexist via profiles: `traceway login --url ... --profile work`, then `traceway profiles list` / `traceway profiles use work`.

### 4. Select a project and smoke-check

```bash
traceway projects list
traceway projects use <project-id>
traceway exceptions list --since 24h
```

The selected project is used implicitly by all subsequent commands.

## Flow: Debug

`/traceway debug issue X` or `/traceway debug <free-form bug description>`.

### 1. Resolve the issue reference

`X` can be several things; resolve it to an exception hash (16 hex chars):

| Reference looks like | How to resolve |
|---|---|
| Dashboard URL | See "Resolving Dashboard URLs" above; for `/issues/...` URLs the path segment right after `/issues/` is the hash, and `?preset`/`?from`/`?to` give the time window. **When a URL points at an issue, fix the LAST (most recent) occurrence of that issue — see step 2.** |
| Bare 16-char hex string | Already the hash |
| Anything else (title, error message, type, file name) | Search: `traceway exceptions list --since 7d --search "<text>"`; widen to `--since 30d` (and `--include-archived`) if empty |
| No issue reference, just a bug description | Skip to triage below |

When a search returns multiple groups, show a shortlist (hash, count, lastSeen, first stack line) and ask the user which one before drilling in.

```bash
traceway exceptions list --since 7d --search "checkout" --output json \
  | jq '.data[]? | {hash: .exceptionHash, count, lastSeen, top: (.stackTrace | split("\n")[0])}'
```

### 2. Drill into the issue

```bash
traceway exceptions show <hash>
```

This is the high-value call: full stack trace, occurrence list with `recordedAt`, `attributes` (user IDs, app versions, request context), and optional `distributedTraceId` / `sessionId` per occurrence. `firstSeen` correlates with deploys: a group that first appeared right after a release points at that release's diff. A bogus hash exits 5 with `not_found`; fall back to search.

**When the user gave an issue URL (or hash), fix the LAST occurrence — not "the group".** A single hash can bundle *several distinct errors*: the hash is computed from a normalized stack trace with the message stripped, so two unrelated failures that share their top frames (e.g. both captured at the same middleware/recovery frame) collapse into one group. The group's representative stack trace and `firstSeen` may belong to a different, now-dormant error than the one the user is looking at. Anchor on the most recent occurrence and fix that specific failure path:

```bash
# The occurrence the user actually wants: the latest one. Pin its exact message + attributes + trace.
traceway exceptions show <hash> --output json \
  | jq '.occurrences | sort_by(.recordedAt) | last
        | {recordedAt, message: (.stackTrace | split("\n")[0]), attributes, traceId, distributedTraceId}'

# Then confirm whether the group is homogeneous or mixed — distinct first lines = distinct bugs:
traceway exceptions show <hash> --output json \
  | jq -r '[.occurrences[].stackTrace | split("\n")[0]] | group_by(.)
           | map("\(length)x \(.[0])") | .[]'
```

If the group is mixed, scope the fix to the last occurrence's error only; mention the other clusters in the report but do not fix them unless asked. The last occurrence's message and `attributes` (not the group's representative stack) define the failure to reproduce and fix — and its `recordedAt` is the time hint to pass to the by-id `traces show` / `sessions show` lookups below.

### 3. Triage and correlate (also the entry point for free-form bug descriptions)

From the description extract symptom, affected endpoint/feature, and time window, then read several signals before forming a hypothesis:

```bash
traceway exceptions list --since 24h --order-by lastSeen        # what is erroring (firstSeen for regressions, count for volume)
traceway logs query --since 24h --min-severity error            # errors and worse (names or OTel numbers both work)
traceway logs query --since 24h --search "payment declined"     # search log bodies
traceway logs query --since 24h --service checkout-api --min-severity warn
traceway logs query --since 24h --attr user.id=42 --min-severity error   # by log attribute; resource:/scope:/log: prefix picks the map
traceway endpoints list --since 24h --search "checkout"         # latency p50/p95/p99 and error counts, --order-by impact|count|p95|lastSeen
traceway tasks list --since 24h --order-by p95                  # background tasks grouped by name; --order-by impact|count|p50|p95|avg|lastSeen
```

`--min-severity` accepts a name (`trace`, `debug`, `info`, `warn`, `error`, `fatal`) or the OTel number (1, 5, 9, 13, 17, 21).

**Correlate by trace**: when an occurrence or log line carries a trace ID, pull the whole request timeline; this is usually the fastest route to a root cause:

```bash
traceway exceptions show $HASH --output json | jq -r '.occurrences[0].distributedTraceId' \
  | xargs -I{} traceway logs query --distributed-trace-id {} --output json
```

`--distributed-trace-id` takes the UUID and returns the logs of *every* OTel trace under it, across services (`--exclude-trace-id <otel-id>` drops one noisy member). Plain `--trace-id` is for a single OTel trace id from a log line.

Pull the whole cross-service trace and the user's session, reusing the occurrence's `recordedAt` as the (mandatory) time hint so both lookups stay partition-bounded:

```bash
OCC=$(traceway exceptions show $HASH --output json | jq -c '.occurrences[0]')
TS=$(jq -r '.recordedAt' <<<"$OCC")
DT=$(jq -r '.distributedTraceId // empty' <<<"$OCC")
SID=$(jq -r '.sessionId // empty' <<<"$OCC")
[ -n "$DT" ]  && traceway traces show "$DT" --recorded-at "$TS"      # every endpoint/task/ai-trace/exception node across services
[ -n "$SID" ] && traceway sessions show "$SID" --started-at "$TS"    # the session + the exceptions that fired in it
```

`traces show` is usually the single highest-value RCA call: it stitches one logical request together end to end across services.

**Check metrics for systemic causes** (spikes lining up with `firstSeen` suggest saturation rather than a code bug):

```bash
traceway metrics list                            # discover names, tag keys, units first — do not guess names
traceway metrics tags <name>                     # the metric's tag keys; add a key for its values
traceway metrics tags <name> <key>               # e.g. values for host → plug into --tag/--group-by
traceway metrics query --name system.cpu.utilization --aggregation max --since 24h
traceway metrics query --name <name> --aggregation avg|sum|count|min|max [--tag key=value] [--group-by <tag>]
```

Discovery first: `metrics list` (server-default 7-day window) shows what exists, so query only names it returned. The CLI also accepts `p50|p95|p99` aggregations, but the server has no quantile aggregation for metric points and silently computes `avg` for them — never present those as percentiles. Latency percentiles come from `traceway endpoints list`, computed from raw request durations. Host metrics from the Traceway OTel Agent live under `system.*` names, and OTLP histogram metrics are stored as two series, `<name>.avg` and `<name>.count`.

### 4. Correlate with the code

1. Open the files and lines named in the stack trace and read the failing path.
2. If the issue started at a known time, check what shipped then: `git log --since "<firstSeen>" --until "<firstSeen + 1h>"` or the deploy history.
3. Form a hypothesis that explains the targeted occurrence's full observation set (its message, affected endpoint, timing, volume) — not just the first stack frame. When a URL/hash anchored you to the last occurrence (step 2), explain *that* error; do not stretch one hypothesis to cover sibling clusters that merely share the hash.
4. Propose or implement the fix per the user's instruction, scoped to the targeted occurrence's failure path.

### 5. Report and clean up

Summarize: symptom, evidence (exception hashes, log excerpts, metric anomalies), root cause, fix. Include `traceway exceptions show <hash>` references so the user can verify. After a fix is deployed and verified, archive only when the user asks:

```bash
traceway exceptions archive <hash> --yes
```

## Flow: Performance

`/traceway perf <endpoint or symptom>`, or any request about slowness, high latency, p95/p99, or "why is X slow". Approach it as a senior performance engineer: quantify first, localize the cost to a span, confirm the cause against the checklist, separate code from saturation, then root cause. Reads only; never archive or mutate.

For the full methodology and the bottleneck checklist (Select N+1, missing index, chatty IPC, connection-pool exhaustion, GC pauses, and ~20 more, each tied to the Traceway signal that confirms it), read `performance.md` in this skill directory. It is the authoritative reference.

The loop, using the read commands documented in this skill:

1. **Quantify and localize.** `traceway endpoints list --since 24h --order-by p95` (or `impact`). Read the latency shape first: p50 already high means every request pays it (systemic: query, index, algorithm); p99 much greater than p50 means a tail (contention, pool exhaustion, GC, retries, a flaky dependency). Note that `impact`/`impactReason` are offset-adjusted for marked-slow endpoints but the raw p50/p95/p99 are not.
2. **Check the accepted baseline.** Before calling an endpoint slow, run `traceway endpoints slow "<endpoint>"`. A non-zero `offsetMs` means an operator accepted that much latency (with a `reason`), so only latency *beyond* the offset is a real regression; `offsetMs 0` means default thresholds apply. Skip this only for a clearly systemic, multi-endpoint slowdown.
3. **Pinpoint when it started: adjust the window down.** Do not investigate the default window blindly; narrow it until it brackets the onset. `traceway endpoints chart --metric-type p95 --interval-minutes <n>` returns latency over time for the top endpoints (`{timestamp, endpoint, value}` in ms): read down one endpoint's buckets for the step where p95 jumps. Confirm the cause with `traceway metrics query --name <metric> --interval-minutes <n>` (the infra/runtime curve). For a specific route not in the top 5, bisect `traceway endpoints list --search <name> --from <a> --to <b>` over adjacent windows. See `performance.md`, "Pinpointing when the slowness started".
4. **Get a representative slow trace.** `traceway endpoints show <endpointId> --recorded-at <t>` for one request's span waterfall, or `traceway traces show <distributedTraceId> --recorded-at <t>` for the cross-service timeline. Take the trace from inside the slow window you just found; find the long pole.
5. **Match the long pole to the checklist.** Is the dominant span a database call, an external call, in-process compute, or a *gap* before work starts (queueing / lock wait / pool exhaustion)? Look it up in `performance.md`.
6. **Separate code from saturation.** Pull infra/runtime metrics over the same window: `traceway metrics query --name system.cpu.utilization --aggregation max`, then `mem.used_pcnt`, `go.gc_pause`. A spike that lines up with the latency onset points at saturation, not a code bug.
7. **Correlate with code and deploys.** With the onset time from step 3, check what shipped then: `git log --since '<onset - 30m>' --until '<onset + 30m>'` or the deploy history. A jump at a release is a regression; a gradual ramp with no deploy is data growth (N+1, missing index, no pagination).
8. **Report.** The bottleneck, the evidence (endpoint percentiles, the onset time, the dominating span, any correlated metric anomaly), the root cause mapped to a checklist item, and a concrete fix, with the `endpoints show` / `traces show` references so the user can verify. If the endpoint is marked slow, state whether the latency exceeded the accepted offset.

## Flow: Query

For free-form requests ("what's broken in prod?", "is /api/checkout slow?", "show errors for service X"), use the read commands directly. A request specifically about latency or slowness routes to the Performance flow above.

### Command reference

| Command | Purpose |
|---|---|
| `traceway projects {list,use}` | List or select the active project |
| `traceway exceptions list` | Grouped exceptions; `--search`, `--search-type text\|regex`, `--order-by lastSeen\|firstSeen\|count`, `--include-archived` |
| `traceway exceptions show <hash>` | One group: full stack trace + occurrences |
| `traceway exceptions occurrence <id> --recorded-at <t>` | One occurrence by id (fast): full detail + `sessionId` + recording |
| `traceway exceptions archive/unarchive <hash>...` | Mutating; explicit user request + `--yes` only |
| `traceway logs query` | Logs; `--search` (`--search-type body\|attribute`), `--service`, `--min-severity <n\|name>`, `--attr [resource:\|scope:\|log:]key=value`, `--trace-id`, `--distributed-trace-id` (+ `--exclude-trace-id`) |
| `traceway endpoints list` | Per-endpoint p50/p95/p99 and counts; `--search`, `--order-by impact\|count\|p95\|lastSeen` |
| `traceway endpoints chart` | Latency over time for the top endpoints; `--metric-type total_time\|p50\|p95\|p99`, `--interval-minutes`. Use to find when latency changed |
| `traceway endpoints slow <endpoint>` | Whether an operator marked this endpoint slow: `{offsetMs, reason}`. `offsetMs 0` = not marked. The offset is the accepted-latency baseline |
| `traceway endpoints show <id> --recorded-at <t>` | One request by id: span waterfall + linked errors |
| `traceway tasks list` | Tasks grouped by name: counts + p50/p95/avg; `--search`, `--order-by impact\|count\|p50\|p95\|avg\|lastSeen`, `--root-filter all\|root\|non-root` |
| `traceway tasks runs [--task <name>]` | Individual runs (id + recordedAt feed `tasks show`); with `--task` adds aggregate stats (avg/median/p95/p99 ms, throughput/min) |
| `traceway tasks show <id> --recorded-at <t>` | One background task run by id |
| `traceway ai-traces list` | AI traces grouped by name: counts, tokens, cost, durations; `--order-by count\|p50\|p95\|avg\|totalTokens\|totalCost\|lastSeen` (default totalCost), `--search`, `--root-filter` |
| `traceway ai-traces show <id> --recorded-at <t>` | One AI trace by id + its conversation |
| `traceway sessions list` | Sessions, newest first (id + startedAt feed `sessions show`); `--attr key=value`, `--order-by startedAt\|duration` |
| `traceway sessions show <id> --started-at <t>` | One session by id + the exceptions that fired in it |
| `traceway traces show <id> --recorded-at <t>` | Distributed trace: every service node sharing the id |
| `traceway metrics list` | Discover metric names, tag keys, type/unit; `--search` (client-side), default window 7d |
| `traceway metrics tags <name> [<key>]` | A metric's tag keys, or the observed values of one key |
| `traceway metrics query --name <metric>` | Time series; `--aggregation`, `--tag`, `--group-by`, `--interval-minutes` |
| `traceway profiles {list,use}`, `login`, `logout`, `version` | Profile and session management |

The by-id `show`/`occurrence` commands take their id from a dashboard URL, a notification, or a `list`/`runs` row — and **require** the record's timestamp (`--recorded-at` / `--started-at`); see "Fast by-id lookups" above. The `list`/`runs` tables print timestamps in RFC3339 exactly so they can be passed straight back.

Not implemented yet (do not fabricate flags; point the user at the web UI): a `list` verb for `traces` (distributed traces are only reachable by id via `traces show`), and session replay.

### Recipes

```bash
# What's broken right now
traceway exceptions list --since 1h --order-by lastSeen --page-size 10 --output json \
  | jq '.data[]? | {hash: .exceptionHash, count, lastSeen}'

# Did anything NEW break since a deploy at 13:00 UTC
traceway exceptions list --from 2026-06-11T13:00:00Z --to "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --order-by firstSeen --output json \
  | jq '.data[]? | select(.firstSeen >= "2026-06-11T13:00:00Z") | {hash: .exceptionHash, firstSeen, count}'

# Worst endpoint by latency
traceway endpoints list --since 1h --order-by p95 --page-size 1 --output json | jq '.data[0]'

# Errors for one service (exceptions --search is free text, not a service filter; use logs)
traceway logs query --service checkout-api --min-severity error --since 1h --output json \
  | jq '.data[]? | {timestamp, body, traceId}'

# Which background tasks are busy and erratic (impact = count * p95-p50 spread)
traceway tasks list --since 24h --order-by impact --output json \
  | jq '.data[]? | {taskName, count, p50Duration, p95Duration, lastSeen}'

# Drill into one task's runs, then one run's spans
traceway tasks runs --task "nightly-sync" --since 24h --output json \
  | jq '{stats, worst: (.data | sort_by(.duration) | last | {id, recordedAt, duration})}'
# → traceway tasks show <id> --recorded-at <recordedAt>

# What is this app spending on LLM calls
traceway ai-traces list --since 7d --order-by totalCost --output json \
  | jq '.data[]? | {traceName, count, totalTokens, totalCost}'

# Query a metric without guessing names: discover, inspect tags, then query
traceway metrics list --search cpu
traceway metrics tags system.cpu.utilization host
traceway metrics query --name system.cpu.utilization --aggregation max --group-by host --since 6h
```

Empty results (`data: null` or `data: []`) are not errors: widen the window, re-check the active project (`traceway projects list`), and if the app was never connected to Traceway, set it up first (the `traceway-setup` skill).
