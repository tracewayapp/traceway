# Browser, Flutter, iOS and Android SDK collection over OpAMP


Use this for an **existing browser frontend project** (React, Svelte, Vue or jQuery) running the JavaScript SDK v2 with remote config enabled (the default), a **Flutter project**, an **iOS project** or an **Android project** running that platform's 2.x SDK (2.1.0 is current) with a project token. SDK v2 is the stable browser release (npm `latest`, `@2`); 1.x clients (iOS 0.1.x) never poll, so they ignore these rules, and so does an app started without a token (standalone, `telemetry` only). It changes the backend's stored collection rules; it does not require a setup plan, a new project, a new project token, or edits to the app's start code. Backend projects and React Native projects reject remote config. Servers released before these agent routes reject them; upgrade the server instead of bypassing a 403. Targeted captures remain dashboard-only.

1. Select the project, then `traceway-write opamp rules list --output json` (`list_sdk_config_rules`). Keep the existing rule documents, ids and `updatedAt`. Inspect the order, conditions, triggers and kill switches before writing. Later matching rules override earlier ones; a newly created rule is appended.
2. State the requested scope: which clients, signals, sampling rates, replay and input masking. Empty `conditions: {}` matches all browsers. Target releases or cohorts with `{"all":[{"key":"service.version","op":"eq","value":"2.0.0"}],"rollout":{"percent":25,"by":"instance"}}`. Rollout is sticky per browser instance; no rollout means 100%. Do not broaden a targeted request to every client or remove masking unless explicitly requested.
3. Write a rule JSON file containing `name`, `enabled`, `conditions`, and `settings`. Create with `traceway-write opamp rules create --file rule.json --yes`. To edit, send the complete revised name, conditions and settings with `opamp rules update <id> --file rule.json --expected-updated-at <updatedAt> --yes`. Preserve unrelated settings and triggers. `enabled: false` disables a rule; omitting enabled on update preserves it. Missing fields inside settings revert to inheritance, not their previous values. A 409 means someone changed the rule: list again and reapply the intended edit.
4. Verify with `opamp rules list`, then `opamp preview --file client.json` (`preview_sdk_config`). A preview file is `{"attributes":{"service.version":"2.0.0"},"instanceUid":"<browser-instance-UUID>"}`; omit instanceUid when rollout identity is irrelevant. Preview evaluates **saved** rules and applies plan limits. It returns settings, matchedRules, matchedCaptures, configHash and heartbeatIntervalSeconds. It is not a dry-run of an unsaved rule and does not prove any browser has applied it or uploaded data.
5. Report the saved rule id, effective settings and remaining verification. Browsers pick up changes at their next poll (normally 5 minutes) or SDK refresh. Developer ceilings and plan limits still apply. Do not promise every byte or session arrived merely because the rule saved.

For RUM requests, read [rum.md](rum.md). Aggregated metrics (stable SDK 2.0.0 and later) are independent of diagnostic signal modes. Do not enable full-session replay just to obtain Web Vitals. Check the installed SDK version; a browser `2.0.0-beta.N` prerelease ignores the metrics settings, while the Flutter, Android and iOS 2.x SDKs apply `metrics.enabled`, `metrics.sampleRate` and `metrics.flushSeconds` to their `app.*` metrics.

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

Signals support `off`, `buffer` and `stream`; sample rates are 0..1. `sessions.baselineSampleRate` samples healthy breadcrumbs/network, **not replay**. Replay needs its own mode and sessionSampleRate. In a Flutter project `replay.captureMode` picks how replay is recorded, `video` (MP4, only when the app adds the `traceway_video` package) or `wireframe` (the widget tree, replayed as HTML), and overrides the app's `screenCaptureMode`; unset keeps the app's choice, and browser SDKs ignore it. The Android and iOS SDKs always record a wireframe (views and Compose screens, or the view hierarchy) and ignore it too. A Flutter app without `traceway_video`, and Linux and Windows, record a wireframe whatever it says. A change applies at once unless the session already uploaded replay in the other mode; then it applies from the next session. `replay.images` (`off`, `wifi` or `always`; SDK default `wifi`; **Replay images** in the rule editor of every browser and app project) sets what a replay shows of pictures. `off`: placeholders everywhere. Android, iOS and Flutter copy the pictures into the wireframe (never a screenshot) with `always`, and with `wifi` only on an unmetered connection at capture time (Flutter on a metered network keeps URL references to network images and uploads nothing). The browser with `wifi` replays pictures by their URLs and uploads no pixels, and with `always` also inlines small copies of the pictures the page may read. A copy is JPEG at quality 0.45 up to 384 px, or PNG up to 192 px when it has transparency, at most 48 KiB, 128 per recorder, 4 MiB per session; masked images never. The app's `replayImages` option is a ceiling, and the SDK applies the lower of the two. It merges like `maskAllInputs`. `settings.enabled: false` is a collection kill switch; `enabled: false` on the rule only skips that rule. Never send server-owned `limits` or `captures` in settings. Settings can also carry `buffer` and `triggers`; preserve them when changing an existing rule.

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

The server checks the syntax only and answers 422 otherwise. A click or hover `value` is a selector made of tags, `#id`, `.class`, `[name]` or `[name="value"]` (operators `=`, `^=`, `$=`, `*=`, `~=`, `|=`) and the descendant, `>`, `+` and `~` combinators. It cannot contain a colon, a comma or a backslash, also inside a quoted value, and a value that is not a plain name must be quoted. A navigation `value` starts with `/`, has no `?`, `#` or `*`, and uses `{name}` only as a whole segment. The server cannot tell whether the selector or the route exists in the app, and the same document means different things in a browser, a Flutter and an Android project. Check the project's framework before writing one.

| Step | Browser project | Flutter project |
|---|---|---|
| `click` | The clicked element or an ancestor matches the CSS selector | The widget under the finger, or a widget above it, matches the selector |
| `hover` | A pointer enters a matching element, never touch | A mouse pointer enters a matching widget, desktop only |
| `navigation` | The pathname of the page's URL | The path of the route the user arrives at (below) |
| `log` | A console line, an OpenTelemetry log record or `captureMessage` | A `print` / `debugPrint` line, a `package:logging` record, a `Traceway.log` line or `captureMessage` |

**Flutter selectors name widgets.** A Flutter app has no DOM, so no element ids, classes or `data-*` attributes exist. The selector syntax reads these properties of a widget instead:

| Selector | Matches a widget that |
|---|---|
| `ElevatedButton` | is of that type. Case and generic arguments are ignored; `*` is any widget |
| `#pay` | has `Key('pay')` (a string `ValueKey`), or is `Semantics(identifier: 'pay')` |
| `.button` | is a `Semantics` with that role: `button`, `link`, `header`, `image`, `textField`, `slider`, `checkbox`, `toggle`, `selected` |
| `[key="42"]` | has a `ValueKey` whose value reads `42` |
| `[identifier="x"]` | has that `Semantics.identifier` |
| `[id="pay"]` | the same as `#pay`: its string key, else its semantics identifier |
| `[label="Delete"]` | has that semantics label or tooltip (an `IconButton`'s tooltip, an `Icon`'s semantic label) |
| `[text="Pay now"]` | shows that text. Never the content of an input field |
| `[type="_Private"]` | is of a type that cannot be written as a tag |

No other attribute name matches anything. Combinators walk the element tree, where a button sits several elements above its `Text`, so write `#cart ElevatedButton`, not `#cart > ElevatedButton`. Prefer a key or a semantics identifier: a `[text=...]` selector stops matching when the wording or the language changes, and widget type names are minified in `--obfuscate` builds. `GlobalKey`, `ObjectKey` and `UniqueKey` give a widget no name.

Do not invent a selector. Take it from one of these:

- The app's source: `Key('...')`, `ValueKey(...)` and `Semantics(identifier: ...)` on the control. The mask markers `ValueKey('traceway-mask')` and `ValueKey('traceway-blank')` never name a widget, and a widget inside a mask records neither its text, label nor keys.
- What the app already recorded. Every tap is a log record with event name `browser.user_action.click` whose `browser.css_selector` attribute is a selector the SDK checked against the tapped widget, for example `ElevatedButton#pay` or `ElevatedButton[text="Print Log Lines"]` (`app.widget.name`, `app.widget.id`, `app.widget.text` and `app.screen.name` are beside it). The **Actions** list of a session shows them.

**Flutter navigation names routes.** A Flutter app has no URL. A navigation step is compared with the path of the route the user arrives at, which is the first of: the route's name (`RouteSettings.name`: named routes, `onGenerateRoute`, routers that name their pages); `/` and the type of the page widget, such as `/CheckoutPage` for `MaterialPageRoute(builder: (_) => const CheckoutPage())`; `/` and the route's type, such as `/DialogRoute`. An app can also report a location with `Traceway.recordNavigation('/orders/8841')`.

- Matching is case-sensitive and by segment: `/users/{id}` matches `/users/42` and not `/users/42/edit`. A query string and a fragment are ignored, and a name without a leading slash (`checkout`) is read as `/checkout`.
- A push and a replace arrive at the new route, a pop arrives at the route it returns to, and a removal matches nothing.
- Navigation exists only where the app attached `Traceway.navigatorObserver` to its navigators, or calls `Traceway.recordNavigation`. Recorded navigations are log records named `browser.navigation` whose `url.full` is the path, so read real paths there before writing a pattern.
- Type-based paths are minified in `--obfuscate` builds. A route a trigger should match needs a name.

**Flutter print lines have no level.** Every `print` / `debugPrint` line is recorded at info, so a trigger condition on `severity` above info never matches one; `package:logging` records and `Traceway.log(message, level)` lines carry their level. The dashboard's log-step builder offers no level for Flutter projects; a level condition can still be written in the rule's JSON. `captureUserActions: false`, `captureNavigation: false` and `captureLogs: false` in the app switch tap, navigation and log-line steps off, and replay needs `screenCapture: true` (a wireframe on every platform; video on Android, iOS or macOS with the `traceway_video` package).

**Android selectors name views and Compose nodes.** The same syntax reads the views (and Jetpack Compose semantics nodes) under the finger:

| Selector | Matches a view or Compose node that |
|---|---|
| `MaterialButton` | is of that class or a subclass (simple name, any case): `Button` matches a `MaterialButton`. A Compose node's type is its role (`Button`, `Checkbox`, `Switch`, `RadioButton`, `Tab`, `Image`, `DropdownList`) or its kind (`Text`, `TextField`, `Slider`, `ProgressIndicator`); `*` is any element |
| `#pay` | has `android:id="@+id/pay"` (the resource entry name), or `Modifier.testTag("pay")` |
| `.button` | has that role: `button`, `link`, `header`, `image`, `textField`, `slider`, `checkbox`, `toggle`, `selected` |
| `[key="42"]` | has the string tag `42` (`android:tag`), or that id or test tag |
| `[identifier="x"]` | the same values as `key` |
| `[label="Delete"]` | has that content description or tooltip, its own or the first one below it along single children |
| `[text="Pay now"]` | shows that text, its own or that of the views under it. An input's hint counts, its content never |
| `[type="..."]` | is of that full or simple class name, for a class that cannot be written as a tag |

A tap matches the view under the finger or one of its parents (the `MaterialButton` around the `TextView` that was hit). Combinators walk the view tree, so `#cart MaterialButton` works where `#cart > MaterialButton` may not. Prefer ids and test tags: class names are renamed in R8-minified builds (a renamed class is named by its nearest superclass that kept its name), while ids, test tags, roles, labels and text survive. Inside a mask (`view.tracewayMask`, `Traceway.mask(view, mode)`, `Modifier.tracewayMask()`, or an `android:tag` / `testTag` of `traceway-mask` or `traceway-blank`) a tap records neither text nor label, and a mask tag never names a view. Take selectors from the app's layouts and composables or from recorded taps: each is a `browser.user_action.click` record whose `browser.css_selector` the SDK checked against the tapped view (`MaterialButton#pay`, or `MaterialButton[text="Pay now"]` without an id), with `app.widget.name`, `app.widget.id`, `app.widget.text`, `app.widget.label` and `app.screen.name` beside it.

**Android navigation names screens.** A navigation step is compared with the screen the user arrives at: `/` and the class of the resumed activity (`/CheckoutActivity`) or of an AndroidX fragment that became visible (`/CartFragment`), or the path the app reports with `Traceway.recordNavigation("/orders/8841")`, which Compose Navigation apps call from their `NavController` listener. Matching is case-sensitive and by segment, as above. Recorded navigations are `browser.navigation` records whose `url.full` is the path; read real paths there before writing a pattern. Class-based paths are renamed in minified builds unless the class keeps its name; report a stable path for a screen a trigger must match.

**Android logs have levels.** `System.out` lines are info and `System.err` lines error; `Traceway.log(message, level, attributes)` takes a `TracewayLogLevel` (info by default). `android.util.Log` and Timber lines are captured at their level (`log.source: logcat`) when the app applies the Traceway Gradle plugin (`com.tracewayapp.symbols`, its `traceway { logcat { minLevel = "info" } }` block); without the plugin they are not. `captureUserActions`, `captureNavigation` and `captureLogs` set to false in `TracewayOptions` switch the matching steps off, and replay needs `screenCapture = true`.

**iOS selectors name views.** The same syntax reads the UIKit views under the finger; there is no hover on iOS, so a hover step never matches:

| Selector | Matches a view that |
|---|---|
| `UIButton`, `CheckoutButton` | is of that class: the name without module or generic arguments, any case; `*` is any view |
| `#pay` | has `accessibilityIdentifier` `pay` (the identifier UI tests use) |
| `.button` | has that role, from its accessibility traits or class: `button`, `link`, `header`, `image`, `textField`, `slider`, `toggle`, `selected` |
| `[key="42"]` | has `tag` 42 |
| `[identifier="x"]` | has that `accessibilityIdentifier` |
| `[label="Delete"]` | has that `accessibilityLabel`, its own or the first one below it along single subviews |
| `[text="Pay now"]` | shows that text: a label's, a button's title, or that of up to three labels under it; never what is typed into a field |
| `[type="..."]` | is of that class, for a class that cannot be written as a tag |

A tap matches the view under the finger or one of its superviews (the `UIButton` around the label that was hit). SwiftUI draws its controls without views of their own, and its `.accessibilityIdentifier` cannot be read: the app names a control with `.tracewayIdentifier("pay")`, a tap inside it is recorded as `#pay`, and `#pay` or `[identifier="pay"]` match it. Without one, a tap on a SwiftUI control names the hosting view. Inside a mask (`tracewayMask`, or a UIKit `accessibilityIdentifier` of `traceway-mask` or `traceway-blank`) or a secure text field a tap records neither text nor label, and a mask identifier never names a view. Take selectors from recorded taps: each is a `browser.user_action.click` record whose `browser.css_selector` the SDK checked against the tapped view (`UIButton#pay`, or `UIButton[text="Pay now"]` without an identifier), with `app.widget.name`, `app.widget.id`, `app.widget.text`, `app.widget.label` and `app.screen.name` beside it.

**iOS navigation names screens.** A navigation step is compared with the screen the user arrives at: `/` and the class of the view controller that appeared (`/CheckoutViewController`; a hosting controller is its SwiftUI root view, `/ContentView`), or the path the app reports with `.tracewayScreen("/checkout")` on a SwiftUI view or `Traceway.recordNavigation("/orders/8841")`. A SwiftUI app records no screens by itself (its window and `NavigationStack` use SwiftUI's own controllers), so its navigation steps need `.tracewayScreen`; once a view uses it, hosting controllers are no longer screens. Containers (navigation, tab, split and page controllers), child view controllers and the system's own controllers are not screens. Matching is case-sensitive and by segment, as above.

**iOS logs have levels.** `Traceway.log(_:level:attributes:)` takes a `TracewayLogLevel` (info by default); `NSLog`, `os_log` and `Logger` lines come from the unified log on iOS 15 and later at their level (`log.source: oslog`), and `print` only with `captureStdout: true` (`stdout` at info, `stderr` at error). `Traceway.captureMessage(_:attributes:)` is a message log step too. `captureUserActions`, `captureNavigation` and `captureLogs` set to false in `TracewayOptions` switch the matching steps off, and replay (a wireframe of the view hierarchy) needs `screenCapture: true`.

After saving, say which widget and route the trigger names and that it is unverified until the journey is performed on a device: a fired trigger sends a `traceway.trigger.fired` log record and, when it asked for replay, the session lists it under **Recording triggers**.

Undo an update by restoring the saved document with the latest `updatedAt`. For a temporary change prefer disabling the added rule. `opamp rules delete <id> --yes` permanently removes it and exposes earlier rules/SDK defaults; those defaults can collect more or less. `opamp rules reorder <id>... --yes` requires every project rule exactly once and changes precedence. Re-list before reordering and preview representative clients afterward.
