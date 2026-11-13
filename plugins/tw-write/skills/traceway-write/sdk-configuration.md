<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/sdk-configuration.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Browser SDK collection over OpAMP


Use this for an **existing browser frontend project** (React, Svelte, Vue or jQuery) running the JavaScript SDK v2 with remote config enabled. SDK v2 is in beta (npm `beta` tag; `@2` matches nothing until 2.0 is stable), and 1.x clients ignore these rules. It changes the backend's stored collection rules; it does not require a setup plan, a new project, a new ingest token, or edits to the app's connection string. Backend and mobile projects reject remote config. Servers released before these agent routes reject them; upgrade the server instead of bypassing a 403. Targeted captures remain dashboard-only.

1. Select the project, then `traceway-write opamp rules list --output json` (`list_sdk_config_rules`). Keep the existing rule documents, ids and `updatedAt`. Inspect the order, conditions, triggers and kill switches before writing. Later matching rules override earlier ones; a newly created rule is appended.
2. State the requested scope: which clients, signals, sampling rates, replay and input masking. Empty `conditions: {}` matches all browsers. Target releases or cohorts with `{"all":[{"key":"service.version","op":"eq","value":"2.0.0"}],"rollout":{"percent":25,"by":"instance"}}`. Rollout is sticky per browser instance; no rollout means 100%. Do not broaden a targeted request to every client or remove masking unless explicitly requested.
3. Write a rule JSON file containing `name`, `enabled`, `conditions`, and `settings`. Create with `traceway-write opamp rules create --file rule.json --yes`. To edit, send the complete revised name, conditions and settings with `opamp rules update <id> --file rule.json --expected-updated-at <updatedAt> --yes`. Preserve unrelated settings and triggers. `enabled: false` disables a rule; omitting enabled on update preserves it. Missing fields inside settings revert to inheritance, not their previous values. A 409 means someone changed the rule: list again and reapply the intended edit.
4. Verify with `opamp rules list`, then `opamp preview --file client.json` (`preview_sdk_config`). A preview file is `{"attributes":{"service.version":"2.0.0"},"instanceUid":"<browser-instance-UUID>"}`; omit instanceUid when rollout identity is irrelevant. Preview evaluates **saved** rules and applies plan limits. It returns settings, matchedRules, matchedCaptures, configHash and heartbeatIntervalSeconds. It is not a dry-run of an unsaved rule and does not prove any browser has applied it or uploaded data.
5. Report the saved rule id, effective settings and remaining verification. Browsers pick up changes at their next poll (normally 5 minutes) or SDK refresh. Developer ceilings and plan limits still apply. Do not promise every byte or session arrived merely because the rule saved.

For a request to stream every session, breadcrumb and replay, retain masking and use:

```json
{
  "name": "Full frontend sessions",
  "enabled": true,
  "conditions": {},
  "settings": {
    "schemaVersion": 1,
    "enabled": true,
    "signals": {
      "errors": { "mode": "stream", "sampleRate": 1 },
      "messages": { "mode": "stream", "sampleRate": 1 },
      "network": { "mode": "stream", "sampleRate": 1 },
      "console": { "mode": "stream", "sampleRate": 1, "minSeverity": "trace" },
      "navigation": { "mode": "stream", "sampleRate": 1 },
      "userActions": { "mode": "stream", "sampleRate": 1 },
      "webVitals": { "mode": "stream", "sampleRate": 1 },
      "resourceTiming": { "mode": "stream", "sampleRate": 1 },
      "custom": { "mode": "stream", "sampleRate": 1 }
    },
    "sessions": { "enabled": true, "baselineSampleRate": 1 },
    "replay": { "mode": "stream", "sessionSampleRate": 1, "maskAllInputs": true }
  }
}
```

Signals support `off`, `buffer` and `stream`; sample rates are 0..1. `sessions.baselineSampleRate` samples healthy breadcrumbs/network, **not replay**. Replay needs its own mode and sessionSampleRate. `settings.enabled: false` is a collection kill switch; `enabled: false` on the rule only skips that rule. Never send server-owned `limits` or `captures` in settings. Settings can also carry `buffer` and `triggers`; preserve them when changing an existing rule.

Undo an update by restoring the saved document with the latest `updatedAt`. For a temporary change prefer disabling the added rule. `opamp rules delete <id> --yes` permanently removes it and exposes earlier rules/SDK defaults; those defaults can collect more or less. `opamp rules reorder <id>... --yes` requires every project rule exactly once and changes precedence. Re-list before reordering and preview representative clients afterward.
