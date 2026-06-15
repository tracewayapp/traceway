# Flutter SDK Reference

The Traceway Flutter SDK is the [`traceway`](https://pub.dev/packages/traceway) package on pub.dev. It supports iOS, Android, and macOS. It does NOT support Flutter web (see the bottom of this file).

## Setup

```bash
flutter pub add traceway
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

## Options (`TracewayOptions`)

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
- **macOS**: sandboxed by default; add `com.apple.security.network.client` (value `<true/>`) to BOTH `macos/Runner/DebugProfile.entitlements` and `macos/Runner/Release.entitlements`.
- **iOS**: nothing needed.

## Screen recording

With `screenCapture: true`, the SDK wraps the app in a `RepaintBoundary`, captures frames at ~15 fps, and on exception encodes the last ~10 seconds to MP4 and sends it with the report. Touch positions are rendered as blue circles on the captured frames only; the live app is unaffected.

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

## Logs and actions

Every captured exception ships with the last ~10 seconds of context from two rolling buffers (200 entries each by default):

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

**Then ask for the upload token.** Symbol uploads authenticate with the dedicated upload token (Connection page > Symbol Upload, the same token shown under Source Maps), NOT the project token from the connection string. Get it from Step 0; it is a CI secret, never committed. `readonly` members cannot generate one.

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

The uploader auto-discovers `build/symbols` and uploads every architecture in one run, so no flags are required. Each value resolves from a CLI flag first (`--token`, `--url`, `--symbols-dir`), then an env var (`TRACEWAY_UPLOAD_TOKEN`, `TRACEWAY_URL`), then the `pubspec.yaml` block. Pass `--dry-run` to preview. Symbols are unique to each build, so upload on every release; a crash only resolves against the exact build it came from.

The build command above produces an Android APK; other targets (`appbundle`, `ios`, `macos`) emit their own `.symbols` files into the same directory, all picked up by a single uploader run. On Android the debug ID is read straight from each file's build-id note. iOS and macOS `.symbols` carry no such note, so the uploader reads the Mach-O UUID from the built `.app` (auto-discovered under `build/`, or point at it with `--app build/macos/Build/Products/Release/YourApp.app`); run the uploader on the same Mac that produced the build. There is no hand-written upload path for Apple builds because that UUID can only be read from the compiled app.

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

The Flutter SDK does not support web (no error tracking, no screen recording there). For Flutter web apps, use the JS SDK in `web/index.html` instead:

```html
<script src="https://cdn.jsdelivr.net/npm/@tracewayapp/frontend@1/dist/traceway.iife.global.js"></script>
<script>
  Traceway.init("your-token@https://traceway.example.com/api/report");
</script>
```

For network capture in Dart code on web (where `HttpOverrides.global` does not run), `TracewayHttpClient` is a drop-in `http.Client`, usable directly or passed to libraries that accept a custom client (Dio, Chopper):

```dart
final client = TracewayHttpClient();
final res = await client.get(Uri.parse('https://api.example.com/users'));
```

## Verify

Add a button that throws (`throw StateError('Test error from Traceway')`), tap it, and check the Issues page in the dashboard. With `screenCapture: true`, the recording appears alongside the stack trace. For an obfuscated release build, confirm the stack trace shows real frames rather than obfuscated offsets once the matching `.symbols` files have been uploaded.
