---
name: traceway
description: 'Operate a Traceway observability instance through the traceway CLI: log in, query exceptions, logs, endpoints, and metrics, and debug production issues down to root cause. Use when the user invokes /traceway with a subcommand, e.g. "/traceway login", "/traceway debug issue <hash|url|title>", "/traceway what''s broken in prod", or whenever they want to investigate errors, crashes, slowness, or logs from an app monitored by Traceway.'
---
<!-- GENERATED FILE: assembled from cli/pkg/mcpserver/knowledge by cli/tools/skillgen. Edit the chunks there and run just gen-skills in cli/. -->

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
> If a `traceway` MCP server is connected, prefer its tools over shelling out to the CLI: they wrap the same API with the same semantics, and this skill's knowledge is available as its resources. The server is this same binary (`traceway mcp`).

{{include "ground-rules.md"}}

{{include "url-resolution.md"}}

{{include "timestamps.md"}}

{{include "notifications.md"}}

## Flow: Login

### 1. Check for an existing install

```bash
traceway version
```

If it prints a version, skip to authentication.

### 2. Install if missing

On POSIX systems (Linux, macOS), use the install script:

```bash
curl -fsSL https://cli.tracewayapp.com/install.sh | sh
```

If the script cannot be used, install manually: prebuilt binaries are on the [tracewayapp/traceway releases page](https://github.com/tracewayapp/traceway/releases) under `cli/vX.Y.Z` tags (the latest release may be a Backend release, so filter for CLI tags):

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

{{include "debug-flow.md"}}

{{include "performance-flow.md"}}

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
| `traceway logs query` | Logs; `--search` (`--search-type body\|attribute`), `--service`, `--min-severity <n>`, `--trace-id` |
| `traceway endpoints list` | Per-endpoint p50/p95/p99 and counts; `--search`, `--order-by impact\|count\|p95\|lastSeen` |
| `traceway endpoints chart` | Latency over time for the top endpoints; `--metric-type total_time\|p50\|p95\|p99`, `--interval-minutes`. Use to find when latency changed |
| `traceway endpoints slow <endpoint>` | Whether an operator marked this endpoint slow: `{offsetMs, reason}`. `offsetMs 0` = not marked. The offset is the accepted-latency baseline |
| `traceway endpoints show <id> --recorded-at <t>` | One request by id: span waterfall + linked errors |
| `traceway tasks show <id> --recorded-at <t>` | One background task run by id |
| `traceway ai-traces show <id> --recorded-at <t>` | One AI trace by id + its conversation |
| `traceway sessions show <id> --started-at <t>` | One session by id + the exceptions that fired in it |
| `traceway traces show <id> --recorded-at <t>` | Distributed trace: every service node sharing the id |
| `traceway metrics query --name <metric>` | Time series; `--aggregation`, `--tag`, `--group-by`, `--interval-minutes` |
| `traceway profiles {list,use}`, `login`, `logout`, `version` | Profile and session management |

The by-id `show`/`occurrence` commands take their id from a dashboard URL, a notification, or an `exceptions show` occurrence — and **require** the record's timestamp (`--recorded-at` / `--started-at`); see "Fast by-id lookups" above.

Not implemented yet (do not fabricate flags; point the user at the web UI): `list` verbs for `tasks` / `sessions` / `ai-traces` / `traces` (only by-id `show` exists for those), and `metrics list/discover`.

{{include "query-recipes.md"}}
