# Eval: infra-cache-stampede

- **ID**: `infra-cache-stampede`
- **Area**: infrastructure
- **Framework**: n/a (Redis-backed API)
- **Checklist item**: Cache stampede / thundering herd (`performance.md` →
  Caching and delivery)

## Telemetry snapshot

`traceway endpoints list --search "GET /api/homepage" --since 24h --output json`:

```json
{ "endpoint": "GET /api/homepage", "count": 210000, "p50": 95, "p95": 210, "p99": 4800, "errorCount": 0 }
```

`traceway endpoints chart --metric-type p99 --since 24h --interval-minutes 5`
for this endpoint: p99 is ~200ms all day except sharp 4–5s spikes at
**:00, :10, :20, :30, :40, :50** — exactly the 10-minute cache TTL boundary.

During a spike window, `traces show` reveals hundreds of near-identical
traces all recomputing the homepage payload (12 DB queries each) within the
same second; between spikes, the cache-hit metric reads 99%. The stampede
traces show no lock contention — every worker independently missed and
recomputed.

## Task

> `GET /api/homepage` p99 spikes to 5s exactly every 10 minutes, aligned to
> the cache TTL. Diagnose to a specific checklist cause.

## Expected diagnosis

- **Root cause**: Cache stampede / thundering herd — at each TTL expiry a
  burst of workers simultaneously miss and recompute the same payload.
- **Key evidence**: spikes phase-locked to the TTL boundary; burst of
  identical recompute traces; 99% hit ratio between spikes (rules out "no
  cache").
- **Fix**: probabilistic early refresh / request coalescing (singleflight)
  / staggered TTLs.

## Pass criteria

- Names **cache stampede** (not "no cache" — hit ratio is 99% between
  spikes; not "slow DB" — queries are fine, the burst is the problem).
- Cites the TTL-aligned periodicity as the decisive signal.
