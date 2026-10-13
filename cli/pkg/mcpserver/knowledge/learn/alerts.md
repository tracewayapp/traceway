# Alerts

Alerts are how Traceway tells people that something happened. They live under **Alerts** in the project sidebar (`/notifications`), in three tabs kept in `?tab=`:

| Tab | What it holds |
|---|---|
| **Channels** (default) | Where notifications go. Create a channel first, then attach rules to it. |
| **Rules** | When to notify. Each rule watches one condition and sends to exactly one channel. A channel can serve many rules. |
| **History** | Every notification that was sent, with its status |

Rules and channels belong to a project and only see that project's data. Channels have their own On/Off switch, and a rule only runs while its channel is on. Changing them needs write access on the project. Browser and mobile projects only offer the New Issue rule type.

## Channels

| Type | Configuration | What it sends |
|---|---|---|
| **Email** | 1 to 10 recipient addresses | An HTML email per alert |
| **Slack** | Incoming webhook URL, optional channel override and username | An attachment coloured by severity: blue info, orange warning, red critical, green recovery |
| **Webhook** | URL, method (POST or PUT), optional headers, optional HMAC secret | JSON with `subject`, `body`, `severity`, `ruleType`, `ruleName`, `url`, `timestamp`. With a secret, `X-Traceway-Signature: sha256=<hex>` |
| **GitHub** | Fine-grained token with Issues read and write, owner, repo, optional labels | A GitHub issue. These issues can start the Auto-Fix agent in the repository. |
| **Pushover** | User key and app token, optional device, priority (normal, high, or emergency with a retry of 30 s or more, an expiry up to 10800 s and an optional callback URL), sound, HTML formatting, TTL | A push notification with the dashboard link |
| **Telegram** | Bot token and chat id | A message, cut at 4096 characters with the link kept |
| **Escalation policy** | An escalation policy of the same organization | Nothing by itself: it opens an on-call **page** that escalates until someone acknowledges (`on-call.md`) |

SMS is not a channel type. It is a personal on-call contact method.

Every channel has a **Test** button that sends "Traceway Test Notification". On an escalation channel, Test opens a **real page** and notifies whoever is on call. Resolve that page when done. Testing again while it is still open is refused.

Alert subjects look like `[<Project>] New error: TypeError` or `[<Project>] Check "api" is down`. Exception alerts carry the exception id, hash, occurred-at time, app version, server, the endpoint or task, the stack trace, the attributes and a link to `/issues/<hash>`. Those three fields (hash, exception id, occurred at) are what an agent needs to open the occurrence (see the `notifications` knowledge in the read skill).

## Rule settings shared by every type

| Setting | Meaning |
|---|---|
| Name | Up to 200 characters, shown in every notification |
| Channel | Exactly one |
| Cooldown | Minimum time between notifications for the same condition. Default 15 minutes. |
| Severity | **Auto** (derived from how bad the condition is) or a fixed Critical, Warning or Info. Recovery notices are always Info. |
| Enabled | New rules start enabled. A rule runs only while both it and its channel are enabled. |

## Rule types

The numbers in brackets are what the evaluator uses when a setting is missing. The **New Rule** form starts every type at the same values (5%, 5 minutes, 10 requests, 500 ms, `*`), so set each field on purpose.

| Rule | Fires when | Settings | Auto severity |
|---|---|---|---|
| **New Issue** | An exception hash is seen for the first time, or again for the first time after it was archived | Ignore patterns: comma-separated, case-insensitive substrings matched against the error type (the first stack line up to its first `:`). `*` is stripped, not a wildcard. | Critical |
| **Error Regression** | An archived (resolved) issue occurs again | None | Critical |
| **Error Rate** | The 5xx share across all endpoints exceeds a percentage | Threshold 5%, lookback 5 min, min requests 10 | Warning, Critical at 2x |
| **Error Count** | Errors in the window reach a count (captured messages are not counted) | Threshold 100, lookback 60 min | Warning, Critical at 5x |
| **Endpoint Error Rate** | One endpoint's 5xx share exceeds a percentage | A specific endpoint (`*` matches nothing here), threshold 2%, lookback 10 min, min requests 20 | Warning, Critical at 2x |
| **Endpoint P95** / **Endpoint P99** | The percentile exceeds a threshold | Endpoint or `*` for all, threshold 500 ms, lookback 5 min | Warning |
| **Apdex Drop** | Apdex falls below a threshold | Threshold 0.85, lookback 15 min, min requests | Warning, Critical below 0.5 |
| **Throughput Drop** | Requests drop versus the baseline window just before, scaled to the same length. No baseline traffic, no alert. | Drop 50%, lookback 15 min, baseline 60 min | Warning, Critical at 80% or more |
| **Impact Score Critical / High / Medium** | An endpoint newly enters the level (0.75, 0.50, 0.25) over the trailing 24 hours. Fires once on entry, again only after it leaves and comes back. The first run after a restart or edit only records the baseline. | Min requests 50 | Critical / Warning / Info |
| **Task Duration** | A task's P95 duration exceeds a threshold | Task or `*`, threshold 30000 ms, lookback 30 min | Warning |
| **Task Failure Rate** | The share of task runs with an exception on their own span exceeds a percentage | Task or `*`, threshold 10%, lookback 60 min, min executions 5 | Warning, Critical at 2x |
| **Metric Threshold** | An aggregated metric violates a comparison. No data in the window, no alert. | Metric, aggregation (avg, max, min, sum, p95, p99, last), operator (gt, gte, lt, lte, eq), value 90, lookback 5 min | Warning, Critical when 20% past the value |
| **No Data** | No telemetry of a kind for N minutes. `any` fires only when all four kinds are silent. | Data type (any, endpoints, exceptions, metrics, tasks), 10 min | Critical |
| **AI Trace Cost** | One AI call costs at least the threshold | Trace name or `*`, $0.50 | Warning, Critical at 3x |
| **AI Conversation Cost** | One conversation's cost over 24 hours reaches the threshold | $5 | Warning, Critical at 3x |
| **AI Flagged Content** | An AI call matches a flagged term. The terms come from the project's AI settings. | Optional term list (empty means any flagged match) | Warning |
| **Monitor Down** | Any monitor in the project goes down, plus a recovery notice when it comes back | None | Critical, recovery Info |

The aggregation in Metric Threshold spans every series of the metric with no tag filter, so prefer `max` (or `min` for lower-is-bad metrics) where one bad host could hide in an average.

Apdex uses fixed bands: satisfied up to 750 ms, tolerating up to 1500 ms, anything slower or a 5xx is frustrated. Apdex = (satisfied + tolerating / 2) / total.

## When rules run

- **On ingest**: New Issue, Error Regression, AI Trace Cost, AI Conversation Cost, AI Flagged Content. These fire within seconds.
- **On state change**: Monitor Down, the moment a probe flips the monitor.
- **Polled**: everything else, every 60 seconds by default (self-hosted: `NOTIFICATION_POLL_SECONDS`).

Cooldowns differ by kind:

- **Polled rules** have one cooldown per rule. After it fires, it is not evaluated again until the cooldown ends. This cooldown survives restarts.
- **Ingest and monitor rules** cool down per item: per exception hash, trace name, conversation or monitor. Two different new errors a minute apart both alert. The same error does not alert again within its cooldown. This memory is in process only, so a restart can let the same item alert once more.
- The cooldown cannot be 0. Empty or 0 becomes 15 minutes.
- **Editing** a rule resets its cooldown and its per-item memory, so the new settings apply at once. Toggling it does not.

Delivery goes through a durable queue. Failed sends are retried after 1, 5, 15 and 60 minutes, five attempts in all. A queued message keeps the channel settings it was sent with.

## Disable versus snooze

- **Disable** turns a rule off until someone turns it back on. The dashboard asks for confirmation.
- **Snooze** mutes a rule for a set time and it comes back on its own. The dashboard offers 30 minutes and 1, 4, 8 or 24 hours. Clearing the snooze turns it back on early.
- A snoozed or disabled rule is **not evaluated at all**, so nothing is queued while it is muted and it does not catch up afterwards.

During a known incident or a deploy, snooze the noisy rule rather than disabling it: a snooze cannot be forgotten in the off position.

The Rules table shows each rule's status as **On**, **Off** or **Snoozed**.

## History

The History tab lists every notification with severity, rule, subject (linking to the page it is about), channel, status and time. It has search and a time range. Statuses:

| Status | Meaning |
|---|---|
| Sent | Delivered. On an escalation channel, a page was opened. |
| Failed | All retries failed. The error is kept. |
| Deduped | An escalation alert folded into a page that was already open |

History opens on the last 7 days.

## Reading and changing it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| List rules with enabled and snoozed state | `traceway alerts list` | `list_notification_rules` |
| Turn a rule on or off (write) | `traceway-write alerts enable <id>` / `disable <id>` | `set_notification_rule_enabled` |
| Snooze a rule (write) | `traceway-write alerts snooze <id> --minutes N` (0 clears) | `snooze_notification_rule` |

Creating and editing rules and channels, the History tab and the Test buttons stay in the dashboard. Channels hold credentials and webhook URLs.

To resolve an alert someone pasted, the read skill's `notifications` section explains how to go from an exception or performance alert to the data behind it.
