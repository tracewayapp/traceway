<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/learn/data-collection.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Remote SDK config (Data Collection)

Remote SDK config decides, per group of browsers, what the browser SDK sends right away, what it keeps on the device in case it is needed, and what it drops, without shipping a new build. It is in beta.

- **Who has it**: browser projects (React, Vue.js, Svelte, jQuery) whose app runs the 2.x browser SDK (`@tracewayapp/frontend` 2.x or a wrapper of the same version, published under the npm `beta` tag). The 1.x SDK and the mobile SDKs never ask for settings. Backend and mobile projects have no Data Collection tab, and the API refuses them with 403.
- **Where**: project switcher, **Edit Project**, **Data Collection** tab. It lists the **Rules** in order (enable switch, move up and down, edit, delete) with **Preview** and **New Rule**, and below them the **Captures** with their status, refreshed every 15 seconds, and **New Capture**.
- **How it travels**: the SDK polls Traceway over OpAMP at start, every 5 minutes by default (`OPAMP_POLL_INTERVAL_SECONDS` on self-hosted servers), when a tab becomes visible after that interval, and when the app calls `refreshConfig()`. There is no push, so a saved change reaches a browser at its next poll.

## Modes and defaults

Every signal is in one mode: **stream** (sent within about a second), **buffer** (kept on the device, sent only when a trigger flushes it or a capture starts) or **off** (dropped, though triggers still see it).

| Signal | Covers | Default |
|---|---|---|
| `errors`, `messages` | Exceptions, `captureMessage` | stream |
| `network` | `fetch` and XHR, as CLIENT spans | buffer |
| `console`, `navigation`, `userActions`, `custom` | Console lines (info and above), route changes, clicks, `track()` and `recordAction()` | buffer |
| `webVitals` | LCP, INP, CLS and the rest | stream |
| `resourceTiming` | Page and asset load timings | off |
| replay | rrweb session replay | buffer |

Session start and end events are sent unless `sessions.enabled` is false. The built-in `default-exception` trigger uploads up to 10 minutes of buffered breadcrumbs and a 30-second replay clip on each exception (5-second cooldown, at most 20 per session). `recordAllSessions: true` in `init()` streams breadcrumbs and replay instead of buffering them.

## How a client's settings are decided

1. **SDK defaults**, as above.
2. **Rules**: every enabled rule whose conditions match the client adds the fields it sets, in list order, and a later rule wins. A rule holds only what it changes. Conditions test what the browser reports (`service.version`, `deployment.environment.name`, `browser.name`, `browser.version`, `os.name`, `user.id`, `session.id`, `service.instance.id`, anything the app sets with `setAttribute`), all must hold, and an optional rollout keeps the rule to a sticky percentage of clients. A rule's settings can also carry triggers and the kill switch (`enabled: false`, which turns everything off for the matching clients).
3. **Ceilings**: the `capture*`, `sessionRecording` and `sessions` options of `init()`, and what the organization's plan does not include, can never be turned back on remotely.
4. **Timed overrides**: a trigger's `stream` action, and captures, which also bring back signals a rule switched off.

**Triggers** run on the device: one to five steps matched in order (event name, signal, severity, log body with `contains`, `icontains`, `starts_with` or `glob`, span status and duration, any attribute), optionally a count within a sliding window, then actions: `flush` (upload the last N seconds, optionally with a replay clip), `stream`, `startReplay`, `emit` (a marker event) or `capture`. There is no regular expression operator on purpose.

**Captures** make one target upload what it was holding (the lookback, 10 minutes by default) and stream everything for 1 minute to 24 hours. A target is a device (`service.instance.id`), a user (`user.id`), a session or any attribute and value. At most 20 are active per project, and a client under a capture polls every 60 seconds. Start one from **New Capture**, from **Capture this device** on a session page, or from **Capture device** (or user, or session) on an issue's event card. A trigger's `capture` action starts one on the device itself. Capture status reads **Waiting for client**, **Delivered · awaiting acknowledgement**, **Applied · awaiting data**, **Collecting**, **Expired · no data received**, **Completed** or **Cancelled**, and reflects at least one matching browser, not every one.

**Preview** shows the document a client with given attributes (and optionally an instance id, which decides rollout buckets) receives from the saved rules and active captures, with plan limits applied. It does not know the app's `init()` options or whether a browser has polled, so it is not proof of what a live browser runs.

## Seeing it work

The SDK reports what it did as log records of the browser project that always stream unless the kill switch is on: `traceway.config.applied` (with the config hash), `traceway.trigger.fired` and `traceway.capture.started`. Breadcrumbs that were sent are stored once, as log records and spans tagged with `session.id`. The session page reads them for the whole session, and an issue's event card reads the 10 minutes before the exception. A session's overview shows the **JavaScript SDK** version that recorded it, which tells a 2.x client from a 1.x one.

## Reading and changing it from an agent

| Task | CLI | MCP tool |
|---|---|---|
| List the rules in order, with `updatedAt` | `traceway opamp rules list` | `list_sdk_config_rules` |
| Preview what a client gets | `traceway opamp preview --file client.json` with `{"attributes":{...},"instanceUid":"..."}` | `preview_sdk_config` |
| Create a rule (write) | `traceway-write opamp rules create --file rule.json` | `create_sdk_config_rule` |
| Replace a rule (write) | `traceway-write opamp rules update <id> --file rule.json --expected-updated-at <updatedAt>` | `update_sdk_config_rule` |
| Reorder or delete rules (write) | `traceway-write opamp rules reorder <id>...` / `delete <id>` | `reorder_sdk_config_rules` / `delete_sdk_config_rule` |

Rule writes follow the three opt-in levels in `organization.md`, and the `traceway-write` skill has the playbook. A rule can stream breadcrumbs for every visitor, turn on full-session replay or set `replay.maskAllInputs: false`, so say what a change collects before saving it. Captures are dashboard only, for reading and starting alike.
