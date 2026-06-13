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

Add a button that throws (`throw StateError('Test error from Traceway')`), tap it, and check the Issues page in the dashboard. With `screenCapture: true`, the recording appears alongside the stack trace.
