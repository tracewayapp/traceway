<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/slo-configuration.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Service level objectives

An SLO holds an indicator (the share of good events) to a target over a rolling window. It belongs to one project. The server computes it in the background; alert rules of type `slo_burn_rate` and `slo_budget_consumed` read the result. Read the learn guides for error budgets, burn rates and how history is computed.

## Discover, preview, save

Select the project first (`projects use` or global `--project`). List existing SLOs so you do not duplicate one. Take names from telemetry, never from guesses: endpoints from `endpoints list`, metrics from `metrics discover`, monitors from `monitors list`. Preview the definition before saving it.

```bash
traceway-write slos list
traceway-write slos preview --file preview.json
traceway-write slos validate create --file slo.json
traceway-write slos create --file slo.json --yes
traceway-write slos show <id>
```

`slos list`, `show <id>`, `preview --file F` and `series <id> --file F` only read and also exist in the read-only CLI. `create --file F`, `update <id> --file F` and `delete <id>` need the write CLI. Offline `validate <action>` checks the request envelope and rejects `sliConfig` fields the kind does not have (the server would drop them silently and save a wider SLO); the server validates the indicator's values.

MCP equivalents: `list_slos`, `get_slo`, `preview_slo`, `get_slo_series`; `create_slo`, `update_slo`, `delete_slo`. Pass `project_id`, `id` for an existing SLO, and the JSON object as `data`.

Preview takes `{"sliKind", "sliConfig"}` (plus `bucketMinutes` for a metric) and returns `good`, `total` and `sli` (a percentage, `null` when nothing matched) over the last 24 hours. A `total` of 0 means the definition matches no telemetry: fix the names or filters before saving. Use the previewed SLI to judge whether the requested target is realistic and tell the user when it is already missed.

## SLO documents

```json
{
  "name": "Checkout availability",
  "description": "Requests to checkout that do not fail",
  "sliKind": "endpoint",
  "sliConfig": {"endpoint": "POST /api/checkout", "good": "availability"},
  "targetPercent": 99.9,
  "windowDays": 30
}
```

`name`, `sliKind`, `sliConfig`, `targetPercent` and `windowDays` are required. `targetPercent` is above 0 and below 100. `windowDays` is 7, 30 or 90. `bucketMinutes` (1, 5, 15 or 60) is required for `metric` and ignored for the other kinds. `enabled` defaults to true; false pauses the SLO.

| `sliKind` | `sliConfig` fields | Good when |
|---|---|---|
| `endpoint` | `endpoint` (empty string for every endpoint), `good`, `latencyThresholdMs` | `good: "availability"`: status below 500. `good: "latency"`: took at most `latencyThresholdMs` |
| `span` | optional `serviceName`, `name`, `kind` (0-5), `attributes` array of `{key, value}` (at most 10, exact match), then `good`, `latencyThresholdMs` | `availability`: span status is not error. `latency`: took at most the threshold |
| `task` | `taskName` (empty string for every task) | No exception on the run |
| `monitor` | `checkIds`: 1-50 monitor ids of this project | The probe came back up |
| `metric` | `metricName`, `aggregation`, `operator` (`gt`, `gte`, `lt`, `lte`), `threshold`, optional `tags` object (at most 20, exact match), optional `missingIsBad` | The value satisfies `operator threshold`, so write the good condition, not the alert condition |
| `session` | optional `appVersion`, optional `includeMessages` | No exception was recorded in the session |

Metric aggregations are `avg`, `min`, `max`, `sum`, `last` and `distribution`. The first five judge one aggregate per slice of `bucketMinutes`. `distribution` judges every observation of a histogram metric and is required for client distribution metrics such as `browser.web_vital.*`; use `bucketMinutes: 1` with it. `missingIsBad` counts slices without samples as bad (a heartbeat) and is rejected with `distribution`.

Browser and mobile projects use `session` and `metric`; backend projects use the other five. A latency indicator ignores the status code, so pair it with an availability SLO. A session SLO stays at No data unless the SDK reports sessions.

Frontend example, the share of interactions at or below 200 ms:

```json
{
  "name": "INP at most 200 ms",
  "sliKind": "metric",
  "sliConfig": {"metricName": "browser.web_vital.inp", "aggregation": "distribution", "operator": "lte", "threshold": 200, "tags": {"deployment.environment.name": "production"}},
  "bucketMinutes": 1,
  "targetPercent": 90,
  "windowDays": 30
}
```

The dashboard's **One SLO per** option is a shortcut for creating one SLO per tag value. Through the CLI, create one document per value with that value in `tags`.

## Update and delete

`update` replaces the whole definition. Read `slos show <id>`, copy `name`, `description`, `sliKind`, `sliConfig`, `targetPercent`, `windowDays` and `bucketMinutes` from its `slo` object, change what was requested and send all of them. An omitted `enabled` keeps the current state.

Changing `targetPercent`, `windowDays`, `name` or `description` keeps the history. Changing `sliKind`, `sliConfig` or `bucketMinutes` discards the history and error budget and recomputes them from raw telemetry; say so before making that change.

`delete` is permanent. It also deletes every alert rule that references the SLO and resolves the pages those rules opened. `slos show <id>` lists those rules under `rules`; report them before deleting. To stop an SLO temporarily, update it with `"enabled": false`.

## Verify and alert

A new SLO starts as No data. The worker computes it within about a minute and backfills the rest of the window over the next minutes. Read it back with `slos show <id>`: `status` (`ok`, `breached`, `no_data`), `currentSli` (a percentage), `budgetRemaining` (percent of the error budget left, negative once breached) and `burnRate1h`. `slos series <id> --file range.json` with `{"fromDate", "toDate"}` (RFC 3339, at most 91 days, optional `intervalMinutes`) returns `good` and `total` per bucket.

An SLO notifies nobody on its own. Add rules with `alerts create` (see [alert-configuration.md](alert-configuration.md)):

```json
{"name":"Checkout budget burning fast","channelId":12,"ruleType":"slo_burn_rate","config":{"sloId":7,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":14.4}}
```

Common burn-rate pairs are 14.4 over 60 and 5 minutes (page), 6 over 360 and 30 minutes (chat) and 1 over 4320 and 360 minutes (planning). `slo_budget_consumed` takes `{"sloId", "consumedPercent"}`, for example 80. Both rule types send a recovery notice and fire only when the condition starts.
