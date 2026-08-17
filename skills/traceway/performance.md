<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/performance.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Performance debugging

Authoritative reference for the **Performance** flow. Approach the investigation as a senior software engineer with deep performance-debugging experience: do not guess at fixes, quantify first, localize the cost to a specific span, confirm the cause against the checklist below, and only then propose a change. Every suspect here is paired with the Traceway signal that confirms it, so the checklist is a lookup table from "what the waterfall shows" to "what is actually wrong".

All commands are read-only. Never archive or mutate anything during a performance investigation. For exact CLI syntax, the mandatory `--recorded-at` / `--started-at` timestamp rules, and dashboard-URL resolution, the parent `SKILL.md` is authoritative (over MCP: the `traceway://knowledge/timestamps` and `traceway://knowledge/url-resolution` resources).

## The investigation loop

1. **Quantify and localize.** Find the slow endpoint(s) and read the shape of the latency:
   ```bash
   traceway endpoints list --since 24h --order-by p95 --page-size 10 --output json \
     | jq '.data[]? | {endpoint, count, p50, p95, p99, errorCount}'
   ```
   Read p50 vs p99 before anything else (see "Reading the latency shape").

2. **Get a representative slow trace.** Drill into one slow request to see where the time goes. Capture an id and its `recordedAt` together, then:
   ```bash
   traceway endpoints show <endpointId> --recorded-at <t>          # span waterfall for one request
   traceway traces show <distributedTraceId> --recorded-at <t>     # cross-service timeline
   ```
   `traces show` is the highest-value call when more than one service is involved: it stitches the whole logical request together end to end.

3. **Find the long pole and match it to the checklist.** In the waterfall, identify the single span (or repeated group of spans) that dominates the duration. Classify it: is it a database call, an external/upstream call, in-process compute, or a *gap* before work starts (which means queueing, lock wait, or connection-pool exhaustion, not the work itself)? Then look it up in the checklist.

4. **Separate code from saturation.** A slow endpoint can be a code bug or an overloaded host. Pull infrastructure and runtime metrics over the same window:
   ```bash
   traceway metrics query --name system.cpu.utilization --aggregation max --since 24h
   traceway metrics query --name mem.used --aggregation max --since 24h
   traceway metrics query --name go.gc_pause --aggregation max --since 24h
   ```
   A metric spike that lines up with the latency rise points at saturation; flat metrics under rising latency point at a code or query problem.

5. **Correlate with code and deploys.** Open the slow path in the source and read it. If p95 climbed at a specific time, check what shipped then (`git log --since ... --until ...` or deploy history). Latency that jumped at a release is a regression; latency that crept up without one is usually data growth.

6. **Report.** State the bottleneck, the evidence (endpoint percentiles, the dominating span, any correlated metric), the root cause mapped to a specific checklist item, and a concrete fix. Include the `endpoints show` / `traces show` references so the user can verify.

## Reading the latency shape

The relationship between percentiles, load, and time tells you which family of causes to look at before you open a single trace:

- **p50 already high** (the median request is slow): every request pays the cost, so it is systemic. Suspect an inefficient query, a missing index, an O(n^2) hot path, or synchronous work that should be backgrounded.
- **p99 much greater than p50** (median fine, tail slow): a minority of requests stall. Suspect contention, connection-pool exhaustion, GC pauses, retries, cache stampedes, or a flaky upstream dependency.
- **Latency that grew over weeks with no deploy**: data-volume driven. Suspect Select N+1, a missing index, or an unbounded result set, all of which scale with table size.
- **Latency that jumped at a deploy**: a regression. Correlate `firstSeen` / the climb time with `git log`.
- **Latency that tracks request volume**: saturation or contention. Confirm with infra metrics and pool-wait gaps in the waterfall.

## Account for user-marked slow endpoints

Some endpoints are *expected* to be slow: a CSV export, a heavy report, a third-party sync. An operator can mark such an endpoint slow with an allowance (`offsetMs`) and a `reason`. Check before you treat latency as a problem:

```bash
traceway endpoints slow "GET /api/reports/export" --output json   # {offsetMs, reason}
```

- `offsetMs 0` (or "MARKED: no"): default thresholds apply; judge latency normally.
- `offsetMs > 0`: the operator accepted that much latency. The apdex and impact thresholds are shifted up by the offset server-side, so the endpoint's `impact`/`impactReason` already discount it. Only latency *beyond* `offsetMs`, or a fresh jump above the prior baseline, is a real regression.

Two things to keep straight:

- **Raw percentiles are not offset-adjusted.** The p50/p95/p99 from `endpoints list` and `endpoints chart` come from raw durations; the offset only moves the apdex/impact thresholds. A marked-slow endpoint can show a high p95 that is entirely expected, so compare p95 against `offsetMs`, not against zero.
- **p95/p99 alert rules are not offset-aware.** `endpoint_p95_threshold` / `endpoint_p99_threshold` notifications fire on raw latency regardless of marking, so a marked-slow endpoint can still page on latency its operators accepted. `apdex_drop` and `impact_score_*` rules are offset-aware. (See "Performance notification" in `SKILL.md`.)

When you report, state the p95 and the allowance together: "p95 is 2.1s but this endpoint is marked slow with a 2s allowance (reason: nightly export), so it is within baseline", or "p95 is 4s against a 2s allowance, so it regressed past the accepted offset".

## Pinpointing when the slowness started

Narrowing the window is how you turn "it is slow" into "it got slow at 14:10, right after deploy X". Do not investigate the default window blindly: adjust it down until it brackets the onset, then correlate that moment with deploys and metrics.

**1. Latency over time: `endpoints chart`.** This is the direct curve. It buckets latency over the window and returns the top 5 endpoints ranked by the chosen metric (plus an "Other" bucket), each as a `{timestamp, endpoint, value}` series in milliseconds. Read down one endpoint's buckets for the step where its value jumps: that bucket dates the onset. Use `--metric-type p95` (or `p99`/`p50`/`total_time`) and a coarse `--interval-minutes` over a wide window first, then re-run the suspect span with a finer interval to sharpen.
```bash
traceway endpoints chart --metric-type p95 --since 7d --interval-minutes 60 --output json \
  | jq '.series[] | select(.endpoint == "GET /api/checkout") | {t: .timestamp, p95_ms: .value}'
```
Caveat: it always returns the *top 5* endpoints by that metric. If your suspect route is not in the top 5 it is folded into "Other"; fall back to method 3 for it.

**2. Infra and runtime over time: `metrics query --interval-minutes`.** For the *cause* behind a latency step (saturation, GC, throughput), bucket any metric the same way. The returned `series` is a list of `{timestamp, value}` points; read down them for the bucket where the value steps up.
```bash
traceway metrics query --name system.cpu.utilization --aggregation max --since 7d --interval-minutes 60 --output json \
  | jq '.series | to_entries[] | .value[] | {t: .timestamp, v: .value}'
```
A flat line that steps up at the same bucket the latency jumped is the saturation signal; flat metrics under a latency step point at code.

**3. A specific non-top-5 endpoint: window bisection.** When the route you care about is in "Other", reconstruct its onset from `endpoints list` (the only source of true p50/p95/p99, computed from raw durations) by comparing the *same endpoint's* percentiles across adjacent windows with `--from`/`--to`. Start wide, split, keep the half where p95 is elevated, recurse until the bracket is tight:
```bash
# is the regression in the first or second half of the suspect day?
traceway endpoints list --search "GET /api/checkout" --from 2026-06-25T00:00:00Z --to 2026-06-25T12:00:00Z \
  --output json | jq '.data[0] | {p95, p99, count}'
traceway endpoints list --search "GET /api/checkout" --from 2026-06-25T12:00:00Z --to 2026-06-26T00:00:00Z \
  --output json | jq '.data[0] | {p95, p99, count}'
```
A few bisection steps localize the jump to an hour or less. Watch `count` alongside p95: if the slow half also carries far more traffic, suspect load, not a code change.

**4. Cross-check and correlate.** Once the methods agree on an onset time:
- A latency jump that coincides with a metric inflection (CPU or GC stepping up at the same bucket) points at saturation.
- A latency jump with flat infra metrics points at a code or query regression: `git log --since '<onset - 30m>' --until '<onset + 30m>'` or the deploy history for what shipped then.
- A latency curve that ramps gradually over days with no single step is data growth (N+1, missing index, unbounded result set), not a deploy.

Then pass the narrowed `--from`/`--to` back into the by-id detail lookups so `--recorded-at` lands inside the slow window and the trace you pull is actually from the slow period, not a fast one before the regression.

## Bottleneck checklist

Each row: the suspect, and how to confirm it from Traceway telemetry.

### Database and data access
| Suspect | How to confirm in Traceway |
|---|---|
| **Select N+1** | Many repeated, near-identical short query spans inside one request in the span waterfall; total DB time dominates while each call is tiny. |
| **Missing or unused index** | One query span is the long pole; its duration grows with table size; same query slow across many traces. |
| **Over-fetching (`SELECT *` / no projection)** | Large query-result spans, high memory; latency tracks row width/count rather than logic. |
| **No pagination / unbounded result set** | p50 has crept up over time with data growth; one query returns more rows every week. |
| **Expensive JOIN or aggregation** | A single query span dominates the trace consistently, independent of load. |
| **Connection-pool exhaustion** | A gap *before* the first DB span (time waiting to acquire a connection); p99 rises with concurrency while p50 stays flat. |
| **Lock / transaction contention** | DB spans queue behind each other; tail latency appears under write load; correlated WARN/ERROR locks in `logs query`. |
| **Missing query/result caching** | The same query runs on every request; high DB-call count per endpoint with low cardinality of inputs. |

### Application and compute
| Suspect | How to confirm in Traceway |
|---|---|
| **Synchronous work that belongs in a background task** | A long handler span runs before the response returns, for work the user does not wait on (emails, exports, image processing). It should be a `CaptureTask`. |
| **CPU-bound serialization** | A compute gap between IO spans (large JSON encode/decode); `system.cpu.utilization` up during the request. |
| **Inefficient algorithm / O(n^2) hot path** | Latency scales superlinearly with input size across traces; no IO to blame in the waterfall. |
| **Unbounded in-memory work / large allocations** | `mem.used` and `go.num_gc` climb with p99; large compute spans. |
| **Excessive logging / telemetry in the hot path** | Many log lines per request in `logs query`; logging spans visible in the waterfall. |

### Network, IO, and external dependencies
| Suspect | How to confirm in Traceway |
|---|---|
| **Chatty IPC / microservice fan-out** | `traces show` reveals many sequential cross-service hops, each adding round-trip latency. |
| **Slow third-party / upstream API** | An external span dominates the trace; p99 spikes correlate with the provider; not your code. |
| **Sequential calls that could run in parallel** | A staircase waterfall: each span starts only when the previous ends, though they are independent. |
| **No timeouts / aggressive retries** | Long-tail p99; retry bursts in `logs query`; the same downstream span repeated within one trace. |
| **No connection reuse (DNS/TLS per call)** | Many short external spans each paying a handshake; per-request connection setup cost. |
| **Large request/response payloads** | Transfer-dominated spans; latency correlates with payload size. |

### Caching and delivery
| Suspect | How to confirm in Traceway |
|---|---|
| **No cache / low hit ratio** | DB or upstream is hit on every request; cache-hit metric (if reported) is low. |
| **Cache stampede / thundering herd** | Latency spikes at cache-expiry boundaries; a correlated burst of identical traces. |
| **Cold start / JIT / connection warmup** | First-request-after-deploy outliers; latency normalizes after warmup; `firstSeen`-correlated. |

### Infrastructure and saturation
| Suspect | How to confirm in Traceway |
|---|---|
| **CPU saturation** | `system.cpu.utilization` (max) spike lines up with the latency rise across many endpoints at once. |
| **Memory pressure / GC pauses** | `mem.used`, `go.gc_pause`, `go.num_gc` correlate with p99; pauses visible as gaps in the waterfall. |
| **Disk / IO bottleneck** | `system.disk.*` or IO-wait metrics up; slow IO spans with no CPU cost. |
| **Undersized / noisy-neighbor instance** | Broad latency rise with no single span culprit; infra metrics elevated with no code change. |

## Notes on the signals

- Host and runtime metrics live under `system.*` (Traceway OTel Agent) and the SDK runtime names (`go.gc_pause`, `mem.used`, `go.go_routines`). The server has no quantile aggregation for metric points: `p50/p95/p99` on `metrics query` silently return `avg`, so never present those as percentiles. Real latency percentiles come only from `endpoints list`, computed from raw request durations.
- OTLP histogram metrics are stored as two series, `<name>.avg` and `<name>.count`. A bogus metric name returns an empty `series: {}` cleanly, so probing names is safe.
- When the waterfall shows a gap with no span, that gap is the finding: it is time the request spent waiting (for a connection, a lock, the scheduler), not time spent doing work.
