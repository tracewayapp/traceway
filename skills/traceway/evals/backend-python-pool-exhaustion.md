# Eval: backend-python-pool-exhaustion

- **ID**: `backend-python-pool-exhaustion`
- **Area**: backend
- **Framework**: Python (FastAPI + PostgreSQL, SQLAlchemy pool size 20)
- **Checklist item**: Connection-pool exhaustion (`performance.md` → Database and data access)

## Telemetry snapshot

`traceway endpoints list --search "POST /api/checkout" --since 24h --output json`:

```json
{ "endpoint": "POST /api/checkout", "count": 96400, "p50": 118, "p95": 940, "p99": 8210, "errorCount": 12 }
```

p50 flat all day; p99 climbs steeply during the 12:00–14:00 and 19:00–21:00
traffic peaks and collapses between them.

`traceway endpoints show <endpointId> --recorded-at <t>` — one p99 request
(total 8.04s):

```
POST /api/checkout ....... 8040ms
├── <gap: no spans> ...... 3120ms
└── db.query ............. 44ms   SELECT ... FROM inventory WHERE sku = $1
└── db.query ............. 61ms   UPDATE orders SET ...
└── http.post ............ 280ms  POST https://payments.example.com/charge
```

`traceway metrics query --name system.cpu.utilization --aggregation max --since 24h`:
35–48%, no spike aligned with the p99 humps. `traceway logs query --search "pool"`:
`sqlalchemy.pool.impl.QueuePool: "QueuePool limit of size 20 overflow 10 reached, connection timed out"` WARN lines during both peaks.

## Task

> `POST /api/checkout` p50 is fine at 118ms but p99 hits 8.2s during traffic
> peaks. Diagnose the bottleneck to a specific checklist cause.

## Expected diagnosis

- **Root cause**: Connection-pool exhaustion — the 3.1s gap *before* the
  first DB span is time waiting to acquire a connection, not work; p99 rises
  with concurrency while p50 stays flat; pool-timeout warnings in the logs
  confirm it.
- **Key evidence**: the pre-DB gap in the waterfall; p99-vs-p50 divergence
  tracking traffic; `QueuePool limit reached` log lines.
- **Fix**: raise pool size / add overflow, shorten checkout transaction scope,
  or shed load (queue) at the peak.

## Pass criteria

- Names **connection-pool exhaustion** (not "slow DB" — the queries are
  44–61ms; not "slow upstream" — the payment call is 280ms).
- Points at the gap *before* the first DB span as the finding, and cites the
  p99/p50 divergence plus the pool log lines.
