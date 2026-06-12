---
name: traceway
description: 'Operate a Traceway observability instance through the traceway CLI: log in, query exceptions, logs, endpoints, and metrics, and debug production issues down to root cause. Use when the user invokes /traceway with a subcommand, e.g. "/traceway login", "/traceway debug issue <hash|url|title>", "/traceway what''s broken in prod", or whenever they want to investigate errors, crashes, slowness, or logs from an app monitored by Traceway.'
---

# Traceway

Drive a Traceway instance from the terminal with the `traceway` CLI. The first word of the argument decides the flow:

| Invocation | Flow |
|---|---|
| `/traceway login` | **Login**: install the CLI if missing, authenticate, select a project |
| `/traceway debug <issue ref or bug description>` | **Debug**: resolve the issue and investigate to root cause |
| `/traceway <anything else>` | **Query**: answer the observability question with CLI reads |
| `/traceway` (no argument) | Ask what they want: log in, debug an issue, or run a query |

> The CLI is under active development. If a flag documented here does not appear in `traceway <command> --help`, trust the binary.

## Ground Rules (All Flows)

- **Reads are safe**: any `list` / `show` / `query` subcommand may run freely; they never mutate server state.
- **Writes require explicit user instruction**: `exceptions archive` / `unarchive` are the only mutating data commands; only run them when the user asks by name, with `--yes` in non-interactive contexts. "Look at this error" means read it, not archive it.
- **Output**: piped output defaults to JSON (table on a TTY). Prefer JSON + `jq`, and `--fields a,b,c` to trim responses. Keep `--page-size` at 10 to 20 for triage.
- **Time windows**: always bound queries, default `--since 1h` for "now" questions, `--since 24h` otherwise. `--since` accepts `s`, `m`, `h`, lowercase `Nd` (no `1w`, no `7d2h`). Absolute windows via `--from` / `--to` (RFC3339).
- **Exit codes**: 0 ok, 1 generic/API, 2 usage, 3 connection, 4 auth, 5 not found, 6 rate limited, 7 server 5xx. Errors emit `{"error":"<stable_id>","message":"...","hint":"...","exit_code":N}` on stderr; branch on the `error` field.
- On exit code 4 (auth), do not run `traceway login` yourself; switch to the Login flow and let the user enter credentials.

## Resolving Dashboard URLs

Users paste dashboard URLs (`https://<instance>/<route>`) as references in any flow. Resolve by route family:

| URL path | Identifies | How to fetch it |
|---|---|---|
| `/issues/<hash>` and `/issues/<hash>/events` | Exception group (hash = 16 hex chars) | `traceway exceptions show <hash>` |
| `/issues/<hash>/<occurrenceId>` (UUID) | One occurrence within the group | `traceway exceptions show <hash> --output json \| jq '.occurrences[] \| select(.id=="<occurrenceId>")'`; occurrences are paginated, walk `--page` if not in the first page |
| `/endpoints/<endpoint>` | Endpoint group; the segment is the URL-encoded endpoint name (`GET%20%2Fapi%2Fusers%2F%3Aid` is `GET /api/users/:id`) | Decode it, then `traceway endpoints list --search "<decoded name>"` (there is no `endpoints show`) |
| `/endpoints/<endpoint>/<endpointId>` | One request (transaction) of that endpoint | No CLI for single transactions; fetch the group as above, correlate with `traceway logs query --trace-id <id>` if a trace ID is known from context |
| `/tasks/<task>` and `/tasks/<task>/<taskId>` | Background task group / single run | No CLI; point the user at the dashboard Tasks page |
| `/sessions/<sessionId>` | Session replay | No CLI; dashboard only. Occurrences reference sessions via their `sessionId` field |
| `/ai-traces/<traceName>` and `/ai-traces/<traceName>/<traceId>` | AI trace group / single trace | No CLI; dashboard only |
| `/logs` | Logs page (its filters are not stored in the URL) | `traceway logs query` with flags taken from the user's description |
| `/issues`, `/endpoints`, `/metrics`, `/` | List and dashboard pages | The matching `list` / `query` command |

**Time window**: most dashboard URLs carry `?preset=<p>` or `?from=<iso>&to=<iso>` (sticky across pages); honor them instead of the default window.

- `preset` values `5m 30m 60m 3h 6h 12h 24h 3d 7d` map directly to `--since`; the CLI has no month unit, so map `1M` to `--since 30d` and `3M` to `--since 90d`.
- `from`/`to` are ISO timestamps; pass via `--from`/`--to`, appending `Z` (or the correct offset) when missing, since the CLI requires RFC3339.
- No time params means the page was on its default; pick `--since` per the ground rules.

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
| Dashboard URL | See "Resolving Dashboard URLs" above; for `/issues/...` URLs the path segment right after `/issues/` is the hash, and `?preset`/`?from`/`?to` give the time window |
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

### 3. Triage and correlate (also the entry point for free-form bug descriptions)

From the description extract symptom, affected endpoint/feature, and time window, then read several signals before forming a hypothesis:

```bash
traceway exceptions list --since 24h --order-by lastSeen        # what is erroring (firstSeen for regressions, count for volume)
traceway logs query --since 24h --min-severity 17               # errors and worse
traceway logs query --since 24h --search "payment declined"     # search log bodies
traceway logs query --since 24h --service checkout-api --min-severity 13
traceway endpoints list --since 24h --search "checkout"         # latency p50/p95/p99 and error counts, --order-by impact|count|p95|lastSeen
```

Severity is an OTel number, not a name: 1 TRACE, 5 DEBUG, 9 INFO, 13 WARN, 17 ERROR, 21 FATAL. The flag is `--min-severity 17`, never `--severity error`.

**Correlate by trace**: when an occurrence or log line carries a trace ID, pull the whole request timeline; this is usually the fastest route to a root cause:

```bash
traceway exceptions show $HASH --output json | jq -r '.occurrences[0].distributedTraceId' \
  | xargs -I{} traceway logs query --trace-id {} --output json
```

**Check metrics for systemic causes** (spikes lining up with `firstSeen` suggest saturation rather than a code bug):

```bash
traceway metrics query --name system.cpu.utilization --aggregation max --since 24h
traceway metrics query --name <name> --aggregation avg|sum|count|min|max [--tag key=value] [--group-by <tag>]
```

The CLI also accepts `p50|p95|p99`, but the server has no quantile aggregation for metric points and silently computes `avg` for them — never present those as percentiles. Latency percentiles come from `traceway endpoints list`, computed from raw request durations. There is no `metrics list`; a bogus name returns an empty `series: {}` cleanly, so probing names is safe. Host metrics from the Traceway OTel Agent live under `system.*` names, and OTLP histogram metrics are stored as two series, `<name>.avg` and `<name>.count`.

### 4. Correlate with the code

1. Open the files and lines named in the stack trace and read the failing path.
2. If the issue started at a known time, check what shipped then: `git log --since "<firstSeen>" --until "<firstSeen + 1h>"` or the deploy history.
3. Form a hypothesis that explains ALL observations (error message, affected endpoint, timing, volume), not just the first stack frame.
4. Propose or implement the fix per the user's instruction.

### 5. Report and clean up

Summarize: symptom, evidence (exception hashes, log excerpts, metric anomalies), root cause, fix. Include `traceway exceptions show <hash>` references so the user can verify. After a fix is deployed and verified, archive only when the user asks:

```bash
traceway exceptions archive <hash> --yes
```

## Flow: Query

For free-form requests ("what's broken in prod?", "is /api/checkout slow?", "show errors for service X"), use the read commands directly.

### Command reference

| Command | Purpose |
|---|---|
| `traceway projects {list,use}` | List or select the active project |
| `traceway exceptions list` | Grouped exceptions; `--search`, `--search-type text\|regex`, `--order-by lastSeen\|firstSeen\|count`, `--include-archived` |
| `traceway exceptions show <hash>` | One group: full stack trace + occurrences |
| `traceway exceptions archive/unarchive <hash>...` | Mutating; explicit user request + `--yes` only |
| `traceway logs query` | Logs; `--search` (`--search-type body\|attribute`), `--service`, `--min-severity <n>`, `--trace-id` |
| `traceway endpoints list` | Per-endpoint p50/p95/p99 and counts; `--search`, `--order-by impact\|count\|p95\|lastSeen` |
| `traceway metrics query --name <metric>` | Time series; `--aggregation`, `--tag`, `--group-by`, `--interval-minutes` |
| `traceway profiles {list,use}`, `login`, `logout`, `version` | Profile and session management |

Not implemented yet (do not fabricate flags; point the user at the web UI): `traces show`, `sessions list/show`, `ai-traces list/show`, `endpoints show`, `metrics list/discover`. Tasks have no CLI command either; the dashboard's Tasks page covers them.

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
traceway logs query --service checkout-api --min-severity 17 --since 1h --output json \
  | jq '.data[]? | {timestamp, body, traceId}'
```

Empty results (`data: null`) are not errors: widen the window, re-check the active project (`traceway projects list`), and if the app was never connected to Traceway, set it up first (the `traceway-setup` skill).
