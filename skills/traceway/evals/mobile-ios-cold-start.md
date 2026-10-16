# Eval: mobile-ios-cold-start

- **ID**: `mobile-ios-cold-start`
- **Area**: mobile
- **Framework**: iOS (native)
- **Checklist item**: Cold start (`performance.md` → Caching and delivery;
  "first-request-after-deploy outliers … latency normalizes after warmup")

## Telemetry snapshot

Mobile `app.start` trace group, last 24h: p50 1.1s, p95 3.8s. The p95 is
entirely first launches after install/update — `firstSeen`-correlated; warm
starts sit at 0.4s.

`traceway traces show <traceId> --recorded-at <t>` — one cold start (3.6s):

```
app.start ................ 3600ms
└── didFinishLaunching ... 2200ms
│   ├── sdk.init .........  900ms  analytics + crash + attribution SDKs, sequential
│   ├── db.migrate .......  700ms  CoreData lightweight migration on main thread
│   └── config.fetch .....  600ms  synchronous remote-config fetch, blocks return
└── first-frame .......... 1400ms
```

No network saturation (config endpoint p95 120ms); the 600ms is one
blocking call plus main-thread contention. Subsequent launches skip the
migration and use cached config.

## Task

> iOS cold start p95 is 3.8s, but only on first launch after install/update.
> Diagnose to a specific checklist cause.

## Expected diagnosis

- **Root cause**: Cold start with synchronous init on the main thread —
  sequential SDK init, main-thread CoreData migration, and a blocking
  remote-config fetch inside `didFinishLaunching`.
- **Key evidence**: first-launch-only outliers (`firstSeen`-correlated);
  warm starts fine; the three init spans serialized on the main thread.
- **Fix**: defer non-critical SDK init off the critical path, move migration
  off main thread, make remote config async with cached fallback.

## Pass criteria

- Names **cold start / synchronous launch init** and ties it to
  first-launch-only data.
- Identifies all three serialized init spans (not just "slow SDK").
