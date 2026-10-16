# Eval: backend-missing-index

- **ID**: `backend-missing-index`
- **Area**: backend
- **Framework**: any (PostgreSQL)
- **Checklist item**: Missing or unused index (`performance.md` → Database and data access)

## Telemetry snapshot

`traceway endpoints list --search "GET /api/users/lookup" --since 24h --output json`:

```json
{ "endpoint": "GET /api/users/lookup", "count": 88100, "p50": 940, "p95": 1210, "p99": 1480, "errorCount": 0 }
```

`traceway endpoints show <endpointId> --recorded-at <t>` — one request
(total 1.02s):

```
GET /api/users/lookup ..... 1020ms
└── db.query ..............  980ms  SELECT id, name, email FROM users WHERE email = $1
└── json.encode ...........   12ms
```

The same query span dominates in *every* sampled trace, at all hours, at all
traffic levels. `endpoints chart --metric-type p50 --since 30d` shows p50
rising smoothly from 210ms to 940ms over two months; `git log` shows no
deploy touching this path; the `users` table grew from 2M to 9M rows in the
same window. `EXPLAIN` (from a DBA paste in the linked issue) shows
`Seq Scan on users`.

## Task

> `GET /api/users/lookup` p50 crept from 210ms to 940ms over two months with
> no deploy on that path. Diagnose the bottleneck to a specific checklist
> cause.

## Expected diagnosis

- **Root cause**: Missing index on `users(email)` — one query span is the
  long pole in every trace, its duration grows with table size, and the
  latency curve tracks data growth, not deploys.
- **Key evidence**: single dominating query span across all traces; smooth
  multi-week climb with no deploy; table growth 2M→9M rows; `Seq Scan`.
- **Fix**: `CREATE INDEX CONCURRENTLY ON users (email)`; verify p50 drops
  back.

## Pass criteria

- Names **missing index** (not N+1 — there is exactly one query; not
  saturation — CPU flat, all-hours slowness).
- Connects the multi-week climb + table growth + no deploy to data-driven
  latency, and cites the `Seq Scan`.
