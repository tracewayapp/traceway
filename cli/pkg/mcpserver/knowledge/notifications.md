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
