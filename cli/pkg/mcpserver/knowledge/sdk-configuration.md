# Browser and Flutter SDK collection over OpAMP


Use this for an **existing browser frontend project** (React, Svelte, Vue or jQuery) running the JavaScript SDK v2 with remote config enabled (the default), or a **Flutter project** running the Flutter SDK 2.x prerelease with a connection string. SDK v2 is the stable browser release (npm `latest`, `@2`); 1.x clients never poll, so they ignore these rules, and so does a standalone Flutter app. It changes the backend's stored collection rules; it does not require a setup plan, a new project, a new ingest token, or edits to the app's connection string. Backend projects and React Native, Android and iOS projects reject remote config. Servers released before these agent routes reject them; upgrade the server instead of bypassing a 403. Targeted captures remain dashboard-only.

1. Select the project, then `traceway-write opamp rules list --output json` (`list_sdk_config_rules`). Keep the existing rule documents, ids and `updatedAt`. Inspect the order, conditions, triggers and kill switches before writing. Later matching rules override earlier ones; a newly created rule is appended.
2. State the requested scope: which clients, signals, sampling rates, replay and input masking. Empty `conditions: {}` matches all browsers. Target releases or cohorts with `{"all":[{"key":"service.version","op":"eq","value":"2.0.0"}],"rollout":{"percent":25,"by":"instance"}}`. Rollout is sticky per browser instance; no rollout means 100%. Do not broaden a targeted request to every client or remove masking unless explicitly requested.
3. Write a rule JSON file containing `name`, `enabled`, `conditions`, and `settings`. Create with `traceway-write opamp rules create --file rule.json --yes`. To edit, send the complete revised name, conditions and settings with `opamp rules update <id> --file rule.json --expected-updated-at <updatedAt> --yes`. Preserve unrelated settings and triggers. `enabled: false` disables a rule; omitting enabled on update preserves it. Missing fields inside settings revert to inheritance, not their previous values. A 409 means someone changed the rule: list again and reapply the intended edit.
4. Verify with `opamp rules list`, then `opamp preview --file client.json` (`preview_sdk_config`). A preview file is `{"attributes":{"service.version":"2.0.0"},"instanceUid":"<browser-instance-UUID>"}`; omit instanceUid when rollout identity is irrelevant. Preview evaluates **saved** rules and applies plan limits. It returns settings, matchedRules, matchedCaptures, configHash and heartbeatIntervalSeconds. It is not a dry-run of an unsaved rule and does not prove any browser has applied it or uploaded data.
5. Report the saved rule id, effective settings and remaining verification. Browsers pick up changes at their next poll (normally 5 minutes) or SDK refresh. Developer ceilings and plan limits still apply. Do not promise every byte or session arrived merely because the rule saved.

For RUM requests, read [rum.md](rum.md). Aggregated metrics (stable SDK 2.0.0 and later) are independent of diagnostic signal modes. Do not enable full-session replay just to obtain Web Vitals. Check the installed SDK version; a browser `2.0.0-beta.N` prerelease ignores the metrics settings, while the Flutter 2.x prerelease applies `metrics.enabled`, `metrics.sampleRate` and `metrics.flushSeconds`.

A metrics-only policy overlay is `"settings":{"schemaVersion":1,"metrics":{"enabled":true,"sampleRate":1,"flushSeconds":300}}`. `sampleRate` accepts 0 or 0.001..1; `flushSeconds` accepts 30..3600. Counts are inverse-weighted estimates when sampled. Preserve existing signal/replay/buffer/trigger settings on replacement. Developer `captureMetrics: false` cannot be raised remotely. Verify an actual metrics upload and dashboard result after preview.

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
      "longTasks": { "mode": "stream", "sampleRate": 1 },
      "routeTiming": { "mode": "stream", "sampleRate": 1 },
      "custom": { "mode": "stream", "sampleRate": 1 }
    },
    "sessions": { "enabled": true, "baselineSampleRate": 1 },
    "replay": { "mode": "stream", "sessionSampleRate": 1, "maskAllInputs": true }
  }
}
```

Signals support `off`, `buffer` and `stream`; sample rates are 0..1. `sessions.baselineSampleRate` samples healthy breadcrumbs/network, **not replay**. Replay needs its own mode and sessionSampleRate. In a Flutter project `replay.captureMode` picks how replay is recorded, `video` (MP4 from the hardware encoder) or `wireframe` (the widget tree, replayed as HTML), and overrides the app's `screenCaptureMode`; unset keeps the app's choice, and browser SDKs ignore it. Linux and Windows have no video encoder and record a wireframe whatever it says. A change applies at once unless the session already uploaded replay in the other mode; then it applies from the next session. `settings.enabled: false` is a collection kill switch; `enabled: false` on the rule only skips that rule. Never send server-owned `limits` or `captures` in settings. Settings can also carry `buffer` and `triggers`; preserve them when changing an existing rule.

## Recording triggers: steps, selectors and routes

A recording trigger is a `triggers[]` entry whose steps are event matchers. The device matches them in order and then runs the actions:

```json
{
  "id": "checkout-declined",
  "name": "Pay, then a declined payment",
  "steps": [
    { "match": { "event": { "type": "click", "value": "#pay" } } },
    { "match": { "event": { "type": "navigation", "value": "/checkout/{id}" } }, "withinSeconds": 30 },
    { "match": { "event": { "type": "log", "value": "Payment declined", "mode": "contains" } }, "withinSeconds": 20 }
  ],
  "actions": [{ "type": "capture", "replay": true, "lookbackSeconds": 60, "durationSeconds": 30, "signals": ["console", "navigation", "userActions", "network"] }],
  "cooldownSeconds": 60,
  "maxPerSession": 3
}
```

An event `type` is `click`, `hover`, `navigation` or `log`; `value` is 1 to 256 characters; `mode` (`exact`, the default, or `contains`) applies to `log`. A trigger has at most five steps, and `withinSeconds` is measured from the step before. Give the trigger an `id` no other rule of the project uses.

The server checks the syntax only and answers 422 otherwise. A click or hover `value` is a selector made of tags, `#id`, `.class`, `[name]` or `[name="value"]` (operators `=`, `^=`, `$=`, `*=`, `~=`, `|=`) and the descendant, `>`, `+` and `~` combinators. It cannot contain a colon, a comma or a backslash, also inside a quoted value, and a value that is not a plain name must be quoted. A navigation `value` starts with `/`, has no `?`, `#` or `*`, and uses `{name}` only as a whole segment. The server cannot tell whether the selector or the route exists in the app, and the same document means different things in a browser and in a Flutter project. Check the project's framework before writing one.

| Step | Browser project | Flutter project |
|---|---|---|
| `click` | The clicked element or an ancestor matches the CSS selector | The widget under the finger, or a widget above it, matches the selector |
| `hover` | A pointer enters a matching element, never touch | A mouse pointer enters a matching widget, desktop only |
| `navigation` | The pathname of the page's URL | The path of the route the user arrives at (below) |
| `log` | A console line, an OpenTelemetry log record or `captureMessage` | A `print` / `debugPrint` line or `captureMessage` |

**Flutter selectors name widgets.** A Flutter app has no DOM, so no element ids, classes or `data-*` attributes exist. The selector syntax reads these properties of a widget instead:

| Selector | Matches a widget that |
|---|---|
| `ElevatedButton` | is of that type. Case and generic arguments are ignored; `*` is any widget |
| `#pay` | has `Key('pay')` (a string `ValueKey`), or is `Semantics(identifier: 'pay')` |
| `.button` | is a `Semantics` with that role: `button`, `link`, `header`, `image`, `textField`, `slider`, `checkbox`, `toggle`, `selected` |
| `[key="42"]` | has a `ValueKey` whose value reads `42` |
| `[identifier="x"]` | has that `Semantics.identifier` |
| `[label="Delete"]` | has that semantics label or tooltip (an `IconButton`'s tooltip, an `Icon`'s semantic label) |
| `[text="Pay now"]` | shows that text. Never the content of an input field |
| `[type="_Private"]` | is of a type that cannot be written as a tag |

No other attribute name matches anything. Combinators walk the element tree, where a button sits several elements above its `Text`, so write `#cart ElevatedButton`, not `#cart > ElevatedButton`. Prefer a key or a semantics identifier: a `[text=...]` selector stops matching when the wording or the language changes, and widget type names are minified in `--obfuscate` builds. `GlobalKey`, `ObjectKey` and `UniqueKey` give a widget no name.

Do not invent a selector. Take it from one of these:

- The app's source: `Key('...')`, `ValueKey(...)` and `Semantics(identifier: ...)` on the control.
- What the app already recorded. Every tap is a log record with event name `browser.user_action.click` whose `browser.css_selector` attribute is a selector the SDK checked against the tapped widget, for example `ElevatedButton#pay` or `ElevatedButton[text="Print Log Lines"]` (`app.widget.name`, `app.widget.id`, `app.widget.text` and `app.screen.name` are beside it). The **Actions** list of a session shows them.

**Flutter navigation names routes.** A Flutter app has no URL. A navigation step is compared with the path of the route the user arrives at, which is the first of: the route's name (`RouteSettings.name`: named routes, `onGenerateRoute`, routers that name their pages); `/` and the type of the page widget, such as `/CheckoutPage` for `MaterialPageRoute(builder: (_) => const CheckoutPage())`; `/` and the route's type, such as `/DialogRoute`. An app can also report a location with `Traceway.recordNavigation('/orders/8841')`.

- Matching is case-sensitive and by segment: `/users/{id}` matches `/users/42` and not `/users/42/edit`. A query string and a fragment are ignored, and a name without a leading slash (`checkout`) is read as `/checkout`.
- A push and a replace arrive at the new route, a pop arrives at the route it returns to, and a removal matches nothing.
- Navigation exists only where the app attached `Traceway.navigatorObserver` to its navigators, or calls `Traceway.recordNavigation`. Recorded navigations are log records named `browser.navigation` whose `url.full` is the path, so read real paths there before writing a pattern.
- Type-based paths are minified in `--obfuscate` builds. A route a trigger should match needs a name.

**Flutter logs have no level.** Every `print` line is recorded at info, so a trigger condition on `severity` above info never matches there. `captureUserActions: false`, `captureNavigation: false` and `captureLogs: false` in the app switch tap, navigation and `print`-line steps off, and replay needs `screenCapture: true` (video on Android, iOS or macOS; a wireframe on every platform).

After saving, say which widget and route the trigger names and that it is unverified until the journey is performed on a device: a fired trigger sends a `traceway.trigger.fired` log record and, when it asked for replay, the session lists it under **Recording triggers**.

Undo an update by restoring the saved document with the latest `updatedAt`. For a temporary change prefer disabling the added rule. `opamp rules delete <id> --yes` permanently removes it and exposes earlier rules/SDK defaults; those defaults can collect more or less. `opamp rules reorder <id>... --yes` requires every project rule exactly once and changes precedence. Re-list before reordering and preview representative clients afterward.
