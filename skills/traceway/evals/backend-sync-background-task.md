# Eval: backend-sync-background-task

- **ID**: `backend-sync-background-task`
- **Area**: backend
- **Framework**: Go (Gin)
- **Checklist item**: Synchronous work that belongs in a background task
  (`performance.md` → Application and compute)

## Telemetry snapshot

`traceway endpoints list --search "POST /api/reports" --since 24h --output json`:

```json
{ "endpoint": "POST /api/reports", "count": 1400, "p50": 9400, "p95": 12100, "p99": 14800, "errorCount": 0 }
```

`traceway endpoints show <endpointId> --recorded-at <t>` — one request
(total 11.9s):

```
POST /api/reports ........ 11900ms
└── db.query .............   320ms  INSERT INTO reports ...
└── pdf.render ............ 9100ms  (in-process)
└── s3.put ................  410ms  PUT reports/2026-10-16/...
└── json.encode ...........   18ms  {"report_id": "...", "status": "ready"}
```

The response returns only after the PDF is fully rendered and uploaded; the
user does not need the bytes synchronously (the UI polls `GET
/api/reports/:id`). `system.cpu.utilization` spikes to 91% on the single
instance during each render. No queue depth anywhere — there is no task
system on this path.

## Task

> `POST /api/reports` p50 is 9.4s. Diagnose the bottleneck to a specific
> checklist cause and propose the fix.

## Expected diagnosis

- **Root cause**: Synchronous work that belongs in a background task — the
  9.1s `pdf.render` span runs before the response returns, for work the user
  does not wait on.
- **Key evidence**: the in-process render span dominating the waterfall;
  response only returned after upload; UI already polls for completion, so
  async is safe.
- **Fix**: return `202 {"status": "processing"}` immediately and move render
  + upload into a `CaptureTask` background task.

## Pass criteria

- Names **synchronous work that belongs in a background task** (not "CPU
  saturation" — CPU spikes because of this render, it is the effect, not the
  cause; not "slow upstream" — S3 put is 410ms).
- Notes the user doesn't need the result synchronously (polling UI) and
  proposes the background-task fix.
