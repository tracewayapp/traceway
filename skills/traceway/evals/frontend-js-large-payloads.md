# Eval: frontend-js-large-payloads

- **ID**: `frontend-js-large-payloads`
- **Area**: frontend
- **Framework**: browser JS (React SPA)
- **Checklist item**: Large request/response payloads (`performance.md` →
  Network, IO, and external dependencies)

## Telemetry snapshot

`traceway endpoints list --search "GET /api/feed" --since 24h --output json`:

```json
{ "endpoint": "GET /api/feed", "count": 120400, "p50": 1810, "p95": 3400, "p99": 5200, "errorCount": 0 }
```

`traceway endpoints show <endpointId> --recorded-at <t>` — one request
(total 3.28s):

```
GET /api/feed ............ 3280ms
└── db.query .............  210ms  SELECT ... LIMIT 50
└── json.encode ..........  180ms
└── transfer ............. 2840ms  response 2.3 MB
```

The response carries 50 feed cards with full-resolution image URLs inlined
as base64 (`data:` URIs, ~45KB each). p50/p95 correlate with the `?page`
depth, not with server load; `system.cpu.utilization` flat at ~25%. Mobile
web p95 is 2× desktop — transfer-dominated.

## Task

> `GET /api/feed` p50 is 1.8s, worse on mobile web. Server-side spans are
> fast. Diagnose the bottleneck to a specific checklist cause.

## Expected diagnosis

- **Root cause**: Large response payloads — 2.3MB per response, dominated by
  inlined base64 images; transfer time is the long pole, not server work.
- **Key evidence**: `transfer` span 2.84s vs 210ms DB; latency tracks page
  depth and is worse on mobile (bandwidth-bound signature); flat CPU.
- **Fix**: thumbnail URLs + lazy-load full images, paginate or compress,
  strip base64 inlining.

## Pass criteria

- Names **large payloads** (not "slow DB" — DB is 210ms; not "no cache" —
  there is no evidence about caching here).
- Identifies transfer as the long pole and ties the mobile-web skew to
  bandwidth.
