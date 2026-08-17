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
traceway metrics query --name system.cpu.utilization --aggregation max --since 24h
traceway metrics query --name <name> --aggregation avg|sum|count|min|max [--tag key=value] [--group-by <tag>]
```

The CLI also accepts `p50|p95|p99`, but the server has no quantile aggregation for metric points and silently computes `avg` for them — never present those as percentiles. Latency percentiles come from `traceway endpoints list`, computed from raw request durations. There is no `metrics list`; a bogus name returns an empty `series: {}` cleanly, so probing names is safe. Host metrics from the Traceway OTel Agent live under `system.*` names, and OTLP histogram metrics are stored as two series, `<name>.avg` and `<name>.count`.

### 4. Correlate with the code

1. Open the files and lines named in the stack trace and read the failing path.
2. If the issue started at a known time, check what shipped then: `git log --since "<firstSeen>" --until "<firstSeen + 1h>"` or the deploy history.
3. Form a hypothesis that explains the targeted occurrence's full observation set (its message, affected endpoint, timing, volume) — not just the first stack frame. When a URL/hash anchored you to the last occurrence (step 2), explain *that* error; do not stretch one hypothesis to cover sibling clusters that merely share the hash.
4. Propose or implement the fix per the user's instruction, scoped to the targeted occurrence's failure path.

### 5. When the evidence is not enough, instrument instead of guessing

Before writing a fix, answer one question honestly: **does the telemetry actually explain this failure?** You have enough when you can name the failing line, the input that reached it, and why that input was wrong. A stack trace on its own is usually not enough. Neither is a plausible story that the data neither confirms nor contradicts.

If you cannot explain it, do not ship a speculative fix. A guess that changes behaviour hides the symptom, destroys the next reproduction, and costs another incident cycle. Instead, add the instrumentation that would answer the open question, and let the next occurrence do the diagnosing.

Pick the signal that closes the specific gap:

| What you could not determine | What to add |
|---|---|
| Which branch ran, or what the inputs were | A log at the decision point with the deciding values as attributes. Use ERROR severity on the failure path so `--min-severity 17` finds it |
| Where the time went, or which step failed | Child spans around each suspect step, opened inside the request's active context |
| Who or what it happened to (user, tenant, release, payload shape) | Attributes on the request span, or attributes passed to `recordException` |
| How often it happens, or under which conditions | A counter or histogram metric, tagged with the low-cardinality dimension you want to slice by |
| Where it broke across a service boundary | Trace context propagation on the call, plus a span or log on both sides |
| Whether the code path runs at all | A log or counter at entry, so absence of data becomes evidence |

Rules for what you add:

- Emit it inside the active span of the request or task. That is what stamps the trace id on the log and makes it show up on the trace.
- Put identifiers in attributes, never in the message text. Messages are stripped before grouping, and attributes are filterable.
- Keep cardinality low. User ids and request ids belong in span attributes, never in a metric tag or a span name.
- Never log secrets, tokens, or personal data.
- Ship it as permanent instrumentation, not a temporary debug print. Something you remove tomorrow answers nothing.
- If the area has no instrumentation at all, set it up first with the `traceway-setup` skill rather than bolting on one log line.

Then close the loop with the user. State what you could not determine, what you added, and the exact command to run when it fires again:

```bash
traceway logs query --trace-id <id> --output json
traceway exceptions show <hash> --output json | jq '.occurrences | last | .attributes'
```

Adding instrumentation is a legitimate outcome of a debug session, not a failure to fix. Report it as the deliverable it is.

### 6. Report and clean up

Summarize: symptom, evidence (exception hashes, log excerpts, metric anomalies), root cause, fix. Include `traceway exceptions show <hash>` references so the user can verify. After a fix is deployed and verified, archive only when the user asks:

```bash
traceway exceptions archive <hash> --yes
```
