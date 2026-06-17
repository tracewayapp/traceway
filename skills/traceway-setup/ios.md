# iOS / Apple SDK Reference

iOS and other Apple apps have two integration paths. Pick by what the app is written in:

| App | Path |
|---|---|
| **Native Swift** (SwiftUI or UIKit) | The Traceway iOS SDK (below). Recommended. Native crash and error capture, reports to `/api/report` like the other Traceway mobile SDKs. |
| **Anything else** (Objective-C only, a cross-platform stack with no Traceway mobile SDK, or a team already standardized on OpenTelemetry) | An OpenTelemetry distribution such as Honeycomb's, with its OTLP exporter repointed at Traceway. See "Non-Swift apps" at the bottom. |

The Traceway iOS SDK reports **errors and crashes only**. There is no session or video replay (unlike Flutter). It speaks the same `/api/report` wire format as the Android, Flutter, and JS SDKs, so the backend ingests it with no server changes.

## Swift apps: the Traceway iOS SDK

The SDK lives at `github.com/tracewayapp/traceway-ios`. Pure Swift, zero third-party dependencies, distributed via Swift Package Manager.

### Requirements

- iOS 13.0+
- Swift 5.9+ / Xcode 15+

### Install (Swift Package Manager)

In Xcode: **File > Add Package Dependencies...** and enter `https://github.com/tracewayapp/traceway-ios.git`, or add it to `Package.swift`:

```swift
.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "0.1.0"),
```

then add `"Traceway"` to the app target's dependencies.

### Initialize

Call `Traceway.start` as early as possible, in the `App` initializer (SwiftUI) or `application(_:didFinishLaunchingWithOptions:)` (UIKit). The connection string format is `<project-token>@https://<instance>/api/report`, identical to the other mobile SDKs.

**SwiftUI:**

```swift
import SwiftUI
import Traceway

@main
struct MyApp: App {
    init() {
        Traceway.start(
            connectionString: "<project-token>@https://<instance>/api/report",
            options: TracewayOptions(version: "1.0.0")
        )
    }

    var body: some Scene {
        WindowGroup { ContentView() }
    }
}
```

**UIKit:**

```swift
import UIKit
import Traceway

@main
final class AppDelegate: UIResponder, UIApplicationDelegate {
    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        Traceway.start(
            connectionString: "<project-token>@https://<instance>/api/report",
            options: TracewayOptions(version: "1.0.0")
        )
        return true
    }
}
```

Wire the token through a build setting or `Info.plist` value, not a committed literal. `Traceway.start` returns `nil` and no-ops on a malformed connection string, so a bad value fails quietly rather than crashing the host app.

### What gets captured automatically

After `start`:

- **Uncaught `NSException`s** (Objective-C / UIKit).
- **Fatal signals**: Swift runtime traps such as force-unwrapping `nil`, array out-of-bounds, `fatalError()`, integer overflow (surface as `SIGTRAP`, `SIGILL`, `SIGABRT`, `SIGSEGV`, ...). Hard crashes are persisted to disk and uploaded on the **next launch**, not at crash time.

### Manual capture

```swift
do {
    try somethingThrowing()
} catch {
    Traceway.capture(error)            // Swift Error
}

Traceway.capture(error: nsError)       // NSError
Traceway.capture(message: "User completed onboarding")
```

### Forcing a flush

Managed (in-process) reports are batched and sent after `debounceMs`. To send immediately, for example before a known exit:

```swift
Traceway.flush(timeout: 5) // seconds; nil = wait indefinitely
```

### Options (`TracewayOptions`)

| Option | Default | Description |
|---|---|---|
| `sampleRate` | `1.0` | Fraction of exceptions to keep (0.0 to 1.0). |
| `debug` | `false` | Log SDK activity via `NSLog`. |
| `version` | `""` | App version, sent as `appVersion` (enables release comparison). |
| `debounceMs` | `1500` | Delay before batching and uploading. |
| `retryDelayMs` | `10000` | Delay before retrying a failed upload. |
| `maxPendingExceptions` | `5` | In-memory cap; oldest is dropped when exceeded. |
| `persistToDisk` | `true` | Persist pending reports so they survive restarts (required for hard-crash capture). |
| `maxLocalFiles` | `5` | Max persisted report files. |
| `localFileMaxAgeHours` | `12` | Delete unsynced files older than this. |

### Testing crash capture (debugger caveat)

Hard crashes are intercepted with POSIX signal handlers. When the **Xcode debugger is attached**, lldb intercepts `SIGSEGV` / `SIGTRAP` first, so the SDK's handler never runs and the crash is not recorded. To test the hard-crash path: install once via Xcode, **stop the debugger**, launch the app from the home screen, trigger the crash, then relaunch and watch the report upload from disk. Managed captures (`Traceway.capture(...)`) work with the debugger attached.

### Symbolication: upload dSYMs

Release crash reports arrive as bare instruction addresses against each loaded binary image. They stay unreadable until Traceway resolves them server-side against the build's **dSYM** debug symbols (the Apple equivalent of JavaScript source maps). Upload the dSYMs on every release build; a crash only resolves against the exact build it came from.

**Token.** Symbol uploads authenticate with the dedicated **upload token** (Connection page > Source Maps / Symbol Upload), NOT the project token from the connection string. Get it from Step 0; it is a CI secret, never committed. `readonly` members cannot generate one. The upload endpoint is `https://<instance>/api/symbols/upload`.

**Produce dSYMs.** The build must emit dSYMs: set `DEBUG_INFORMATION_FORMAT = dwarf-with-dsym` for the Release configuration (this is already the default for Archive builds). Without it there are no dSYMs to upload.

**Upload script.** The SDK repo ships a ready-made uploader at `Scripts/upload_symbols.sh`. It finds this build's dSYM DWARF binaries (`*.dSYM/Contents/Resources/DWARF/*`) and POSTs each one as multipart `files=@...` to `/api/symbols/upload`. It derives the upload URL from a base URL or a `/api/report` URL, and no-ops safely (exit 0) on non-Release builds or when the token is unset, so it never breaks a Debug build. Copy it into the app repo, then add a **Run Script** build phase (Target > Build Phases > +) that runs it after the build:

```bash
"${SRCROOT}/Scripts/upload_symbols.sh"
```

Set the token and instance for the phase via the environment (CI secret in CI, a local untracked value otherwise):

```bash
TRACEWAY_UPLOAD_TOKEN=<upload-token>
TRACEWAY_URL=https://<instance>        # base URL; the script appends /api/symbols/upload
```

To upload by hand (for example from CI after an `xcodebuild archive`), point it at the dSYM directory:

```bash
TRACEWAY_UPLOAD_TOKEN=<upload-token> TRACEWAY_URL=https://<instance> \
  ./Scripts/upload_symbols.sh --dsym-dir <path-to>.xcarchive/dSYMs --dry-run
```

`--dry-run` lists what would be sent without uploading. Inside an Xcode build phase, `DWARF_DSYM_FOLDER_PATH` is set automatically and no `--dsym-dir` is needed. If the SDK script is not vendored, the equivalent is one `find` piped to `curl`:

```bash
find "$DWARF_DSYM_FOLDER_PATH" -type f -path '*.dSYM/Contents/Resources/DWARF/*' \
  -exec curl -fsS -H "Authorization: Bearer $TRACEWAY_UPLOAD_TOKEN" \
    -F "files=@{}" "$TRACEWAY_URL/api/symbols/upload" \;
```

Self-hosted instances must have blob storage (S3 or a persistent volume) configured, or uploaded symbols disappear when the container is recreated.

### Verify

1. In a Debug build, tap a button that calls `Traceway.capture(SomeError())`, then `Traceway.flush(timeout: 5)`, and confirm it appears on the Issues page within a few seconds.
2. For hard crashes: build a Release configuration, run it **without the debugger** (home-screen launch), trigger a crash (force-unwrap `nil`, `fatalError()`, ...), relaunch the app, and confirm the crash shows up on Issues.
3. After uploading dSYMs for that Release build, confirm the crash stack trace shows real symbol names and `file:line`, not bare addresses.

## Non-Swift apps: OpenTelemetry into Traceway

For an iOS/Apple app that is not a Swift app the SDK targets (Objective-C only, a cross-platform stack with no Traceway mobile SDK, or a codebase already standardized on OpenTelemetry), there is no native Traceway SDK. Use an OpenTelemetry distribution such as **Honeycomb's** and repoint its OTLP exporter at Traceway. This is the same OTLP/HTTP path the backend uses (Step 2), so all the rules in `data-model.md` apply.

A Honeycomb-style distro is configured with an API key plus an endpoint. Override both so the data lands in Traceway instead of Honeycomb:

- **Endpoint**: `https://<instance>/api/otel` (the SDK appends `/v1/traces`, `/v1/metrics`, `/v1/logs`).
- **Auth header**: `Authorization: Bearer <project-token>`. Traceway authenticates on this Bearer header, not on Honeycomb's `x-honeycomb-team`, so set the `Authorization` header explicitly (Honeycomb's own API-key header is ignored by Traceway and harmless).
- **`service.name`** / **`service.version`**: set them; they become the Server Name and enable release comparison.

```swift
import Honeycomb

let options = try HoneycombOptions.Builder()
    .setServiceName("my-ios-app")
    .setServiceVersion("1.0.0")
    .setApiEndpoint("https://<instance>/api/otel")
    .setHeaders(["Authorization": "Bearer <project-token>"])
    .build()
Honeycomb.configure(options: options)
```

(Field and builder names vary by distro version; the three things that matter are endpoint, the `Authorization: Bearer` header, and service name. A plain OpenTelemetry-Swift setup with an `OTLPHTTPExporter` pointed at the same endpoint and header is equivalent.)

Constraints are the backend OTLP rules: OTLP/HTTP only (protobuf or JSON), OTLP/gRPC is NOT supported, gzip is fine, max request body 10 MB.

How the data shows up follows the standard classification in `data-model.md`: recorded `exception` events become **Issues**, instrumented network calls and screen transitions become spans, and any span carrying HTTP attributes that looks like a root request becomes an **Endpoint**. Verify on the Issues page first: throw or record a test error and confirm it appears.
