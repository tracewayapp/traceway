# Eval: backend-go-select-n-plus-1

- **ID**: `backend-go-select-n-plus-1`
- **Area**: backend
- **Framework**: Go (Gin + PostgreSQL)
- **Checklist item**: Select N+1 (`performance.md` → Database and data access)

## Telemetry snapshot

`traceway endpoints list --search "GET /api/orders" --since 24h --output json`:

```json
{ "endpoint": "GET /api/orders", "count": 48210, "p50": 412, "p95": 2380, "p99": 3110, "errorCount": 0 }
```

`traceway endpoints show <endpointId> --recorded-at <t>` — span waterfall of one
p95 request (total 2.31s):

```
GET /api/orders ............ 2310ms
└── db.query ............... 3ms   SELECT * FROM orders WHERE user_id = $1 LIMIT 50
└── db.query ............... 2ms   SELECT * FROM order_items WHERE order_id = $1
└── db.query ............... 2ms   SELECT * FROM order_items WHERE order_id = $1
... (347 more near-identical order_items spans, 1–3ms each)
└── json.encode ............ 41ms
```

`traceway metrics query --name system.cpu.utilization --aggregation max --since 24h`:
flat at 22–31% all day. p50/p95 have crept up ~15% per week for a month with
no deploy in that window (`git log` clean).

## Task

> `GET /api/orders` p95 is 2.4s and climbing ~15% a week with no recent
> deploy. Diagnose the bottleneck to a specific checklist cause and propose
> the fix.

## Expected diagnosis

- **Root cause**: Select N+1 — the handler loads 50 orders, then issues one
  `order_items` query per order (≈348 near-identical short spans); total DB
  time dominates while each call is tiny.
- **Key evidence**: repeated near-identical query spans in the waterfall;
  flat CPU (rules out saturation); latency creeping with data growth and no
  deploy (rules out a regression).
- **Fix**: single JOIN / batched `WHERE order_id IN (...)` load, or a
  dataloader.

## Pass criteria

- Names **Select N+1** (not "slow query" or "missing index" — the individual
  queries are 1–3ms, so neither fits).
- Cites the repeated spans *and* rules out saturation/regression from the
  flat CPU and clean deploy history.
- Proposes batching the item load.
