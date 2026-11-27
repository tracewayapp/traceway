# iOS / Apple SDK Reference

iOS and other Apple apps have two integration paths. Pick by what the app is written in:

| App | Path |
|---|---|
| **Native Swift** (SwiftUI or UIKit) | The Traceway iOS SDK (below). Recommended. Crashes and errors with their breadcrumbs, sessions, wireframe session replay, and remote collection settings. |
| **Anything else** (Objective-C only, a cross-platform stack with no Traceway mobile SDK, or a team already standardized on OpenTelemetry) | An OpenTelemetry distribution such as Honeycomb's, with its OTLP exporter repointed at Traceway. See "Non-Swift apps" at the bottom. |

## Swift apps: the Traceway iOS SDK

The SDK lives at `github.com/tracewayapp/traceway-ios` and is distributed with Swift Package Manager. Docs (2.x): https://docs.tracewayapp.com/client/ios

### Choose the version

There are two generations with the same connection string. 0.1.x starts with `Traceway.start`; 2.x starts with `Traceway.initialize(connectionString:options:)` (Swift reserves `Traceway.init` for initializers) and keeps `start` as a deprecated alias:

- **0.1.x** (the stable release): errors and crashes only, sent to `/api/report`, no third-party dependencies.
- **2.x** (the 2.x prerelease, `2.0.0-beta.N`): rebuilt on the official OpenTelemetry Swift SDK. Logs, spans and `app.*` performance metrics go as OTLP/HTTP to `/api/otel/v1/logs`, `/api/otel/v1/traces` and `/api/otel/v1/metrics`, wireframe replays to `/api/recordings`, and the SDK polls `/api/otel/v1/opamp` for the iOS project's **Data Collection** rules. It needs Xcode 26.4 (Swift 6.3) or later and a Traceway server that accepts iOS projects on those endpoints; there is no fallback to `/api/report`.

Read the resolved version from `Package.resolved`. Keep an existing app on its major, and move a 0.1.x app to 2.x only when the user asks for it or for something only 2.x has (breadcrumbs, sessions, session replay, remote config, captures and recording triggers, network spans and trace propagation, sending telemetry to another OpenTelemetry backend). A new integration installs the 2.x prerelease, the line the project's Connection page shows, when the Traceway server accepts iOS projects on OTLP and OpAMP and the team builds with Xcode 26.4 or later; otherwise it installs 0.1.x. Moving to 2.x removes the options `debounceMs`, `retryDelayMs`, `maxPendingExceptions`, `maxLocalFiles` and `localFileMaxAgeHours`; delete them from the call. Unsent 0.1.x reports younger than 12 hours are sent once on the first 2.0 start.

### Install (Swift Package Manager)

In Xcode: **File > Add Package Dependencies...**, enter `https://github.com/tracewayapp/traceway-ios.git` and ask for `2.0.0-beta.1` or later, or add it to `Package.swift` (SwiftPM only offers a prerelease to a requirement that names one):

```swift
.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "2.0.0-beta.1"),   // 2.x
.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "0.1.0"),          // 0.1.x
```

then add the `Traceway` product to the app target: `.product(name: "Traceway", package: "traceway-ios")` (the package identity and the product name differ).

### Initialize

Call `Traceway.initialize` (on 0.1.x, `Traceway.start` with the same arguments) as early as possible, in the `App` initializer (SwiftUI) or `application(_:didFinishLaunchingWithOptions:)` (UIKit). The connection string is `<project-token>@https://<instance>`; the 1.x form ending in `/api/report` works too.

**SwiftUI:**

```swift
import SwiftUI
import Traceway

@main
struct MyApp: App {
    init() {
        Traceway.initialize(
            connectionString: "<project-token>@https://<instance>",
            options: TracewayOptions(
                version: "1.0.0",
                screenCapture: true   // 2.x: wireframe session replay
            )
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
        Traceway.initialize(
            connectionString: "<project-token>@https://<instance>",
            options: TracewayOptions(version: "1.0.0", screenCapture: true)
        )
        return true
    }
}
```

Wire the token through a build setting or `Info.plist` value, not a committed literal. `Traceway.initialize` returns `nil` and no-ops on a malformed connection string, so a bad value fails quietly rather than crashing the host app.

### What gets captured automatically (2.x)

After `initialize`:

- **Uncaught `NSException`s** (Objective-C / UIKit), written to disk and sent before the app ends.
- **Fatal signals**: Swift runtime traps such as force-unwrapping `nil`, array out-of-bounds, `fatalError()`, integer overflow (surface as `SIGTRAP`, `SIGILL`, `SIGABRT`, `SIGSEGV`, ...). Hard crashes are persisted to disk and uploaded on the **next launch**, with the session they happened in.
- **Breadcrumbs**, kept in a ring buffer on the device and uploaded with an exception: URLSession requests (OpenTelemetry client spans), screens (view controllers that appear, `.tracewayScreen("/path")` on SwiftUI views, `Traceway.recordNavigation`), taps with the CSS-like selector of the view (`UIButton#pay`), and `Traceway.log` lines. `print` and `NSLog` are not captured.
- **Sessions** and, with `screenCapture: true`, a **wireframe replay** of the last 30 seconds before an exception: the view hierarchy as HTML, never a screenshot. Pictures of image views are included only when the project's `replay.images` setting (default `wifi`: not cellular, a hotspot or Low Data Mode) and the `replayImages` option allow it, otherwise images are placeholders; typed text is masked, and `view.tracewayMask = .blur` / `.blank()`, the SwiftUI modifier `.tracewayMask(_:)`, or `accessibilityIdentifier = "traceway-mask"` / `"traceway-blank"` on a UIKit view hides views (`tracewayPrivacyMask` is the deprecated 2.0.0-beta.1 name).

### Manual capture

```swift
do {
    try somethingThrowing()
} catch {
    Traceway.captureException(error)   // Swift Error or NSError
}

Traceway.captureMessage("User completed onboarding")
Traceway.log("Loaded 12 items")                         // 2.x breadcrumb, level .info
Traceway.log("Payment retry", level: .warn)
Traceway.track("checkout_failed", attributes: ["reason": "card_declined"])
Traceway.setAttribute("user.id", user.id)               // 2.x: on every later record, targetable from the dashboard
Traceway.flush(timeout: 5)                              // send what is ready now; seconds
```

Name the views users tap with `accessibilityIdentifier` (`#pay` in a recording trigger's click step) and report SwiftUI screens with `.tracewayScreen("/checkout")`; selectors and screen paths are in the docs' "Recording triggers" section.

### Options (`TracewayOptions`, 2.x)

| Option | Default | Description |
|---|---|---|
| `sampleRate` | `1.0` | Fraction of exceptions and messages to report (0.0 to 1.0). |
| `debug` | `false` | Log SDK activity via `NSLog`. |
| `version` | bundle version | App version, sent as `service.version` (enables release comparison). |
| `serviceName` / `environment` | bundle id / none | `service.name` and `deployment.environment.name`. |
| `screenCapture` | `false` | Wireframe session replay. |
| `replayImages` | `.always` | Ceiling for pictures in the replay (`.off`, `.wifi`, `.always`); the `replay.images` setting (default `wifi`) can only lower it. |
| `persistToDisk` | `true` | Keep state, the ring buffer and undelivered telemetry on disk (required for hard-crash capture). |
| `captureLogs` / `captureNetwork` / `captureNavigation` / `captureUserActions` | `true` | Ceilings for `Traceway.log`, URLSession requests, screens and taps; no remote rule can turn one back on. |
| `remoteConfig` | `true` | Poll OpAMP for the Data Collection rules. |
| `collection` | none | Local collection settings, the same document as a rule. |
| `propagateTraceHeaderUrls` | `[]` | URL prefixes whose requests carry W3C `traceparent`. |
| `telemetry` | none | Send ordinary logs and spans to another OTLP/HTTP receiver too; without a connection string, only there. |
| `ignoreErrors` / `beforeCapture` | network noise / none | Drop exceptions whose `Type: message` line matches a string or `NSRegularExpression`; a callback returning `false` drops one. |

0.1.x has `sampleRate`, `debug`, `version`, `persistToDisk` and its batching options `debounceMs`, `retryDelayMs`, `maxPendingExceptions`, `maxLocalFiles`, `localFileMaxAgeHours`.

### Testing crash capture (debugger caveat)

Hard crashes are intercepted with POSIX signal handlers. When the **Xcode debugger is attached**, lldb intercepts `SIGSEGV` / `SIGTRAP` first, so the SDK's handler never runs and the crash is not recorded. To test the hard-crash path: install once via Xcode, **stop the debugger**, launch the app from the home screen, trigger the crash, then relaunch and watch the report upload from disk. Managed captures (`Traceway.captureException(...)`) work with the debugger attached.

### Symbolication: upload dSYMs

Release crash reports arrive as bare instruction addresses against each loaded binary image. They stay unreadable until Traceway resolves them server-side against the build's **dSYM** debug symbols (the Apple equivalent of JavaScript source maps), for errors and crashes alike and with both SDK generations. Upload the dSYMs on every release build; a crash only resolves against the exact build it came from.

**Token.** Symbol uploads authenticate with the dedicated **upload token** (Connection page > Source Maps / Symbol Upload), NOT the project token from the connection string. It is a CI secret, never committed. `readonly` members cannot generate one. The upload endpoint is `https://<instance>/api/symbols/upload`.

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

1. In a Debug build, tap a button that calls `Traceway.captureException(SomeError())`, then `Traceway.flush(timeout: 5)`, and confirm it appears on the Issues page within a few seconds. With 2.x, open the issue: its breadcrumbs and, with `screenCapture: true`, the replay are there, and the project's **Sessions** page lists the session.
2. For hard crashes: build a Release configuration, run it **without the debugger** (home-screen launch), trigger a crash (force-unwrap `nil`, `fatalError()`, ...), relaunch the app, and confirm the crash shows up on Issues.
3. After uploading dSYMs for that Release build, confirm the crash stack trace shows real symbol names and `file:line`, not bare addresses.

## Non-Swift apps: OpenTelemetry into Traceway

For an iOS/Apple app that is not a Swift app the SDK targets (Objective-C only, a cross-platform stack with no Traceway mobile SDK, or a codebase already standardized on OpenTelemetry), there is no native Traceway SDK. Use an OpenTelemetry distribution such as **Honeycomb's** and repoint its OTLP exporter at Traceway. This is the same OTLP/HTTP path the backend uses (see "Backend OTel Setup" in SKILL.md), so all the rules in `data-model.md` apply.

A Honeycomb-style distro is configured with an API key plus an endpoint. Override both so the data lands in Traceway instead of Honeycomb:

- **Endpoint**: `https://<instance>/api/otel` (the SDK appends `/v1/traces`, `/v1/metrics`, `/v1/logs`).
- **Auth header**: `Authorization: Bearer <project-token>`. Traceway authenticates on this Bearer header, not on Honeycomb's `x-honeycomb-team`, so set the `Authorization` header explicitly (Honeycomb's own API-key header is ignored by Traceway and harmless).
- **`service.name`** / **`service.version`**: set them; they become the Server Name and enable release comparison.

Both `build()` and `Honeycomb.configure(options:)` are `throws`, so the whole thing has to sit inside a `do` / `catch` (or a throwing function). Call it once, as early as possible:

```swift
import Honeycomb

do {
    let options = try HoneycombOptions.Builder()
        .setServiceName("my-ios-app")
        .setServiceVersion("1.0.0")
        .setAPIEndpoint("https://<instance>/api/otel")
        .setHeaders(["Authorization": "Bearer <project-token>"])
        .build()
    try Honeycomb.configure(options: options)
} catch {
    NSLog("Honeycomb configure failed: \(error)")
}
```

The method is `setAPIEndpoint`, with a capital `API`, matching `setAPIKey`. Swift is case-sensitive, so `setApiEndpoint` does not compile. Do not drop the line if it fails: without it the exporter falls back to Honeycomb's own default endpoint and the telemetry never reaches Traceway.

No Honeycomb API key is needed. `build()` only demands one when the resolved endpoint is a Honeycomb host, and `https://<instance>/api/otel` is not.

(Field and builder names vary by distro version; the three things that matter are endpoint, the `Authorization: Bearer` header, and service name. A plain OpenTelemetry-Swift setup with an `OTLPHTTPExporter` pointed at the same endpoint and header is equivalent.)

Constraints are the backend OTLP rules: OTLP/HTTP only (protobuf or JSON), OTLP/gRPC is NOT supported, gzip is fine, max request body 10 MB.

How the data shows up follows the standard classification in `data-model.md`: recorded `exception` events become **Issues**, and instrumented network calls and screen transitions become spans. An iOS project treats its telemetry as an end user's device: issues keep the resource's device and user attributes, and spans stay spans instead of becoming **Endpoints**. Verify on the Issues page first: throw or record a test error and confirm it appears.
