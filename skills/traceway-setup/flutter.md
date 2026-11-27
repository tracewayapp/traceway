# Flutter SDK Reference

The Traceway Flutter SDK is the [`traceway`](https://pub.dev/packages/traceway) package on pub.dev, version 2.1.0. It is built on the OpenTelemetry SDK for Dart and sends Dart errors, logs, request spans, sessions, `app.*` performance metrics and session replay to the project. It runs on iOS, Android, macOS, Linux and Windows, not on Flutter web (see the end of this file). The package has no native code; video replay is the separate `traceway_video` add-on. Docs: https://docs.tracewayapp.com/client/flutter

## Choose the version

- **New integration**: install 2.1.0. It needs Traceway Cloud or a self-hosted server whose Flutter projects accept OTLP and OpAMP (server 2.0 or later). On an older self-hosted server, report that and install 1.x (`traceway: ^1.0.9`) only if the user agrees.
- **Existing app**: read the `traceway` entry in `pubspec.lock`.
  - 2.x (`2.0.0-beta.N`, or a Git dependency with `ref: v2-session-capture`): move it to `^2.1.0` and apply "Upgrading from 2.0" below.
  - 1.x (for example `1.0.9`) is an older project. Keep it as it is unless the user asks to upgrade or wants something only 2.x has (Data Collection rules, captures and recording triggers, performance metrics and the Flutter Performance dashboard, wireframe replay, trace propagation, sending telemetry to another OpenTelemetry provider). An upgrade also deletes the options 2.x removed: `debounceMs`, `retryDelayMs`, `maxPendingExceptions`, `maxLocalFiles`, `localFileMaxAgeHours`, `eventsWindow`, `eventsMaxCount`.

`curl -s https://pub.dev/api/packages/traceway | jq -r .latest.version` shows the newest release. If `flutter pub add` cannot resolve `^2.1.0`, say so instead of picking another version.

## Install

```bash
flutter pub add 'traceway:^2.1.0'
```

Requirements: Flutter 3.38.1 or later (Dart 3.10). No Gradle or CocoaPods change is needed, but a release build needs the network permissions in "Platform permissions" below. Add `traceway_video` only when the user wants video replay (see "Session replay").

## Initialize

Wrap the app in `Traceway.run` in `main()` and attach the navigator observer. This is the code the project's Connection page shows:

```dart
import 'package:flutter/material.dart';
import 'package:traceway/traceway.dart';

void main() {
  Traceway.run(
    token: const String.fromEnvironment('TRACEWAY_TOKEN'),
    options: TracewayOptions(
      screenCapture: true,
      version: '1.0.0',
    ),
    child: MyApp(),
  );
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) => MaterialApp(
        navigatorObservers: [Traceway.navigatorObserver],
        home: const HomePage(),
      );
}
```

- The SDK sends to Traceway Cloud (`https://cloud.tracewayapp.com`) by default. For a self-hosted server add `baseUrl: const String.fromEnvironment('TRACEWAY_URL'),` under `token:`. An empty `baseUrl` means Cloud.
- `version` becomes `service.version`; set it from the app's real version so rules and release comparisons work. `environment` becomes `deployment.environment.name`.
- `Traceway.run` starts the client, runs the app in a zone that catches uncaught async errors and `print` lines, installs `FlutterError.onError` and `PlatformDispatcher.onError`, and wraps the app in the `Traceway` widget that records taps and replay.
- **Navigator observer.** Without it the SDK still records errors, taps and replay, but no screen names. With go_router: `GoRouter(observers: [Traceway.navigatorObserver], routes: [...])`. One observer watches one `Navigator`: a nested one (tabs, a go_router `ShellRoute`) gets its own `Navigator(observers: [TracewayNavigatorObserver()], ...)`.
- An app that sets up the Flutter binding before `Traceway.run` (to await plugins) keeps its own zone: its uncaught async errors are still recorded through `PlatformDispatcher.onError`, but its `print` lines are not captured.
- An app that calls `runApp` itself can start the client alone with `TracewayClient.initialize(token: ..., baseUrl: ..., options: ...)`, but then gets no `Traceway` widget and no zone.
- Settings the SDK cannot use never take the app down. A missing token (often a build without the env file below) or a malformed `telemetry` URL is reported as a `FlutterError` (library `traceway`) in the debug console, and the app runs without the SDK. A `baseUrl` that is not an HTTP(S) URL prints one warning and leaves the SDK off. Check the debug console after setup.

### Where the token comes from

The project token is on the project's **Connection** page. The dashboard and `traceway setup` put it in a `.env` file at the Flutter app root (next to `pubspec.yaml`), which the build turns into compile-time constants:

```ini
TRACEWAY_TOKEN=YOUR_TOKEN
TRACEWAY_URL=https://traceway.mycompany.com
```

`TRACEWAY_URL` is written only for a self-hosted server; on Traceway Cloud the file has `TRACEWAY_TOKEN` alone. Run and build with the file:

```bash
flutter run --dart-define-from-file=.env
flutter build apk --dart-define-from-file=.env
```

- Add `.env` to `.gitignore` (the Flutter template does not ignore it) and never commit it.
- Every build that should report needs the flag: CI build steps, release scripts, and IDE run configurations (`"args": ["--dart-define-from-file=.env"]` in VS Code's `launch.json`). In CI, write the file from a secret or pass `--dart-define=TRACEWAY_TOKEN=$TRACEWAY_TOKEN`.
- A token compiled into an app can be read out of it. That is expected for a project token, which can only send data.

The `traceway setup` plan entry for a Flutter app:

```json
{"name": "Shop App", "framework": "flutter", "envFile": "app/.env", "envVar": "TRACEWAY_TOKEN"}
```

- The apply step writes `TRACEWAY_TOKEN` into that file and, off Cloud, also `TRACEWAY_URL` (the URL variable defaults to `envVar` with `TOKEN` replaced by `URL`; `urlEnvVar` names another one).
- The project also gets a symbol upload token. It is written only when the plan names `uploadTokenEnvFile` or `uploadTokenEnvVar` (default variable `TRACEWAY_UPLOAD_TOKEN`, plus `TRACEWAY_URL` off Cloud). Give it its own gitignored file, `"uploadTokenEnvFile": "app/.env.symbols"`, or leave it on the Connection page for a CI secret. Do not name only `uploadTokenEnvVar`: the token then goes into `envFile`, the file the build compiles in with `--dart-define-from-file`.
- Plan an upload token only when the release build is obfuscated (see "Symbol upload").

### Upgrading from 2.0

- A 2.0 app passes `connectionString:` to `Traceway.run`. It still works on 2.1.0: the SDK reads the token and the server out of it, in any older spelling, and prints one deprecation warning. Replace it with `token:` (and `baseUrl:` off Cloud), and move the value into the `.env` file above.
- `TracewayClient.initialize` now takes named parameters, `initialize(token:, baseUrl:, options:)`.
- `TracewayOptions.legacyConnectionString` is removed: delete it. Reports a 1.x build left on the device are sent once through the normal 2.x path.
- Video replay moved to the `traceway_video` add-on and the default `screenCaptureMode` is now `wireframe`. An app that used video adds the add-on (see "Session replay"). An app that does not use video removes the `flutter_quick_video_encoder` block from `android/build.gradle.kts`, or Gradle fails on a project that no longer exists.

## What is captured automatically

- **Dart errors**: framework errors (`FlutterError.onError`), unhandled root-isolate errors (`PlatformDispatcher.onError`) and uncaught async errors (the `Traceway.run` zone), each with its Dart stack. After recording, an error is handed on as it would be without the SDK (to an outer zone, or to the app's own `PlatformDispatcher.onError`, such as Crashlytics).
- **Not captured: native crashes, ANRs and hang stacks.** A crash in a plugin's Kotlin, Java, Swift, Objective-C or C code, or in the engine, ends the process before Dart runs. Hangs are only the `app.hang` metric: Flutter reports no `AppHang` issue, because no stack can be taken from the blocked UI isolate. For native crashes, ANRs and hangs with stacks, add the Traceway Android and iOS SDKs to the host apps (`android/`, `ios/`): see `android.md` and `ios.md`.
- **Logs**, each with `log.source`: `print` / `debugPrint` lines as info (`print`), every `package:logging` record with its level and logger name (`logging`), and `Traceway.log` (`api`). `dart:developer.log` is not captured.
- **Network**: every dart:io HTTP request (`package:http`, Dio and anything else on `HttpClient`) as a client span, through `HttpOverrides.global`. Plugins that call a native SDK over a platform channel (Firebase) are not seen. A client on the platform's own stack (`cupertino_http`, `cronet_http`) must be wrapped in `TracewayHttpClient(inner)`.
- **Navigation** from `Traceway.navigatorObserver`. Dialogs, sheets and menus are navigations with `navigation.overlay: true` and do not change the current screen.
- **Taps**, with a selector of the widget (`browser.user_action.click`).
- **Screen context** on every exception and message: `app.screen.name`, `app.screen.route` (the route template, when known), `app.screen.previous`, `app.screen.duration_ms` and `viewport`.
- **Sessions** and **app performance metrics**: cold and hot start (`app.start.duration`), time to initial and full display per screen (`app.screen.ttid`, `app.screen.ttfd`), slow and frozen frames, `app.hang`, unhandled errors (`app.crash`), sessions, users, peak memory and HTTP requests. They are aggregated on the device, uploaded every five minutes and when the app goes to the background, and fill the project's **Flutter Performance** dashboard.

Production defaults, keep them: errors and messages stream; logs, requests, navigation, taps, custom events and replay are buffered on the device (2 MiB / 10 minutes) and an exception uploads the last 10 minutes of them with its replay clip (at most one clip per 5 seconds and 20 per session). Identical errors within a second are sent once, at most 60 errors leave per minute, and network failures (`SocketException`, timeouts, 4xx statuses: `defaultIgnoreErrors`) are not reported. Do not set `recordAllSessions` to prove that setup works.

**The SDK never takes the app down.** Every public call, hook, timer and background task runs through one guarded helper, so a failure inside the SDK is contained (printed once with `debug: true`) and the app's own operation goes on unchanged.

### Screen and tap names

Screens and taps are named from the app's code, and the Data Collection page's recording triggers match those names.

- **Screens.** A route is reported by its `RouteSettings.name` (for example `/users/42`), else `/` plus the page widget's type (`/CheckoutPage`), else `/` plus the route's type (`/DialogRoute`). Types are minified in `--obfuscate` builds, so name the routes of an obfuscated app (`MaterialPageRoute(settings: const RouteSettings(name: '/checkout'), ...)`).
- **Route templates.** Metrics group screens by template (`/orders/:id`). Without one, id-like segments are replaced (`/orders/8841` becomes `/orders/:id`). A router that does not report to a `NavigatorObserver`, or that knows its templates, reports itself:

  ```dart
  router.routerDelegate.addListener(() {
    final match = router.routerDelegate.currentConfiguration;
    Traceway.recordNavigation(match.uri.path, route: match.fullPath);
  });
  ```

- **Taps.** Each tap is recorded with the best selector the control has: a string `Key` or `Semantics(identifier:)` first (`ElevatedButton#pay`), then another key, the shown text (`ElevatedButton[text="Pay now"]`), the label, and the type alone. A control a trigger should match needs a `Key` or a `Semantics(identifier:)`. Add keys only to the controls the user wants to trigger on; do not key the whole app unasked.

## Manual capture

All of these are static methods on `Traceway`. They do nothing before the SDK has started, and they never throw into the app.

```dart
// Errors and messages: pass the stack trace from catch (e, st), or the issue has no frames
try {
  await placeOrder(cart);
} catch (error, stack) {
  Traceway.captureException(error, stack, {'order.step': 'payment'});
}
Traceway.captureMessage('Checkout opened with an empty cart');

// Log lines with a level: trace, debug, info (default), warn, error, fatal
Traceway.log('payment declined',
    level: TracewayLogLevel.warn, attributes: {'provider': 'stripe'});

// Breadcrumbs and named events that triggers can match
Traceway.recordAction(category: 'cart', name: 'add_item', data: {'sku': 'SKU-123'});
Traceway.track('checkout_failed', {'reason': 'card_declined'});

// The user and other attributes on every later record; rules and captures can target user.id
Traceway.setAttribute('user.id', user.id);
Traceway.setAttributes({'plan': 'pro'});
Traceway.removeAttribute('plan');
Traceway.clearAttributes();

// One user action as a span; requests inside run() are its children
final checkout = Traceway.startInteraction('checkout', attributes: {'cart.items': 3});
try {
  await checkout.run(() => api.placeOrder(cart));
} catch (error, stack) {
  checkout.captureException(error, stack);
} finally {
  checkout.end();
}

// Spans of your own (a Tracer of package:dartastic_opentelemetry); null before start
final span = Traceway.tracer?.startSpan('load-catalog');
try {
  await loadCatalog();
} finally {
  span?.end();
}

// Screens and navigation
Traceway.setRoute('/orders/:id');
Traceway.recordNavigation('/orders/42', from: '/orders', action: 'push', route: '/orders/:id');
Traceway.reportFullyDisplayed(); // the screen shows its real content

// Metrics of your own, charted like the built-in ones
Traceway.metric('checkout.duration', 1234, {'plan': 'pro'});
Traceway.count('cart.add');
Traceway.setMetricDimension('experiment', 'b'); // at most two; never a user id

await Traceway.flush(5000);   // send what is queued, waiting at most 5000 ms
await Traceway.refreshConfig(); // ask for remote settings now, e.g. after sign-in
```

`Traceway.currentSessionId` is the current `session.id`, `Traceway.getInstanceId()` the installation id a dashboard capture targets, `Traceway.ready` completes when the SDK has started, and `Traceway.shutdown()` stops it. There is no `setUser`: a user is the `user.id` attribute. `Traceway.client` (`TracewayClient.instance`) has the same methods, except that `metric` and `count` are `recordMetric` and `recordCount` there and `recordNavigation` exists only on `Traceway`.

## Options

`TracewayOptions`, all optional. The `capture*` options, `screenCapture`, `replayImages` and `sessions` are ceilings: Data Collection rules can lower collection but never turn on what the app turned off. `sampleRate`, `baselineSampleRate`, `recordAllSessions` and `screenCaptureMode` are defaults that a rule can change either way.

| Option | Default | Description |
|--------|---------|-------------|
| `sampleRate` | `1.0` | Share of exceptions and messages reported |
| `debug` | `false` | Print debug info to the console, including why something was dropped |
| `version` | `''` | `service.version` (`unknown` when empty) |
| `serviceName` | `flutter-app` | `service.name` |
| `environment` | none | `deployment.environment.name` |
| `attributes` | `{}` | On every record; also used for rule targeting |
| `screenCapture` | `false` | Record session replay (see "Session replay") |
| `replayImages` | `TracewayImagePolicy.always` | The most a wireframe may show of images; ceiling of the remote `replay.images` |
| `fps` | `15` | Replay frames per second (1 to 59) |
| `maxBufferFrames` | `150` | Frames kept on the device; the upper bound of every clip |
| `persistToDisk` | `true` | Keep SDK state and undelivered telemetry on disk across restarts |
| `captureLogs` | `true` | `package:logging`, `Traceway.log`, `print` / `debugPrint` |
| `captureNetwork` | `true` | Install `HttpOverrides.global` to record dart:io requests |
| `captureNavigation` | `true` | Record navigations from the observer and `recordNavigation` |
| `captureUserActions` | `true` | Record taps and feed taps and hovers to recording triggers |
| `captureMetrics` | `true` | `app.*` performance metrics (Flutter Performance dashboard) |
| `metricDimensions` | `{}` | Up to two dimensions of your own on every metric (`custom.<key>`) |
| `recordAllSessions` | `false` | Stream breadcrumbs and replay all the time instead of keeping them for errors |
| `collection` | none | Local collection settings, the same document as a Data Collection rule |
| `sessions` | `true` | Send session records; the session id exists either way |
| `baselineSampleRate` | `0` | Share of sessions whose breadcrumbs stream continuously |
| `remoteConfig` | `true` | Poll the server over OpAMP for Data Collection rules |
| `exportDelayMs` | `1000` | Batching delay for logs and spans |
| `telemetry` | none | Send logs, spans and metrics to another OpenTelemetry provider (below) |
| `propagateTraceHeaderUrls` | `[]` | URLs (`String` prefix or `RegExp`) whose requests carry a W3C `traceparent` |
| `ignoreErrors` | `defaultIgnoreErrors` (network failures) | Patterns matched against the first line of `Type: message`; `[]` reports everything |
| `beforeCapture` | none | `bool Function(TracewayCapturedError)`; return `false` to drop an exception or message |
| `screenCaptureMode` | `TracewayScreenCaptureMode.wireframe` | `.wireframe` or `.video` (needs `traceway_video`); remote `replay.captureMode` overrides it |
| `capturePixelRatio` | `0.75` | Video frame resolution scale (video add-on only) |
| `captureDeviceInfo` | `true` | Device model, name, local IP, locale, screen, runtime; `false` keeps only `os.name` and `os.version` |
| `reportHangsInDebug` | `false` | Report hangs in debug builds too |
| `captureAppHangs` | `true` | Measure hangs for the `app.hang` metric (no `AppHang` issue on Flutter) |
| `appHangThresholdMs` | `2000` | How long the UI isolate must be blocked to count as a hang (at least 250) |
| `isUnmeteredNetwork` | none | `FutureOr<bool> Function()`: whether the device is on an unmetered network, for `replay.images = wifi` |

To link app requests to backend traces, set `propagateTraceHeaderUrls` to the app's own API origins only (for example `['https://api.example.com/']`). It is empty by default because an unexpected header can break a strict CORS or signature check, and requests to Traceway or the `telemetry` receivers never get it. The backend must extract `traceparent` (OpenTelemetry HTTP instrumentation does).

## Session replay

`screenCapture: true` turns replay on. Nothing is uploaded until an exception is captured (it gets a clip of up to the last 30 seconds, cut to the frames held) or a capture or rule streams replay (consecutive segments of about 8 seconds). Replay uploads count against a per-session budget of 50 MiB and 10,000 events.

- **Wireframe** (default, every platform including Linux and Windows): the widget tree recorded as HTML and rrweb mutations, a few kilobytes per segment, with no screenshots and no native code. It is not pixel-exact: custom painting is a dashed outline, platform views (web views, maps) and video textures are labeled placeholders. While the soft keyboard is up, a neutral "Keyboard" box is drawn at the bottom, as on Android and iOS.
- **Video** (Android, iOS, macOS): the app's pixels as MP4, from the separate add-on. Add it only when the user asks for video:

  ```bash
  flutter pub add 'traceway_video:^2.1.0'
  ```

  ```dart
  import 'package:traceway_video/traceway_video.dart';

  void main() {
    TracewayVideo.register();
    Traceway.run(
      token: const String.fromEnvironment('TRACEWAY_TOKEN'),
      options: const TracewayOptions(
        screenCapture: true,
        screenCaptureMode: TracewayScreenCaptureMode.video,
      ),
      child: const MyApp(),
    );
  }
  ```

  Without `TracewayVideo.register()`, video mode prints one warning that says what to add and records a wireframe; so does a platform without an encoder (Linux, Windows). The add-on's encoder needs a Gradle change on Android and CocoaPods on iOS and macOS (see "Platform permissions").
- A Data Collection rule can switch a client between video and wireframe (`replay.captureMode`); one session's replay stays one kind.

**Masks** change the recording and tap records only, never the live app:

```dart
TracewayMask(child: Text('4242 4242 4242 4242'))                    // blur: layout kept, text as *, images as placeholders
TracewayMask(mode: TracewayMaskMode.blur(ratio: 0.5), child: CreditCardWidget())
TracewayMask(mode: TracewayMaskMode.blank(color: Color(0xFF000000)), child: SensitiveDataWidget())

// No wrapper: a marker key
Text(card.number, key: const ValueKey('traceway-mask'))
ListTile(key: const ValueKey('traceway-blank'), title: Text(account.iban))
```

- A marker key replaces the widget's own key, and the children of one `Row`, `Column` or `Stack` need distinct keys, so mark one of them that way and wrap the others in `TracewayMask`.
- `TracewayPrivacyMask`, the 1.x and 2.0.0-beta.1 name, is a deprecated alias of `TracewayMask`.
- Text typed into fields is replaced with `*` in wireframes and pixelated in video unless a rule sets `replay.maskAllInputs: false`; obscured fields are always masked.
- The shown text, label and keys of a masked widget are left out of tap records.

**Replay images.** The remote `replay.images` setting (`off`, `wifi` by default, `always`) decides what a wireframe shows of images, capped by the `replayImages` option:

| `replay.images` | Network | What the wireframe shows |
|---|---|---|
| `off` | any | A gray placeholder box; no URL, no pixels |
| `wifi` (default) | metered or unknown | Network images (`Image.network`, `NetworkImage`) as an `<img>` with their URL, fetched when the replay plays; other images as placeholders. Nothing is uploaded |
| `wifi` | unmetered (Wi-Fi, Ethernet) | The pixels of every image (network, asset, memory, file) inlined as a small picture |
| `always` | any | The same inlined pictures |

- A picture is made from the image's decoded pixels at its displayed size (one pixel per logical pixel), never a screenshot. Opaque pictures are JPEG at quality 0.45, longest side at most 384 px. A picture with transparency is never JPEG (it would replay as a box): it is PNG, longest side at most 192 px. One over 48 KiB is tried once at half size, then left out.
- At most 128 pictures per recorder (the one shown longest ago makes room) and 4 MiB per session. A `DecorationImage` (a `BoxDecoration` image, `CircleAvatar.backgroundImage`) keeps its URL or placeholder. Images inside a mask are always placeholders. Video is not affected.
- Unmetered is what `isUnmeteredNetwork` returns when the app sets it (for example from `connectivity_plus`); otherwise the SDK guesses from the network interfaces that are up (Android `wlan*`/`eth*`, iOS `en0`, desktop Wi-Fi and Ethernet), and counts the network as metered until it knows.

## Send telemetry to another OpenTelemetry provider

`telemetry` sends ordinary logs, request spans and metrics to any OTLP/HTTP receiver (an OpenTelemetry Collector, Grafana, Honeycomb). With a token, errors, `captureMessage` issues, session records, replay and remote config stay on Traceway, and both sides carry `session.id`:

```dart
Traceway.run(
  token: const String.fromEnvironment('TRACEWAY_TOKEN'),
  options: const TracewayOptions(
    telemetry: TracewayTelemetry(
      url: 'https://otlp.your-provider.com', // /v1/logs, /v1/traces, /v1/metrics are appended
      headers: {'x-api-key': String.fromEnvironment('OTLP_KEY')},
      // logs:, traces:, metrics: TracewayTelemetryEndpoint(url:, headers:) take complete URLs
    ),
  ),
  child: const MyApp(),
);
```

- Without a shared `url`, a signal you do not list keeps its Traceway destination. URLs must be HTTP(S) without credentials, a query or a fragment. The Traceway token is never sent to the receiver.
- Leaving out the token runs the SDK standalone: `telemetry` then needs a `url` or all three signal endpoints, every record goes to the receiver (exceptions as log records with the event name `exception`), and remote config, replay uploads and session records are off.
- The receiver must accept OTLP/HTTP protobuf (gzip-compressed) and delta sums and exponential histograms. A key built into the app can be read out of it: use a write-only ingestion key or a collector that holds the backend credentials.
- Do not set this up unless the user asks for it.

## Symbol upload

A plain `flutter build --release` keeps readable stack traces (function names and `file:line`), and nothing needs uploading. Symbol upload is needed only when the release build passes `--obfuscate` and/or `--split-debug-info=<dir>`: traces then arrive as bare offsets until Traceway resolves them from the build's `.symbols` files.

1. **Check the build.** Read the `flutter build` commands in CI (`.github/`), `fastlane/`, a `Makefile` or scripts, and ask the user how they build releases. Without either flag, skip this section.
2. **Use the upload token**, not the project token. It is on the project's Connection page under **Symbol Upload** (or written by `traceway setup` when the plan asks, see "Where the token comes from"). It is a CI secret, never committed and never compiled into the app.
3. **Configure the uploader once.** On a self-hosted server, add the URL to `pubspec.yaml` (Cloud is the default):

   ```yaml
   traceway:
     url: https://traceway.mycompany.com
   ```

4. **Upload on every release**, right after the build:

   ```bash
   flutter build apk --release --obfuscate --split-debug-info=build/symbols --dart-define-from-file=.env
   TRACEWAY_UPLOAD_TOKEN=YOUR_UPLOAD_TOKEN dart run traceway:upload_symbols
   ```

   ```yaml
   - name: Upload Flutter symbols
     run: dart run traceway:upload_symbols
     env:
       TRACEWAY_UPLOAD_TOKEN: ${{ secrets.TRACEWAY_UPLOAD_TOKEN }}
       TRACEWAY_URL: ${{ secrets.TRACEWAY_URL }} # self-hosted only
   ```

- The uploader finds `build/symbols` and uploads every architecture in one run. Each value resolves from a flag (`--token`, `--url`, `--symbols-dir`), then an environment variable (`TRACEWAY_UPLOAD_TOKEN`, `TRACEWAY_URL`), then the `pubspec.yaml` block. It does not read `.env` files: with a local `.env.symbols`, run `set -a; . ./.env.symbols; set +a` first. `--dry-run` previews the upload but still needs a token.
- Symbols are unique per build, so a crash resolves only against the exact build it came from.
- On iOS and macOS the uploader reads the debug id from the built `.app` (found under `build/`, or `--app build/macos/Build/Products/Release/YourApp.app`), so run it on the Mac that built the app.
- Self-hosted servers need blob storage (S3 or a persistent volume), or uploaded symbols disappear when the container is recreated.

## Verify

Set the dashboard time picker to the last 15 minutes.

1. Run the app with the env file. The debug console shows no `FlutterError` from library `traceway`; with `debug: true` it logs the start.
2. Add a button that throws (`throw StateError('Test error from Traceway')`), tap it, and open `/issues`: the error is listed in the Flutter project only, with its Dart stack and the screen it happened on. With `screenCapture: true` the issue has a replay clip.
3. Open `/sessions`: the session is listed with its breadcrumb timeline (navigations, taps, logs, requests).
4. Send the app to the background (or wait five minutes), then open the **Flutter Performance** dashboard under `/dashboards`: app start, screen display times and frames have data.
5. For Data Collection, add a temporary rule on `/data-collection` (for example console streamed), restart the app or call `Traceway.refreshConfig()`, check that it applies, then remove the rule. The SDK polls at start, every 5 minutes, and when the app returns to the foreground and a poll is due.
6. For an obfuscated release build, confirm that a test error shows real frames once that build's `.symbols` files are uploaded.

## Platform permissions

- **Android**: add the `INTERNET` permission to `android/app/src/main/AndroidManifest.xml`. Flutter's template declares it only in the debug and profile manifests, so without it a release build sends nothing.

  ```xml
  <uses-permission android:name="android.permission.INTERNET"/>
  ```

- **macOS**: add `com.apple.security.network.client` (`<true/>`) to both `macos/Runner/DebugProfile.entitlements` and `macos/Runner/Release.entitlements`.
- **iOS**: nothing to add.
- **A local or plain-HTTP server while testing** (`traceway setup` then writes the server's own address, for example `TRACEWAY_URL=http://localhost:8082`): the iOS Simulator and desktop builds reach it as written; an Android emulator or USB device does after `adb reverse tcp:8082 tcp:8082` (else the emulator reaches the host as `10.0.2.2`); a phone on Wi-Fi needs the host's LAN address. Plain `http://` needs a debug-only cleartext exception on Android 9+ (a network security config in `android/app/src/debug/`, see `android.md`) and `NSAppTransportSecurity` › `NSAllowsLocalNetworking` in `ios/Runner/Info.plist` on iOS. Traceway Cloud needs none of this.
- **`traceway_video` only**: its encoder plugin, `flutter_quick_video_encoder` 1.7.2, declares `compileSdk` 33, and a current Flutter template fails the build (`:flutter_quick_video_encoder is currently compiled against android-33`). Add this to `android/build.gradle.kts`, above `subprojects { project.evaluationDependsOn(":app") }`:

  ```kotlin
  project(":flutter_quick_video_encoder") {
      plugins.withId("com.android.library") {
          extensions.getByType<com.android.build.api.variant.LibraryAndroidComponentsExtension>().finalizeDsl {
              it.compileSdk = maxOf(it.compileSdk ?: 0, 34)
          }
      }
  }
  ```

  The plugin is CocoaPods only, so an iOS or macOS app built with Swift Package Manager alone needs CocoaPods for it.

## Flutter web

The SDK does not run on Flutter web: it needs dart:io. In a web build `Traceway.run` prints one line and runs the app without the SDK. For the web build, create a separate browser project (React, Svelte, Vue.js or jQuery in the framework picker; a Flutter project is a mobile project) and add the JS SDK to `web/index.html` with that project's token:

```html
<script src="https://cdn.jsdelivr.net/npm/@tracewayapp/frontend@2.1.0/dist/traceway.iife.global.js"></script>
<script>
  Traceway.init({ token: "YOUR_TOKEN" });
</script>
```

For a self-hosted server add `baseUrl: "https://traceway.mycompany.com"` next to `token`. The JS SDK records the page's `fetch` and `XMLHttpRequest` calls, which Dart's HTTP clients use in a web build. Its replay records the DOM, and Flutter web paints into a canvas, so a replay shows little of the app.
