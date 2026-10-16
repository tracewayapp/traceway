# Eval: backend-node-sequential-calls

- **ID**: `backend-node-sequential-calls`
- **Area**: backend
- **Framework**: Node.js (NestJS, microservice fan-out)
- **Checklist item**: Sequential calls that could run in parallel
  (`performance.md` → Network, IO, and external dependencies)

## Telemetry snapshot

`traceway endpoints list --search "GET /api/dashboard" --since 24h --output json`:

```json
{ "endpoint": "GET /api/dashboard", "count": 31200, "p50": 3890, "p95": 4520, "p99": 5100, "errorCount": 0 }
```

`traceway traces show <traceId> --recorded-at <t>` — cross-service timeline
(total 4.41s):

```
GET /api/dashboard (edge) .............. 4410ms
└── svc-users GET /profile .............  890ms  ─┐
└── svc-billing GET /balance ...........  920ms   ├ staircase: each starts
└── svc-notify GET /unread .............  870ms   │ only when the previous
└── svc-reco GET /suggestions ..........  940ms  ─┘ ends
└── json.encode ........................  120ms
```

Each downstream service is healthy on its own (their p95s are 120–200ms in
their own dashboards); the 900ms here is mostly network round-trip per hop.
`system.cpu.utilization` flat at ~30%.

## Task

> `GET /api/dashboard` p50 is 3.9s. Every downstream service looks healthy
> on its own. Diagnose the bottleneck to a specific checklist cause.

## Expected diagnosis

- **Root cause**: Sequential downstream calls that could run in parallel —
  the staircase waterfall shows four independent service hops serialized;
  each hop's own latency is fine, the sum is the problem.
- **Key evidence**: staircase shape in `traces show`; downstream services
  healthy in isolation; flat CPU rules out saturation.
- **Fix**: `Promise.all` the four independent calls (they share no data
  dependencies); expected p50 ≈ 1.1s.

## Pass criteria

- Names **sequential calls that could run in parallel** (not "slow
  upstream" — upstreams are healthy; not "chatty fan-out" — four calls is
  not chatty, the serialization is the issue).
- Reads the staircase waterfall correctly and notes the calls are
  independent.
