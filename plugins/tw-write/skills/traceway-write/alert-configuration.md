<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/alert-configuration.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Alert channels and project rules

A project **rule** watches a condition and routes to exactly one **channel**. Ordinary channels send notifications; an `escalation` channel opens an on-call page. Read the learn alert guide for evaluation timing, cooldowns, severities and recovery behavior. Personal notification chains are a different concept: they decide how to reach a responder after a page targets that person.

## Discover, validate, save

Select the project first (`projects use` or global `--project`). List channels and rules before changing them. Create the destination before a rule that references its returned integer ID. Updates send the complete configuration, not a patch: preserve unrelated fields. Inspect current rule/channel relationships before deleting a channel; deleting it also deletes its attached rules.

```bash
traceway-write channels list
traceway-write alerts list
traceway-write channels validate create --file channel.json
traceway-write channels create --file channel.json --yes
traceway-write alerts validate create --file rule.json
traceway-write alerts create --file rule.json --yes
traceway-write alerts list
```

Each resource supports `list`, `create --file F`, `update <id> --file F`, `delete <id>`. Use `channels test <id>` only when asked to send a test; escalation tests open a real page. Existing `alerts enable`, `disable` and `snooze --minutes N` change state separately. Offline `validate <action>` checks the request envelope; the server validates adapter configuration, references, project type and permissions. It is not a delivery test.

MCP equivalents: `list_notification_channels`, `create_notification_channel`, `update_notification_channel`, `delete_notification_channel`, `test_notification_channel`; `list_notification_rules`, `create_notification_rule`, `update_notification_rule`, `delete_notification_rule`. Pass `project_id`, `id` for an existing record, and the JSON object as `data` for creates/updates.

## Channel documents

```json
{"name":"Operations email","channelType":"email","config":{"recipients":["ops@example.com"]}}
```

| `channelType` | `config` fields |
|---|---|
| `email` | `recipients`: 1–10 email addresses |
| `slack` | `webhookUrl`; optional `channel`, `username` |
| `webhook` | `url`; optional `method` (`POST`/`PUT`), `headers` object, HMAC `secret` |
| `github` | `token`, `owner`, `repo`; optional `labels` array |
| `pushover` | `userKey`, `appToken`; optional `device`, `priority`, `retry`, `expire`, `sound`, `ttl`, `html`, `callback` |
| `telegram` | `botToken`, `chatId` |
| `escalation` | `policyId`: existing escalation policy in the same organization |

Configuration includes credentials. Obtain real values through the environment's secret mechanism; avoid printing them or committing JSON with secrets. A saved channel is not evidence delivery succeeded. Channel documents also accept optional `enabled`: new channels default to true; omitting it on update preserves the current state.

## Rule documents

```json
{
  "name":"API latency above 1 second",
  "channelId":12,
  "ruleType":"endpoint_p95_threshold",
  "config":{"endpoint":"GET /api/orders","thresholdMs":1000,"lookbackMinutes":10},
  "cooldownMinutes":15,
  "severity":"warning"
}
```

Use discovered endpoint/task/metric names; choose thresholds from observed traffic and the user's service objectives. `severity` is an empty string for automatic severity, or `critical`, `warning`, `info`. New rules start enabled. A zero/omitted cooldown defaults to 15 minutes. `enabled` and snooze are separate operations, not fields in this document. Changing type, channel or condition resets cooldown state; cosmetic edits do not.

| `ruleType` | `config` fields |
|---|---|
| `new_error` | `ignorePatterns` array of case-insensitive substrings (`*` is stripped) |
| `error_regression`, `check_down` | `{}` |
| `error_rate_threshold` | `thresholdPercent`, `lookbackMinutes`, `minRequests` |
| `error_count_threshold` | `thresholdCount`, `lookbackMinutes` |
| `endpoint_error_rate` | `endpoint` (specific, not `*`), `thresholdPercent`, `lookbackMinutes`, `minRequests` |
| `endpoint_p95_threshold`, `endpoint_p99_threshold` | `endpoint` (or `*`), `thresholdMs`, `lookbackMinutes` |
| `apdex_drop` | `thresholdApdex`, `lookbackMinutes`, `minRequests` |
| `throughput_drop` | `dropPercent`, `lookbackMinutes`, `baselineWindowMinutes` |
| `impact_score_critical`, `impact_score_high`, `impact_score_medium` | `minRequests` |
| `task_duration_threshold` | `taskName` (or `*`), `thresholdMs`, `lookbackMinutes` |
| `task_failure_rate` | `taskName` (or `*`), `thresholdPercent`, `lookbackMinutes`, `minExecutions` |
| `metric_threshold` | `metricName`, `aggregation`, `operator`, `thresholdValue`, `lookbackMinutes`; optional `tags` object and `groupBy` array |
| `no_data` | `dataType` (`any`, `endpoints`, `exceptions`, `metrics`, `tasks`), `silenceMinutes` |
| `ai_trace_cost` | `traceName` (or `*`), `thresholdCost` in dollars |
| `ai_conversation_cost` | `thresholdCost` in dollars |
| `ai_flagged_content` | `terms` array (empty means any configured flagged term) |
| `slo_burn_rate` | `sloId`, `longWindowMinutes`, `shortWindowMinutes`, `burnRateThreshold` |
| `slo_budget_consumed` | `sloId`, `consumedPercent` |

Metric aggregations are `avg`, `max`, `min`, `sum`, `p95`, `p99`, `last`; operators are `gt`, `gte`, `lt`, `lte`, `eq`. Tags filter observed series and groupBy evaluates groups separately. Without grouping, an average can hide one unhealthy host: use `max`, `min` or grouping as appropriate. No data does not trigger a threshold rule; configure a No Data rule for missing telemetry. Browser/mobile projects accept only `new_error` and the SLO rule types.

Read delivery history with `notification-history list --file query.json`, where the file is `{"pagination":{"page":1,"pageSize":20}}` with optional `search`, `fromDate`, `toDate`. A queued send retains the channel settings at enqueue time; edits do not rewrite pending notifications.
