---
name: integration-test
description: Run a live-instance verification of traceway-cli that goes beyond the Go smoke suite — exercises real-data detail endpoints, list→drill-in chains (tasks, sessions), discovery-driven metric probing, TTY-default rendering, and emits a human-readable coverage report. Invoke ONLY when the user explicitly asks (e.g. "run integration tests", "verify the CLI against stormwind"). Never invoke automatically after edits or commits. Assumes the user is already authenticated and a default project is configured.
---

# integration-test — traceway-cli

A repeatable protocol for verifying the CLI end-to-end against a live Traceway server, focused on the things the Go smoke suite can't easily cover.

## Trigger

Invoke ONLY when the user explicitly asks. Never run as a side effect of edits, commits, or builds.

## Relationship to the Go smoke suite

The `just smoke-test` target (`test/smoke/*_test.go`, build tag `smoke`) is the primary regression check. Against a live instance it already covers:

- JSON shape of every list endpoint and `profiles list`.
- Output-format coverage (json/table/yaml) for projects, profiles, exceptions, endpoints, logs.
- Client-side enum validation (`--search-type`, `--order-by`, `--sort-direction`, `--aggregation`, `--tag`).
- Time-range parsing edges (`--since 7D`, missing `--to`, `--since` + absolute mix, far-future windows).
- `metrics query` missing `--name`, bogus name → empty series, malformed tag.
- `exceptions show <zeros>` → exit 5 `not_found`.
- `--profile no-such-profile` → exit 4; `--project 0…0` → non-zero, no panic, no `connection_failed`.

**Do not re-implement these here.** If they regress, that's a Go-test bug, not a skill failure.

What this skill adds beyond `just smoke-test`:

1. **Real-data detail endpoints** — `exceptions show <captured-hash>`, populated `metrics query --name <real>` with every aggregation + group-by.
2. **Discovery-driven metric probing** — `metrics list` / `metrics tags` feed real names and tag keys into `metrics query`.
3. **List → drill-in chains on real data** — `tasks list` → `tasks runs --task` → `tasks show`; `sessions list` → `sessions show`.
4. **TTY-vs-pipe default** — table rendering to a real terminal.
5. **Coverage matrix report** — a human-readable artifact, on demand.
6. **Safety doctrine** — the forbidden-verb blocklist and `confirmMutation` env hygiene, applied to every probe.

## Hard constraints

**Read-only. No exceptions.** Even if a subcommand looks safe by name, check `--help` for mutating flags before running.

### Forbidden verbs and flags

Skip any subcommand whose name or `--help` mentions:

- `archive`, `unarchive`, `resolve`, `unresolve`, `mute`, `ack`, `acknowledge`
- `create`, `delete`, `update`, `set`, `put`, `post`, `add`, `remove`, `rm`
- `assign`, `claim`, `close`, `reopen`
- `login`, `logout`, `token`, `rotate`, `regenerate`
- `--archive`, `--resolve`, `--delete`, `--write`, `--mutate`, `--apply`, `--commit`

If a new subcommand is ambiguous (`sync`, `refresh`, `replay`, `export`), do not run it — list it under "skipped — manual review". If `--dry-run` exists, **still skip** write-shaped subcommands.

### Mutation safeguards

The CLI gates mutations via `confirmMutation` (`cmd/traceway/querycommon.go`). The harness MUST:

- Never pass `--yes`.
- `unset TRACEWAY_ASSUME_YES` at the top of the script.
- Run with stdin from `/dev/null`.

So that if a forbidden verb slips through, the gate refuses with exit 2 `usage_error` instead of hanging on a prompt.

## Pre-flight

Run in order. Stop if any fails.

1. Build: `nix develop --command go build -o ./bin/traceway ./cmd/traceway`.
2. Config exists (don't print — JWT inside): `test -f "${XDG_CONFIG_HOME:-$HOME/.config}/traceway/config.json"`.
3. Reachability + capture `TW_PROJECT_ID`:
   ```bash
   ./bin/traceway projects list --output json | jq -e 'type=="array" and length>=1' >/dev/null
   TW_PROJECT_ID=$(./bin/traceway projects list --output json | jq -r '.[0].id')
   ```

`projects list --output json` returns a **bare array**, not a `{data, pagination}` envelope.

## Detail-endpoint probes

### `exceptions show <captured-hash>`

1. Capture a real hash:
   ```bash
   HASH=$(./bin/traceway exceptions list --since 720h --page-size 1 --output json | jq -r '.data[0].exceptionHash // empty')
   ```
   If empty, retry against other projects via `--project <id>`. If still empty, skip with reason `no exception found across all projects`.
2. With a real hash: three output formats + `--help`. JSON shape: `{group: {...}, occurrences: [...], pagination: {...}}` — assert `.group and .occurrences`.
3. Capture `.occurrences[0].traceId` if present for the logs probe below.

### `metrics list` / `metrics tags` → `metrics query` (discovery-driven)

Discover instead of guessing:

```bash
MJSON=$(./bin/traceway metrics list --output json)                 # server-default 7d window
MNAME=$(jq -r 'first(.metrics[] | select(.tagKeys | length > 0) | .name) // .metrics[0].name // empty' <<<"$MJSON")
MKEY=$(jq -r "first(.metrics[] | select(.name == \"$MNAME\") | .tagKeys[0]) // empty" <<<"$MJSON")
```

If `metrics list` is empty for every project, skip the live block with reason `no metrics discovered`.

Probe `metrics list` itself (json/table, `--since 24h`, `--search`), then `metrics tags $MNAME` (keys form), `metrics tags $MNAME $MKEY` (values form, expect ≥1 value), and `metrics tags no.such.metric.zzz` → exit 5 `not_found`.

For the discovered metric:

- Three output formats + `--help`.
- All aggregations: `avg`, `sum`, `count`, `min`, `max`, `p50`, `p95`, `p99`.
- `--interval-minutes 15`.
- `--group-by $MKEY` when a tag key was discovered (splits `__all__` into per-value series).

JSON shapes: `metrics list` → `{metrics: [{name, tagKeys, metricType?, unit?}]}`; `metrics tags <n> <k>` → `{values: [...]}`; `metrics query` → `{results: [{name, unit, series: {<tag-key>: [{timestamp, value}, ...]}}]}` — `series` is a map keyed by group tag, default key `__all__`.

### `logs query --trace-id <captured>` + new filter flags

If a real trace id was captured above, run `logs query --trace-id $TRACE --since 720h`. Assert exit 0 and `{data, pagination}` shape. **Empty results (`data: null`, `total: 0`) with exit 0 are a pass** — assert the envelope, not row counts, unless the probe seeded its own capture.

Also probe (no captured data needed for the error cases):

- `--min-severity error` and `--min-severity 17` → both exit 0; `--min-severity severe` → exit 2 `usage_error`. Sanity-check monotonicity when data exists: `total` at `trace` ≥ `warn` ≥ `error`.
- `--attr`: capture one real key from a log record's `resourceAttributes`, then `--attr "resource:$KEY=$VAL"` → expect ≥1 row (it matched itself). `--attr "span:k=v"` → exit 2 (bad scope).
- `--distributed-trace-id $DT` (from the occurrence, when captured) → exit 0, envelope shape. `--distributed-trace-id not-a-uuid` → exit 2. `--exclude-trace-id` without `--distributed-trace-id` → exit 2.

### `tasks list` → `tasks runs` → `tasks show` (chain on real data)

1. `tasks list --since 720h` (json/table/yaml; `--order-by p95`; `--root-filter root`; bogus `--order-by p95_duration` → exit 2 — the CLI takes camelCase names, not the wire's snake_case). Shape: `{data: [{taskName, count, p50Duration, ...}], pagination}`. If empty across all projects, skip the chain with reason `no tasks found`.
2. Capture `TASKNAME` from the first row. `tasks runs --task "$TASKNAME"` → assert `.stats.count >= 1` and every row's `taskName` matches; table form prints the stats block (THROUGHPUT line). Plain `tasks runs` (no `--task`) → rows without stats.
3. Capture `.data[0].id` + `.data[0].recordedAt` from the runs output and feed them straight into `tasks show <id> --recorded-at <ts>` → assert `.task.id` echoes.

### `sessions list` → `sessions show`

`sessions list --since 720h` (json/table; `--order-by duration`). Shape: `{data: [{id, startedAt, ...}], pagination}`. Feed `.data[0].id` + `.data[0].startedAt` into `sessions show <id> --started-at <ts>` → assert `.session` present. Skip with reason `no sessions found` when empty everywhere.

### `ai-traces list`

`ai-traces list --since 720h` (json/table; `--order-by totalTokens`). Shape: `{data: [{traceName, count, totalTokens, totalCost, ...}], pagination}`. Skip with reason `no AI traces found` when empty everywhere.

### By-id detail commands (captured id + recordedAt)

These all **require** a timestamp flag; capture the id and its `recordedAt` together from `exceptions show`, then exercise them. All read-only.

1. Capture one occurrence's id + recordedAt (+ optional trace/session ids):
   ```bash
   OCC=$(./bin/traceway exceptions show "$HASH" --output json | jq -c '.occurrences[0]')
   OID=$(jq -r '.id'                 <<<"$OCC")
   OTS=$(jq -r '.recordedAt'         <<<"$OCC")
   DT=$(jq -r '.distributedTraceId // empty' <<<"$OCC")
   SID=$(jq -r '.sessionId // empty'         <<<"$OCC")
   ```
2. `exceptions occurrence $OID --recorded-at $OTS` — assert exit 0 and `.exception.id == $OID`.
3. **Required-flag enforcement** (no live data needed): `exceptions occurrence $OID` with no `--recorded-at` → exit 2 `usage_error`; `--recorded-at notadate` → exit 2 `invalid_timestamp`; `endpoints show not-a-uuid --recorded-at $OTS` → exit 2 `usage_error`.
4. If `$DT` is non-empty: `traces show $DT --recorded-at $OTS` → exit 0, `.nodes` is an array. If `$SID` is non-empty: `sessions show $SID --started-at $OTS` → exit 0, `.session` present.
5. `endpoints show` / `tasks show` / `ai-traces show` need an id of their own type — capture one from a `traces show` node when available (`.nodes[].endpoint.id` + `.endpoint.recordedAt`, etc.); otherwise skip with reason `no <type> id captured`.

## TTY-vs-pipe default

If `script` is available:
```bash
script -q /dev/null ./bin/traceway projects list | head -20
```
Expect a table. Piping without `script` should yield JSON. Mark as "not verified" if `script` is absent.

## Subcommand skip lists

Mutating (skip with reason `forbidden verb`):
`exceptions archive`, `exceptions unarchive`, `login`, `logout`, `profiles use` (local mutation), `projects use` (local mutation of `state.json`).

Not in the CLI (skip with reason `subcommand not in CLI`) — kept so the report shows the gap if it ships:
`traces list`.

## Observation and reporting

Classify each invocation:

| Result | Classification |
|---|---|
| exit 0, valid JSON, expected keys present | **pass** |
| exit 0 but stdout empty when data expected, or JSON invalid / missing keys | **fail — schema** |
| exit non-zero, clean message, expected error | **pass — error case** |
| exit non-zero with panic or stack trace | **fail — crash** |
| exit non-zero on a happy path | **fail — unexpected error** |
| stderr non-empty on a passing command | **warn — noisy** |
| > 10s on a list call | **warn — slow** |

Drive the run from one ephemeral `/tmp/*.sh` script (not committed). Top of script:

```bash
#!/usr/bin/env bash
set -u                            # never set -e
unset TRACEWAY_ASSUME_YES
exec </dev/null                   # no TTY for the suite
LOG=$(mktemp /tmp/traceway-it.XXXXXX.log)
```

Capture exact invocation, exit code, and first 20 lines of stdout/stderr per probe to `$LOG`.

End-of-run markdown report:

1. **Summary**: `N pass, M fail, W warn, S skipped` + wall-clock.
2. **Failures table**: command, classification, one-line excerpt.
3. **Warnings table**: same shape.
4. **Skipped table**: command + reason.
5. **Coverage matrix**: dimensions exercised per probed command (`—` for not exercised).
6. **Smoke-suite pointer**: note whether `just smoke-test` was run in this session and its result, so the report stands alone.
7. **Log path**: location of `$LOG`.

Do not inline the full log.

## What this skill does not do

- It does not duplicate `just smoke-test`. Assume that suite's coverage is green; if you suspect regressions there, run smoke separately.
- It does not write or update the CLI's Go test files.
- It does not perform any login, token rotation, profile creation, or credential mutation.
- It does not gate commits or CI.

## When to expand

If a new read subcommand ships, add a probe block. If a new mutating subcommand ships, add it to the forbidden list. If a regression pattern shows up that is deterministic and stateless, push it down to `test/smoke/*_test.go` — not into this skill.
