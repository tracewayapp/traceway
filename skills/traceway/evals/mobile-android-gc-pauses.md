# Eval: mobile-android-gc-pauses

- **ID**: `mobile-android-gc-pauses`
- **Area**: mobile
- **Framework**: Android (native, ART)
- **Checklist item**: Memory pressure / GC pauses (`performance.md` →
  Infrastructure and saturation)

## Telemetry snapshot

`scroll.jank` trace group (feed scroll): p50 frame time 22ms, p99 190ms.
The p99 frames cluster in sessions with >3 minutes of continuous scrolling.

`traceway traces show <traceId> --recorded-at <t>` — one janky scroll
window:

```
scroll.window ............ 2400ms
├── frame ................   24ms
├── frame ................   28ms
├── <gap: no spans> ......  165ms   ← ART GC pause (concurrent → blocking)
├── frame ................  188ms
└── image.decode .........   96ms   (full-res bitmap, 4096×4096)
```

`mem.used` climbs monotonically across the session; the image cache has no
size cap (in-memory LRU unbounded). GC pause frequency tracks scroll depth,
not network or server state. Server spans for the same feed are fast
(p95 300ms).

## Task

> Android feed scroll p99 frame time is 190ms, worsening the longer the
> session runs. Server side is fine. Diagnose to a specific checklist cause.

## Expected diagnosis

- **Root cause**: Memory pressure / GC pauses — unbounded in-memory image
  cache grows with scroll depth; ART GC pauses surface as gaps in the frame
  timeline.
- **Key evidence**: the span-less gap (GC, not work); `mem.used` climbing
  with session length; full-res decodes feeding an uncapped cache; server
  side clean.
- **Fix**: cap the image cache (e.g. 128MB LRU), decode at display size,
  recycle bitmaps.

## Pass criteria

- Names **memory pressure / GC pauses** and reads the gap as GC rather than
  missing work.
- Connects the uncapped cache + `mem.used` climb to the pauses (not "slow
  images" alone — decode is 96ms, the 165ms gap is the killer).
