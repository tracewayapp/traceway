# Eval: mobile-flutter-shader-jank

- **ID**: `mobile-flutter-shader-jank`
- **Area**: mobile
- **Framework**: Flutter
- **Checklist item**: Cold start / JIT / connection warmup (`performance.md`
  → Caching and delivery; "latency normalizes after warmup")

## Telemetry snapshot

`route.transition` trace group (product detail page): p50 320ms, p99 2.1s.
The p99s are *first visits per install* to any page with the animated hero
carousel — `firstSeen`-correlated per route; second visits render at ~300ms.

`traceway traces show <traceId> --recorded-at <t>` — one first visit (2.0s):

```
route.transition ......... 2000ms
├── build/layout .........  180ms
├── <gap: raster thread> . 1450ms  ← shader compilation (SkSL warmup)
└── first-frame ..........  370ms
```

`system.cpu.utilization` on device spikes during the gap (raster thread
compiling shaders); no network spans in the window — the page data was
already cached. After the first visit, subsequent transitions are smooth.

## Task

> Flutter product-page transitions hit 2.1s p99, but only the first visit
> per install; data is cached and the network is idle. Diagnose to a
> specific checklist cause.

## Expected diagnosis

- **Root cause**: Shader/JIT warmup jank — first-frame SkSL compilation on
  the raster thread; classic warmup pattern (bad once, then fine).
- **Key evidence**: first-visit-only outliers; raster-thread gap with no
  spans and no network; CPU spike on device during the gap.
- **Fix**: ship SkSL warmup (`flutter build --bundle-sksl-path`), or
  pre-warm the carousel offscreen.

## Pass criteria

- Names **shader/JIT warmup** (not "slow build" — build/layout is 180ms;
  not network — idle).
- Recognizes the first-visit-only + normalizes-after-warmup signature.
