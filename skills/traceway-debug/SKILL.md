---
name: traceway-debug
description: Investigate a bug or production issue using observability data from a Traceway instance — exceptions, logs, endpoint stats, and metrics queried via the traceway CLI, correlated with the codebase. Use when the user describes a bug, error, crash, slowness, or anomaly and a Traceway instance is monitoring the affected app, e.g. "/traceway-debug users report 500s on checkout since this morning".
---

# Debug with Traceway

Investigate a described bug using telemetry from a Traceway instance, then correlate findings with the codebase to find the root cause.

## Prerequisites

The `traceway` CLI must be installed, authenticated, and pointed at the right project:

```bash
traceway version                 # installed?
traceway projects list           # authenticated? right instance?
traceway projects use <id>       # select the project for the affected app
```

If the CLI is missing, install it first (see the `traceway-install-cli` skill, or https://github.com/tracewayapp/traceway/releases). If authentication fails (exit code 4), ask the user to run `traceway login --url https://<instance>`.

**CLI behavior for agents:** piped output defaults to JSON (one record per line); `--fields a,b,c` trims responses; errors emit `{"error":"<stable_id>","message":"...","hint":"...","exit_code":N}` on stderr. Time ranges: `--since 1h|24h|7d` or `--from/--to` (RFC3339). All list commands paginate with `--page` / `--page-size` (default 50).

## Step 1: Frame the Investigation

From the user's bug description, extract: the symptom (error, wrong behavior, slowness, crash), the affected endpoint/feature, and the time window. Default to `--since 24h` if no timeframe was given; widen later if needed.

## Step 2: Look for Exceptions

Most bugs surface as grouped exceptions (Issues):

```bash
# Recent exception groups, most impactful first
traceway exceptions list --since 24h

# Search for terms from the bug description (error message, type, file)
traceway exceptions list --since 24h --search "checkout" 
traceway exceptions list --since 24h --search "NullPointer" --search-type regex

# Sort by what matters: lastSeen (default), firstSeen (regressions), count (volume)
traceway exceptions list --since 7d --order-by firstSeen

# Full detail for a group: stack trace, occurrences, tags
traceway exceptions show <hash>
```

`exceptions show` is the high-value call — it returns the full stack trace and occurrence tags (user IDs, app versions, request context). Use `firstSeen` to correlate with deploys: a group that first appeared right after a release points at that release's diff.

## Step 3: Query Logs Around the Failure

```bash
# Errors and worse, in the window
traceway logs query --since 24h --min-severity 17

# Search log bodies for terms from the bug report
traceway logs query --since 24h --search "payment declined"

# Narrow by service in multi-service projects
traceway logs query --since 24h --service checkout-api --min-severity 13
```

Severity numbers are OTel-standard: 1=TRACE, 5=DEBUG, 9=INFO, 13=WARN, 17=ERROR, 21=FATAL.

**Correlate by trace:** if a log record or exception includes a trace ID, pull every log line from that exact request:

```bash
traceway logs query --since 24h --trace-id <trace-id>
```

This reconstructs the request timeline — usually the fastest route to a root cause.

## Step 4: Check Endpoint Health (for Slowness / Error Rates)

```bash
# Per-endpoint p50/p95/p99, error counts — sorted by impact
traceway endpoints list --since 24h

# Find the affected endpoint
traceway endpoints list --since 24h --search "checkout"

# Worst latency first
traceway endpoints list --since 24h --order-by p95
```

Compare windows to find when a regression started, e.g. `--since 1h` vs `--since 7d`, or two explicit `--from/--to` ranges around a suspected deploy.

## Step 5: Check Metrics (for Resource / Systemic Issues)

```bash
# Latency over time
traceway metrics query --name http.server.duration --aggregation p95 --since 24h

# Resource saturation (Go SDK default metrics)
traceway metrics query --name cpu.used_pcnt --aggregation avg --since 24h
traceway metrics query --name mem.used_pcnt --aggregation max --since 24h

# Group a metric by tag, filter by tag
traceway metrics query --name http.server.duration --aggregation p95 --group-by endpoint --since 6h
traceway metrics query --name queue.depth --aggregation max --tag queue=email --since 6h
```

Aggregations: `avg, sum, count, min, max, p50, p95, p99`. Use `--interval-minutes` to control bucket size (0 = auto). Spikes that line up with the exception's `firstSeen` time confirm a systemic cause (OOM, CPU saturation, downstream slowness) rather than a code bug.

## Step 6: Correlate with the Code

With a stack trace, trace timeline, or regression window in hand:

1. Open the files/lines named in the stack trace and read the failing path.
2. If the issue started at a known time, check what shipped then: `git log --since "<firstSeen>" --until "<firstSeen + 1h>"` or the deploy history.
3. Form a hypothesis that explains **all** observations (error message, affected endpoint, timing, volume) — not just the first stack frame.
4. Propose or implement the fix per the user's instruction.

## Step 7: Report and Clean Up

Summarize: symptom → evidence (exception hashes, log excerpts, metric anomalies) → root cause → fix. Include `traceway exceptions show <hash>` references so the user can verify.

After a fix is deployed and verified, the exception group can be archived — this mutates server state, so only do it when the user asks:

```bash
traceway exceptions archive <hash> --yes
```

## If There Is No Telemetry

Empty results usually mean the wrong project, wrong time window, or the app is not instrumented. Check `traceway projects list`, widen `--since`, and if the app was never connected to Traceway, set it up first (see the `traceway-setup-project` skill).
