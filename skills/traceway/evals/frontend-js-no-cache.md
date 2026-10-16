# Eval: frontend-js-no-cache

- **ID**: `frontend-js-no-cache`
- **Area**: frontend
- **Framework**: browser JS (React SPA)
- **Checklist item**: No cache / low hit ratio (`performance.md` → Caching and delivery)

## Telemetry snapshot

`traceway endpoints list --search "GET /api/products" --since 24h --output json`:

```json
{ "endpoint": "GET /api/products", "count": 388000, "p50": 640, "p95": 980, "p99": 1400, "errorCount": 0 }
```

The product catalog changes at most twice a day, yet the endpoint is hit on
*every* page navigation — 388k hits/day for ~40k daily users. The
`cache.hit_ratio` metric (reported by the CDN edge) sits at 4%. Waterfall of
one request is unremarkable: 380ms DB + 120ms serialize, no single slow span.

`traceway endpoints chart --metric-type total_time --since 7d` shows total
server time for this endpoint is 3× the next endpoint — it is the fleet's
biggest cost center by volume, not by per-request latency.

## Task

> `GET /api/products` per-request latency looks fine, but it is the fleet's
> biggest total-time cost center. Diagnose to a specific checklist cause.

## Expected diagnosis

- **Root cause**: No effective cache — a twice-daily-changing catalog is
  fetched on every navigation with a 4% edge hit ratio; the DB is hit on
  every request.
- **Key evidence**: 388k hits vs 40k users; 4% hit ratio; total_time
  dominance without per-request slownness.
- **Fix**: edge-cache the catalog response with a 6h TTL + surrogate-key
  purge on catalog publish; or `stale-while-revalidate`.

## Pass criteria

- Names **no cache / low hit ratio** (not "slow query" — per-request spans
  are fine; the problem is volume × repetition).
- Uses the hit-ratio metric and the hits-per-user ratio, not just latency.
