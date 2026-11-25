# Flutter SDK Reference

The Traceway Flutter SDK is the [`traceway`](https://pub.dev/packages/traceway) package on pub.dev. Error tracking works on iOS, Android, macOS, Linux and Windows; screen recording needs a hardware video encoder and works on iOS, Android and macOS only (the 1.x docs list only iOS, Android and macOS). It does NOT support Flutter web (see the bottom of this file).

## Choose the version

There are two generations with the same entry point (`Traceway.run`) and connection string:

- **1.x** (newest stable, `1.0.9`): errors with the last ~10 seconds of logs, actions and screen recording, sent to `/api/report`. `flutter pub add traceway` installs it.
- **2.x** (the 2.x prerelease, `2.0.0-beta.N`): rebuilt on the OpenTelemetry SDK for Dart. Logs and spans go as OTLP/HTTP to `/api/otel/v1/logs` and `/api/otel/v1/traces`, performance metrics to `/api/otel/v1/metrics`, screen recordings to `/api/recordings`, and the SDK polls `/api/otel/v1/opamp` for the Flutter project's **Data Collection** rules. It needs a Traceway server that accepts Flutter projects on those endpoints; there is no fallback to `/api/report`. Install it with `flutter pub add 'traceway:{"git":{"url":"https://github.com/tracewayapp/traceway-flutter.git","ref":"v2-session-capture"}}'`.

Read the installed version from `pubspec.lock`. Keep an existing app on its major, and move a 1.x app to 2.x only when the user asks for it or for something only 2.x has (remote config, captures and recording triggers, performance metrics and the Flutter Performance dashboard, trace propagation, sending telemetry to another OpenTelemetry backend). A new integration installs the 2.x prerelease with `flutter pub add 'traceway:{"git":{"url":"https://github.com/tracewayapp/traceway-flutter.git","ref":"v2-session-capture"}}'`, the command the project's Connection page shows, when the Traceway server accepts Flutter projects on OTLP and OpAMP; on an older server it installs 1.x. `curl -s https://pub.dev/api/packages/traceway` shows the stable release as `latest`; the 2.x preview is installed from Git. Commit the resolved revision in the app’s pubspec.lock. Moving from 1.x to 2.x removes options (`debounceMs`, `retryDelayMs`, `maxPendingExceptions`, `maxLocalFiles`, `localFileMaxAgeHours`, `eventsWindow`, `eventsMaxCount`); delete them from the call. Docs (2.x): https://docs.tracewayapp.com/client/flutter

## Setup

```bash
flutter pub add 'traceway:{"git":{"url":"https://github.com/tracewayapp/traceway-flutter.git","ref":"v2-session-capture"}}'  # 2.x prerelease, new integrations
flutter pub add traceway                  # 1.x, the stable release (older servers)
```

Wrap the app in `Traceway.run()`:

```dart
import 'package:flutter/material.dart';
import 'package:traceway/traceway.dart';

void main() {
  Traceway.run(
    connectionString: 'your-token@https://traceway.example.com/api/report',
    options: TracewayOptions(
      screenCapture: true,
      version: '1.0.0',
    ),
    child: MyApp(),
  );
}
```

This automatically captures Flutter framework errors (`FlutterError.onError`), native platform channel errors, and uncaught async errors (via Dart's `Zone` mechanism).

Wire the navigator observer so navigation transitions are recorded:

```dart
MaterialApp(
  navigatorObservers: [Traceway.navigatorObserver],
  home: const HomePage(),
);
```

## Manual capture

```dart
try {
  await riskyOperation();
} catch (e, st) {
  TracewayClient.instance?.captureException(e, st);
}

TracewayClient.instance?.captureMessage('User completed onboarding');
```

## 2.x options (`TracewayOptions`)

The `capture*` options, `screenCapture` and `sessions` are ceilings: remote rules can lower collection, never raise it.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `sampleRate` | `double` | `1.0` | Share of exceptions and messages reported |
| `screenCapture` | `bool` | `false` | Record the screen (Android, iOS, macOS; needs a video encoder) |
| `debug` | `bool` | `false` | Print debug info to the console |
| `version` | `String` | `''` | Sent as `service.version` (`unknown` when empty) |
| `serviceName` | `String?` | `flutter-app` | Sent as `service.name` |
| `environment` | `String?` | none | Sent as `deployment.environment.name` |
| `attributes` | `Map<String, String>` | `{}` | On every record; also used for remote-config targeting (`user.id`) |
| `capturePixelRatio` | `double` | `0.75` | Screenshot resolution scale |
| `maxBufferFrames` | `int` | `150` | Frames in the recording ring buffer (~10 s at 15 fps) |
| `fps` | `int` | `15` | Frames per second for screen capture (1 to 59) |
| `persistToDisk` | `bool` | `true` | Keep SDK state and undelivered telemetry on disk |
| `captureLogs` | `bool` | `true` | `print` / `debugPrint` as log records |
| `captureNetwork` | `bool` | `true` | `HttpOverrides.global`: every dart:io request as a client span |
| `captureNavigation` | `bool` | `true` | Transitions from `Traceway.navigatorObserver` |
| `captureUserActions` | `bool` | `true` | Taps with a widget selector; feeds click/hover trigger steps |
| `captureMetrics` | `bool` | `true` | `app.*` performance metrics (Flutter Performance dashboard) |
| `metricDimensions` | `Map<String, String>` | `{}` | Up to two `custom.<key>` metric dimensions |
| `recordAllSessions` | `bool` | `false` | Stream breadcrumbs and recording instead of buffering them |
| `collection` | `Map<String, Object?>?` | none | Local collection settings, the same document as a Data Collection rule |
| `sessions` | `bool` | `true` | Send session records; the session id exists either way, so captures and rules that stream replay still work |
| `baselineSampleRate` | `double` | `0` | Share of sessions whose breadcrumbs stream continuously |
| `remoteConfig` | `bool` | `true` | Poll OpAMP for collection settings |
| `exportDelayMs` | `int` | `1000` | Batching delay for logs and spans |
| `propagateTraceHeaderUrls` | `List<Pattern>` | `[]` | Requests to these URLs carry a W3C `traceparent` naming their client span (`String`: URL prefix; `RegExp`: match) |
| `telemetry` | `TracewayTelemetry?` | none | Send logs, spans and metrics to another OTLP/HTTP receiver (below) |

In 2.x, breadcrumbs (console, network, navigation, taps, custom) are buffered on the device (2 MiB / 10 minutes) and uploaded when an exception or a trigger asks for them; errors and messages stream. Keep these production defaults; do not set `recordAllSessions` to prove setup works. To link app requests to backend traces, set `propagateTraceHeaderUrls` to the app's own API origins only (for example `['https://api.example.com/']`); it is empty by default because an unexpected header can break a strict CORS or signature check, and requests to Traceway or the `telemetry` receivers never get it. It reaches requests through `TracewayHttpClient` always and other dart:io requests only with `captureNetwork: true`; a request that already has a `traceparent` (the app's own tracing) keeps it. The backend must extract `traceparent` (OTel HTTP instrumentation does). `HttpOverrides` only sees dart:io clients: wrap a `cupertino_http` / `cronet_http` client in `TracewayHttpClient(inner)`. Wrapping the default dart:io client is also fine; each request is still recorded once. Other 2.x API: `Traceway.track(name, attrs)`, `Traceway.setAttribute`, `Traceway.recordNavigation(path)` for routers without a `NavigatorObserver`, `Traceway.reportFullyDisplayed()`, `Traceway.metric` / `Traceway.count`, `TracewayClient.instance?.flush()`, `refreshConfig()`.

### 2.x: another OpenTelemetry backend, or standalone

```dart
Traceway.run(
  connectionString: '<token>@https://<instance>/api/report',
  options: const TracewayOptions(
    telemetry: TracewayTelemetry(
      url: 'https://collector.example.com',  // /v1/logs, /v1/traces, /v1/metrics appended
      headers: {'x-api-key': '<collector-key>'},
      // logs:, traces:, metrics: TracewayTelemetryEndpoint(url:, headers:) are complete URLs
    ),
  ),
  child: MyApp(),
);
```

With a connection string, ordinary logs (print, navigation, taps, `track`, `recordAction`), request spans and metrics go to the receiver; a signal without a destination stays on Traceway. Exceptions, `captureMessage` issues, session and control records, OpAMP and screen recordings stay on Traceway; both sides carry `session.id`. The Traceway token is never sent to the receiver. Leaving out `connectionString` runs standalone: `telemetry` then needs a base `url` or all three signal endpoints, every record (exceptions as `exception` log records) goes to the receiver, and remote config, recording uploads and session records are off. The receiver must accept OTLP/HTTP protobuf (logs and spans gzip-compressed) and delta sums and exponential histograms. URLs must be HTTP(S) without credentials, query or fragment. An invalid `telemetry` or connection string does not crash the app: `Traceway.run` reports it as a `FlutterError` (library `traceway`, in the console) and runs the app without the SDK, so after setup check the debug console for that error; `TracewayClient.initialize` called directly throws `ArgumentError`. A key in the app binary is readable by its users, so use a write-only ingestion key or a collector that holds the backend credentials.

## 1.x options (`TracewayOptions`)

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `sampleRate` | `double` | `1.0` | Event sampling rate (0.0 to 1.0) |
| `screenCapture` | `bool` | `false` | Records the last ~10 seconds of screen as video |
| `debug` | `bool` | `false` | Prints debug info to the console |
| `version` | `String` | `''` | App version string |
| `debounceMs` | `int` | `1500` | Milliseconds before sending batched events |
| `capturePixelRatio` | `double` | `0.75` | Screenshot resolution scale |
| `maxBufferFrames` | `int` | `150` | Max frames in recording buffer (~10s at 15fps) |
| `fps` | `int` | `15` | Frames per second for screen capture (1 to 59) |
| `retryDelayMs` | `int` | `10000` | Retry delay in ms on failed uploads |
| `maxPendingExceptions` | `int` | `5` | Max exceptions held in memory before oldest is dropped |
| `persistToDisk` | `bool` | `true` | Persist pending exceptions to disk across app restarts |
| `maxLocalFiles` | `int` | `5` | Max exception files stored on disk |
| `localFileMaxAgeHours` | `int` | `12` | Hours after which unsynced local files are deleted |
| `captureLogs` | `bool` | `true` | Mirror every `print` / `debugPrint` into the rolling log buffer |
| `captureNetwork` | `bool` | `true` | Install `HttpOverrides.global` to record every dart:io HTTP call |
| `captureNavigation` | `bool` | `true` | Record transitions reported by `Traceway.navigatorObserver` |
| `eventsWindow` | `Duration` | `10s` | Rolling window kept in the log/action buffers |
| `eventsMaxCount` | `int` | `200` | Hard cap applied independently to logs and actions |

## Platform permissions

- **Android**: add the `INTERNET` permission to `android/app/src/main/AndroidManifest.xml`:
  ```xml
  <uses-permission android:name="android.permission.INTERNET"/>
  ```
- **Android build**: `flutter_quick_video_encoder` 1.7.2 (the screen recorder's encoder) declares `compileSdk` 33, and a current Flutter template (Flutter 3.50, AGP 9.3) fails the release build with "requires libraries and applications that depend on it to compile against version 34 or later ... :flutter_quick_video_encoder is currently compiled against android-33". Add to `android/build.gradle.kts`, above `subprojects { project.evaluationDependsOn(":app") }`:
  ```kotlin
  // The encoder 1.7.2 declares API 33; the AndroidX libraries it builds with need 34.
  project(":flutter_quick_video_encoder") {
      plugins.withId("com.android.library") {
          extensions.getByType<com.android.build.api.variant.LibraryAndroidComponentsExtension>().finalizeDsl {
              it.compileSdk = maxOf(it.compileSdk ?: 0, 34)
          }
      }
  }
  ```
- **macOS**: sandboxed by default; add `com.apple.security.network.client` (value `<true/>`) to BOTH `macos/Runner/DebugProfile.entitlements` and `macos/Runner/Release.entitlements`.
- **iOS**: nothing needed.

## Screen recording

With `screenCapture: true`, the SDK wraps the app in a `RepaintBoundary`, captures frames at ~15 fps, and on exception encodes the last ~10 seconds to MP4 and sends it with the report (1.x) or uploads it to `/api/recordings` linked to the exception (2.x; a capture that asks for replay also streams the session as consecutive MP4 segments). Touch positions are rendered as blue circles on the captured frames only; the live app is unaffected.

Mask sensitive content with `TracewayPrivacyMask` (applies to recorded frames only):

```dart
TracewayPrivacyMask(
  child: Text('4242 4242 4242 4242'),
)

TracewayPrivacyMask(
  mode: TracewayMaskMode.blur(ratio: 0.5),
  child: CreditCardWidget(),
)

TracewayPrivacyMask(
  mode: TracewayMaskMode.blank(color: Color(0xFF000000)),
  child: SensitiveDataWidget(),
)
```

## Logs and actions (1.x)

In 2.x these are OpenTelemetry records in the ring buffer described above, not limited to 10 seconds, and taps are recorded too. In 1.x, every captured exception ships with the last ~10 seconds of context from two rolling buffers (200 entries each by default):

- **Logs**: every `print` / `debugPrint` line, mirrored via a Zone print override. `dart:developer.log` is NOT captured.
- **Network actions**: every dart:io HTTP request (method, URL, status, duration, byte counts) via `HttpOverrides.global`, which catches `package:http`, Dio, Firebase, and anything else on the dart:io HttpClient.
- **Navigation actions**: push / pop / replace / remove, from `Traceway.navigatorObserver`.
- **Custom actions**:
  ```dart
  Traceway.recordAction(
    category: 'cart',
    name: 'add_item',
    data: {'sku': 'SKU-123', 'qty': 2},
  );
  ```

## Symbol upload (obfuscated builds)

A plain `flutter build --release` keeps enough symbol information that crash stack traces are already readable (function names plus `file:line`), and the SDK reports them as-is. You only need symbol upload when the build is hardened with `--obfuscate` and/or `--split-debug-info=<dir>`: those flags strip and rename Dart symbols into per-architecture `.symbols` files, so production traces then arrive as bare instruction offsets and stay unreadable until Traceway resolves them server-side from the build's `.symbols` files (the same way it handles JavaScript source maps).

**First, check whether the build is obfuscated.** Look at how release artifacts are produced (the `flutter build` invocations in CI under `.github/`, in `fastlane/`, in a `Makefile`, or in shell scripts) and ask the user how they build for release. If neither `--obfuscate` nor `--split-debug-info` is passed, stack traces are already readable and symbol upload is unnecessary; skip the rest of this section. If obfuscation is (or will be) used, set up the upload.

**Then use the project's upload token.** Symbol uploads authenticate with the dedicated upload token (Connection page > Symbol Upload, the same token shown under Source Maps), NOT the project token from the connection string. It is a CI secret, never committed. `readonly` members cannot generate one.

The `traceway` package ships an uploader (`dart run traceway:upload_symbols`) that finds the `.symbols` files, derives each one's architecture and debug ID, and posts them. Configure it once, then run it on every release.

**One-time setup.** Add a `traceway:` block to the app's `pubspec.yaml`:

```yaml
traceway:
  url: https://<instance>   # omit on Traceway Cloud
  # upload_token: ...        # prefer the env var below, especially in CI
```

The upload token resolves from `TRACEWAY_UPLOAD_TOKEN` (or `--token`) before the `upload_token` key, so keep it out of `pubspec.yaml` in CI.

**Per release.** Build with obfuscation and a known symbols directory, then run the uploader:

```bash
flutter build apk --release --obfuscate --split-debug-info=build/symbols

TRACEWAY_UPLOAD_TOKEN=<upload-token> dart run traceway:upload_symbols
```

The uploader auto-discovers `build/symbols` and uploads every architecture in one run, so no flags are required. Each value resolves from a CLI flag first (`--token`, `--url`, `--symbols-dir`), then an env var (`TRACEWAY_UPLOAD_TOKEN`, `TRACEWAY_URL`), then the `pubspec.yaml` block. Pass `--dry-run` to preview, but note that it still requires a resolved token and URL: without them it exits with `No upload token. Set TRACEWAY_UPLOAD_TOKEN, pass --token, or add traceway.upload_token to pubspec.yaml`. Symbols are unique to each build, so upload on every release; a crash only resolves against the exact build it came from.

The build command above produces an Android APK; other targets (`appbundle`, `ios`, `macos`) emit their own `.symbols` files into the same directory, all picked up by a single uploader run. On Android the debug ID is read straight from each file's build-id note. iOS and macOS `.symbols` carry no such note, so the uploader reads the Mach-O UUID from the built `.app` (auto-discovered under `build/`, or point at it with `--app build/macos/Build/Products/Release/YourApp.app`); run the uploader on the same Mac that produced the build.

The uploader exists because that UUID has to be read out of the compiled app, which is easiest on the build machine. It is not the only path. If a pipeline cannot run the Dart uploader, read the UUID yourself and pass it as a `debug_id` field:

```bash
dwarfdump --uuid build/macos/Build/Products/Release/YourApp.app/Contents/Frameworks/App.framework/App

curl -fsS -H "Authorization: Bearer <upload-token>" \
  -F "files=@build/symbols/app.darwin-arm64.symbols" \
  -F "debug_id=<uuid-from-dwarfdump>" \
  https://<instance>/api/symbols/upload
```

Case and dashes are normalized server-side, so paste the UUID exactly as `dwarfdump` prints it. The architecture is read from the `.symbols` filename; pass `-F "arch=arm64"` too if the file has been renamed.

**Known issue, verified 2026-08-17.** This curl body is byte-identical to what the Dart uploader sends, and on macOS both currently accept the upload (`2/2 uploaded`, `debug_id` matching the runtime build id) while obfuscated release traces still come back unresolved. The curl path inherits the bug, so it is not a workaround for it. Until that is fixed, keep a copy of the `.symbols` files with each release so traces can be resolved offline.

Wire the upload into the release pipeline right after the build step (CI, fastlane, or a build script), with the token from a secret:

```yaml
- name: Upload Flutter symbols
  run: dart run traceway:upload_symbols
  env:
    TRACEWAY_URL: ${{ secrets.TRACEWAY_URL }}
    TRACEWAY_UPLOAD_TOKEN: ${{ secrets.TRACEWAY_UPLOAD_TOKEN }}
```

Self-hosted instances must have blob storage (S3 or a persistent volume) configured, or uploaded symbols disappear when the container is recreated.

## Flutter web

The Flutter SDK does not support web (no error tracking, no screen recording there). For Flutter web apps, use the JS SDK in `web/index.html` instead, with the connection string of a separate browser project (a Flutter project is a mobile project):

```html
<script src="https://cdn.jsdelivr.net/npm/@tracewayapp/frontend@2/dist/traceway.iife.global.js"></script>
<script>
  Traceway.init("your-token@https://traceway.example.com/api/report");
</script>
```

The Dart SDK, `TracewayHttpClient` included, does not run on web. The JS SDK records the page's `fetch` and `XMLHttpRequest` calls, which is what Dart's HTTP clients use in a web build, so no Dart-side network setup is needed there.

## Verify

Add a button that throws (`throw StateError('Test error from Traceway')`), tap it, and check the Issues page in the dashboard. With `screenCapture: true`, the recording appears alongside the stack trace. For an obfuscated release build, confirm the stack trace shows real frames rather than obfuscated offsets once the matching `.symbols` files have been uploaded.
