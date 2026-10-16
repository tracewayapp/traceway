# Eval: infra-cpu-saturation

- **ID**: `infra-cpu-saturation`
- **Area**: infrastructure
- **Framework**: n/a (fleet-wide)
- **Checklist item**: CPU saturation (`performance.md` → Infrastructure and saturation)

## Telemetry snapshot

`traceway endpoints list --since 24h --order-by p95 --page-size 10`: the top
8 endpoints *all* show p95 2–4× their 7-day baseline, starting at 15:40
yesterday. No single span dominates — waterfalls show every span type
(DB, serialize, upstream) uniformly slower, with no long pole.

`traceway metrics query --name system.cpu.utilization --aggregation max --since 24h --interval-minutes 60`:

```
14:00  38%   15:00  44%   16:00  97%   17:00  98%   18:00  96% ...
```

The CPU step at ~15:40 lines up exactly with the latency rise across all
endpoints. `git log` shows the only deploy in the window was a config-only
change at 15:35 that doubled the log verbosity (see also
`backend-excessive-logging`, not this eval). `mem.used` flat.

## Task

> Eight unrelated endpoints all went 2–4× slower at 15:40 yesterday, with no
> single slow span. Diagnose to a specific checklist cause — and say what it
> is *not*.

## Expected diagnosis

- **Root cause**: CPU saturation — fleet-wide latency rise with no long
  pole, aligned exactly with the `system.cpu.utilization` step to 97%+.
- **Key evidence**: broad rise across endpoints (rules out a code bug in one
  path); no dominating span (rules out a slow query/upstream); metric
  inflection at the same timestamp.
- **What it is not**: not a code regression (config-only deploy, no single
  culprit span); the doubled log verbosity is a *contributor* to CPU, worth
  fixing, but the finding is saturation.
- **Fix**: scale out / move the verbose logging off the hot path, then
  re-check headroom.

## Pass criteria

- Names **CPU saturation** and explicitly rules out a single code/query
  cause using the breadth of the rise and the metric alignment.
- Does not blame the config deploy as a code regression without evidence.
