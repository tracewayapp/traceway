# iOS SDK Reference

Swift apps (SwiftUI or UIKit) use the Traceway iOS SDK, `github.com/tracewayapp/traceway-ios`, distributed with Swift Package Manager and built on the official OpenTelemetry Swift SDK. It reports errors and crashes with their breadcrumbs, app hangs, log lines, sessions, a wireframe session replay and app performance metrics, and follows the iOS project's **Data Collection** rules.

An Objective-C app uses the SDK through one small Swift bridge file (see "Objective-C apps"). A cross-platform stack with no Traceway SDK, or a team standardized on OpenTelemetry, has no native SDK: go to "Non-Swift apps" at the end. App extensions (widgets, share, notification service) are not supported: instrument the containing app. Docs: https://docs.tracewayapp.com/client/ios

## Choose the version

A new integration installs **2.1.0**. It needs Xcode 26.4 or later (Swift 6.3), iOS 13 or later, and a Traceway server that accepts iOS projects (Traceway Cloud does).

Read the resolved version in `Package.resolved` (pin `traceway-ios`) before changing anything:

- **2.x** (`2.0.0-beta.N` or 2.1.x): move it to `from: "2.1.0"`. The 2.0 start call still works (see "Upgrading from 2.0").
- **0.1.x** (older projects): errors and crashes only, started with `Traceway.start`. Keep it unless the user asks to upgrade or asks for something only 2.x has (breadcrumbs, sessions, replay, Data Collection rules, captures, request spans, app hangs, log capture, performance metrics, another OpenTelemetry destination). Upgrading removes the options `debounceMs`, `retryDelayMs`, `maxPendingExceptions`, `maxLocalFiles` and `localFileMaxAgeHours`; delete them from the call. Unsent 0.1.x reports younger than 12 hours and recorded crashes are sent once on the first 2.x start.

## Install

In Xcode: **File > Add Package Dependencies…**, enter `https://github.com/tracewayapp/traceway-ios.git`, choose the Dependency Rule **Up to Next Major Version** from 2.1.0, and add the **Traceway** library to the app target. In a `Package.swift`:

```swift
dependencies: [
    .package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "2.1.0"),
],
targets: [
    .target(
        name: "MyApp",
        dependencies: [.product(name: "Traceway", package: "traceway-ios")]
    ),
]
```

The package identity is `traceway-ios` and the product is `Traceway`, so the target needs the explicit `.product(name:package:)` form. The package brings in `opentelemetry-swift-core`, `opentelemetry-swift` and `swift-protobuf`.

**An `.xcodeproj` app when you cannot run Xcode** (an agent on Linux, or no GUI): the package reference lives in `project.pbxproj`, and Xcode is the tool that writes it. Pick the first that applies:

1. The repo generates its project: add the package to the generator, not to `project.pbxproj`. XcodeGen (`project.yml`):

   ```yaml
   packages:
     Traceway:
       url: https://github.com/tracewayapp/traceway-ios.git
       from: 2.1.0
   targets:
     MyApp:
       dependencies:
         - package: Traceway
           product: Traceway
   ```

   then `xcodegen generate`. Tuist (`Project.swift`): `packages: [.remote(url: "https://github.com/tracewayapp/traceway-ios.git", requirement: .upToNextMajor(from: "2.1.0"))]` and `.package(product: "Traceway")` in the target's `dependencies`.
2. Otherwise edit `project.pbxproj` with the five entries Xcode would write. Pick three new 24-hex-digit IDs that do not occur in the file yet (below `PKGREF`, `PRODDEP`, `BUILDFILE`), keep the file's tabs, and add:

   ```
   /* Begin PBXBuildFile section */   (the first section in "objects = {"; create it if missing)
        BUILDFILE /* Traceway in Frameworks */ = {isa = PBXBuildFile; productRef = PRODDEP /* Traceway */; };

   /* the app target's PBXFrameworksBuildPhase */
            files = (
                BUILDFILE /* Traceway in Frameworks */,
            );

   /* the app target's PBXNativeTarget, next to its name */
            packageProductDependencies = (
                PRODDEP /* Traceway */,
            );

   /* the PBXProject object, next to mainGroup */
            packageReferences = (
                PKGREF /* XCRemoteSwiftPackageReference "traceway-ios" */,
            );

   /* Begin XCRemoteSwiftPackageReference section */   (before the closing "};" of objects)
        PKGREF /* XCRemoteSwiftPackageReference "traceway-ios" */ = {
            isa = XCRemoteSwiftPackageReference;
            repositoryURL = "https://github.com/tracewayapp/traceway-ios.git";
            requirement = {
                kind = upToNextMajorVersion;
                minimumVersion = 2.1.0;
            };
        };
   /* End XCRemoteSwiftPackageReference section */

   /* Begin XCSwiftPackageProductDependency section */
        PRODDEP /* Traceway */ = {
            isa = XCSwiftPackageProductDependency;
            package = PKGREF /* XCRemoteSwiftPackageReference "traceway-ios" */;
            productName = Traceway;
        };
   /* End XCSwiftPackageProductDependency section */
   ```

   Add to an existing `files`, `packageProductDependencies` or `packageReferences` list rather than writing a second one, and touch only the app target (not test or extension targets). Xcode resolves the package (and writes `Package.resolved`) the first time the user opens the project; tell them so. If the file looks generated or unusual (no `objects = {`, a workspace with several projects), ask the user to add the package in Xcode instead: **File > Add Package Dependencies…**, paste `https://github.com/tracewayapp/traceway-ios.git`, **Dependency Rule** **Up to Next Major Version** with `2.1.0`, **Add Package**, tick **Traceway** for the app target, **Add Package**.

## Initialize

Call `Traceway.initialize` as early as possible: in the `App` initializer (SwiftUI) or `application(_:didFinishLaunchingWithOptions:)` (UIKit). This is the snippet the project's Connection page shows:

```swift
import SwiftUI
import Traceway

@main
struct MyApp: App {
    init() {
        let info = Bundle.main.infoDictionary
        Traceway.initialize(
            token: info?["TracewayToken"] as? String ?? "",
            options: TracewayOptions(
                version: "1.0.0",
                screenCapture: true
            )
        )
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}
```

UIKit does the same in the app delegate:

```swift
func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
) -> Bool {
    let info = Bundle.main.infoDictionary
    Traceway.initialize(
        token: info?["TracewayToken"] as? String ?? "",
        options: TracewayOptions(version: "1.0.0", screenCapture: true)
    )
    return true
}
```

- `token` is the project token. The SDK sends to Traceway Cloud (`https://cloud.tracewayapp.com`) by default. Pass `baseUrl` (a `String`) only for a self-hosted server: `baseUrl: info?["TracewayURL"] as? String ?? ""`.
- Swift keeps `Traceway.init(...)` for initializers, so iOS calls it `initialize` (the browser and Android say `init`, Flutter `run`).
- `initialize` never crashes the app. With settings it cannot use (no token, a `baseUrl` that is not an HTTP(S) URL) it logs one `[Traceway] not started: …` line, returns `nil` and the SDK stays off. A second call does not start a second client. A `baseUrl` without a scheme gets `https://`, and a blank one is Traceway Cloud.
- Without debug the console still says once when the token is rejected (HTTP 401), when the server does not take iOS 2.x telemetry (HTTP 404) and when a call came before `initialize`. With `debug: true` it logs `[Traceway] started (2.1.0) for https://…` and why an issue was dropped.
- A SwiftUI app records no screens by itself (its window, `NavigationStack` and sheets use SwiftUI's own controllers): add `.tracewayScreen("/path")` to each screen's root view (see "Manual capture"). UIKit view controllers need no code.
- `screenCapture: true` turns on the wireframe replay. Leave the other defaults; do not add `recordAllSessions: true` to prove that setup works.

### Where the token comes from

Never commit the token as a literal. The Connection page and `traceway setup` use the same wiring. When the app already keeps its secrets in a gitignored `.xcconfig` that its configuration file `#include?`s (for example `Config/Secrets.xcconfig`), use that file as the plan's `envFile` instead of adding a `Traceway.xcconfig`: no new `.gitignore` line or include is needed.

1. `Traceway.xcconfig`, kept out of version control. Set it as the app target's configuration file in the project's Info tab, or include it from the configuration file the app already uses (`#include? "Traceway.xcconfig"` does not fail when the file is missing, for example on a fresh clone):

   ```
   // "//" starts a comment here, so a URL is written as https:/$()/host.
   TRACEWAY_TOKEN = YOUR_TOKEN
   ```

   On a self-hosted server it also has `TRACEWAY_URL = https:/$()/traceway.mycompany.com`. In an `.xcconfig` everything after `//` is a comment, and `$()` expands to nothing, so the build setting is the full URL.
2. `Info.plist` passes the build settings to the app (the target's Info tab, or the plist file):

   ```xml
   <key>TracewayToken</key>
   <string>$(TRACEWAY_TOKEN)</string>
   ```

   Off Cloud also `<key>TracewayURL</key><string>$(TRACEWAY_URL)</string>`.
3. The app reads `Bundle.main.infoDictionary`, as in the snippet above. A build without the file gets an empty token and runs without the SDK.

A local or plain-HTTP server: the Simulator reaches the Mac's `localhost` directly; a device needs the Mac's LAN address in `TRACEWAY_URL`. App Transport Security blocks plain `http://`: add `NSAppTransportSecurity` › `NSAllowsLocalNetworking` = `YES` to the app's `Info.plist` (for `localhost`, `.local` names and LAN addresses; keep it out of release builds if the app has no other use for it), or an `NSExceptionDomains` entry for a named plain-HTTP host. Traceway Cloud needs none of this.

The setup plan entry for an iOS app (SKILL.md Step 3):

```json
{"name": "Shop iOS", "framework": "ios",
 "envFile": "ios/Traceway.xcconfig", "envVar": "TRACEWAY_TOKEN"}
```

The CLI (and the skill's wait script) writes `TRACEWAY_TOKEN` there. On a server other than Traceway Cloud it also writes `TRACEWAY_URL` (or the plan's `urlEnvVar`), stored as `https:/$()/host`. It creates the file's directory when it is missing. It does not edit `Info.plist` or the Xcode project: add the two keys and the configuration file reference yourself. The project's upload token for dSYMs is written only when the plan names `uploadTokenEnvFile` or `uploadTokenEnvVar` (default variable `TRACEWAY_UPLOAD_TOKEN`); otherwise it stays on the project's Connection page (see "Symbol upload"). Confirm the `.xcconfig` is gitignored before the plan is submitted.

### Upgrading from 2.0

2.0 started with `Traceway.initialize(connectionString: "token@https://host", options: ...)` (and the older alias `Traceway.start`). Both still work, are deprecated and log one warning naming the new form: `Traceway.initialize(token: "token", baseUrl: "https://host", options: ...)`, and for Traceway Cloud just `token:`. Every older spelling of the string keeps working (with or without the old path suffix, a trailing slash, no scheme). When you touch an existing 2.0 integration, move it to `token:` and keep the value in the same build setting.

## What is captured automatically

After `initialize`:

- **Crashes**, always reported (they bypass `ignoreErrors` and `beforeCapture`):
  - uncaught `NSException`s, written to disk and sent before the app ends;
  - fatal signals (`SIGTRAP`, `SIGILL`, `SIGABRT`, `SIGSEGV`, `SIGBUS`, `SIGFPE`: Swift traps, `fatalError`, force-unwrapped `nil`, bad memory access), reported on the **next launch** with the session they happened in. The handler runs on an alternate signal stack (`sigaltstack`), so stack overflows are recorded too. Crash reporters installed before Traceway still get every crash.
- **Exit reasons** (`captureExitReasons`, on by default), from MetricKit (iOS 14+, delivered at the next launch):
  - a crash the SDK's own handlers did not record becomes an issue of type `NativeCrash`;
  - a watchdog termination (`0x8badf00d`) becomes an issue of type `WatchdogTermination` and counts `app.anr`;
  - a run that most likely ran out of memory in the foreground counts one `app.oom` at the next start. It is a heuristic: a count, never an issue.
- **App hangs** (`captureAppHangs`, on by default): a main thread blocked for `appHangThresholdMs` (2 s) in the foreground becomes one `AppHang` issue, `Main thread blocked for 3.2 s`, with the main thread's stack and the screen. A hang that ends the app is reported at the next start with `app.hang.fatal: true`. Hangs in the background or while a debugger is attached are not reported unless `reportHangsInDebug: true`.
- **Log lines** (signal `console`): `NSLog`, `os_log` and `Logger` through the unified log on iOS 15 and later (`log.source: oslog`, read every 10 s and right before an exception), and `Traceway.log` (`log.source: api`). `print` is captured only with `captureStdout: true` (`log.source: stdout`, and `stderr` for standard error); it is off by default because it redirects file descriptors 1 and 2. System frameworks' lines and the SDK's own are left out.
- **Breadcrumbs**, kept on the device (2 MiB, 10 minutes) and uploaded with an exception: URLSession requests as client spans, screens (view controllers that appear, `.tracewayScreen("/path")` on SwiftUI views, `recordNavigation`), taps with a CSS-like selector of the view (`UIButton#pay`), custom events, log lines.
- **Sessions** (`session.start`, `session.end`), and with `screenCapture: true` a **wireframe replay**: an exception uploads the last 30 seconds.
- **App performance metrics** (`captureMetrics`, on by default), uploaded every five minutes and when the app goes to the background: `app.start.duration`, `app.screen.ttid`, `app.screen.ttfd`, `app.frames.slow` and `app.frames.frozen`, `app.hang` and `app.hang.duration`, `app.crash`, `app.anr`, `app.oom`, `app.session`, `app.user`, `app.user.affected`, `app.memory.peak` and `app.request.*`. The iOS project's **Mobile Performance** dashboard charts them.

Errors and messages stream. Every other signal is buffered until an exception, a trigger or a capture uploads it, unless a Data Collection rule says otherwise. Identical errors within a second collapse into one, and at most 60 errors a minute are sent.

The SDK never takes the app down. It has no traps in its sources, contains its own failures (network, server settings, your callbacks) and lets the app's own operation go on. Public calls before `initialize` or after `shutdown` do nothing.

## Manual capture

```swift
do {
    try await placeOrder(cart)
} catch {
    Traceway.captureException(error, attributes: ["order.step": "payment"])
}

Traceway.captureMessage("Checkout opened with an empty cart")
Traceway.log("Coupon applied", level: .info, attributes: ["coupon": "WINTER10"])  // trace, debug, info, warn, error, fatal
Traceway.track("checkout_failed", attributes: ["reason": "card_declined"])          // an event triggers can match
Traceway.recordAction(category: "cart", name: "add_item", data: ["sku": "SKU-123"])

Traceway.setAttribute("user.id", user.id)   // on every later record; rules and captures can target it
Traceway.setAttributes(["plan": "pro"])
Traceway.removeAttribute("plan")
Traceway.clearAttributes()

Traceway.flush(timeout: 5)                  // seconds; at most 2 s on the main thread
```

- `captureException` records the stack of the caller. Both issue types carry the screen (`app.screen.name`, `app.screen.route`, `app.screen.previous`, `app.screen.duration_ms`, `viewport`).
- `ignoreErrors` (default `TracewayOptions.defaultIgnoreErrors`: offline, timeouts, 4xx) and `beforeCapture` filter exceptions, messages and `AppHang` issues, never crashes.
- `user.id` is an attribute; there is no `setUser`. `Traceway.instanceId` and `Traceway.currentSessionId` are properties.

Screens, routes and metrics:

```swift
CheckoutView().tracewayScreen("/checkout")   // SwiftUI views report their screen: push, then pop when it comes back
OrderView(order: order).tracewayScreen("/orders/\(order.id)", route: "/orders/:id")
Button("Pay now") { pay() }.tracewayIdentifier("pay")   // names the SwiftUI control for taps and click steps: #pay
Traceway.recordNavigation("/orders/\(order.id)", action: "push", from: "/orders", route: "/orders/:id")
Traceway.setRoute("/orders/:id")            // the current screen's route template, until the next screen
Traceway.reportFullyDisplayed()             // the screen shows real content: ends app.screen.ttfd
Traceway.metric("checkout.total", value: 129.9, dimensions: ["currency": "EUR"])
Traceway.count("checkout.completed")
Traceway.setMetricDimension("plan", "pro")  // custom.plan on every later metric; nil removes it
```

View controllers are reported without code; once any view uses `.tracewayScreen`, hosting controllers are no longer screens of their own. SwiftUI's own `.accessibilityIdentifier` cannot be read, so SwiftUI controls are named with `.tracewayIdentifier`; without it a tap names the hosting view. Custom metric names are `[a-z][a-z0-9_.]`, outside `app.`, `browser.`, `traceway.` and `custom.`, with at most two dimensions; never a user id.

Interactions and spans of your own:

```swift
let checkout = Traceway.startInteraction("checkout", attributes: ["cart.items": cart.count])
defer { checkout.end() }
do {
    // URLSession requests made inside run {} become child spans of "checkout"
    let order = try await checkout.run { try await api.placeOrder(cart) }
    checkout.setAttribute("order.id", order.id)
} catch {
    checkout.captureException(error)   // reported, linked, marks it failed
}

// Traceway.tracer is an OpenTelemetryApi Tracer, nil before initialize.
let span = Traceway.tracer?.spanBuilder(spanName: "render-invoice").startSpan()
defer { span?.end() }
```

Spans from `Traceway.tracer` carry the session id and nest under a running interaction. Traceway never registers itself as the global OpenTelemetry (`OpenTelemetry.instance`): an app's own setup stays untouched, and what libraries record through the global providers is not sent to Traceway. A failed interaction (an exception captured through it, or `end(failed: true)`) is sent with its exception; a successful one waits in the buffer like other spans. The async `run` stays on the caller's actor. To link requests to a backend trace, list the API prefixes in `propagateTraceHeaderUrls`; it is empty by default, so no request gets a `traceparent` header until you do.

## Options

`TracewayOptions(...)` in its parameter order, the same order as the other Traceway mobile SDKs. It is `Sendable` (a `static let` of it is fine in Swift 6), and the SDK works in Swift 6 language mode without `@preconcurrency`. The `capture*` options, `screenCapture`, `replayImages` and `sessions` are ceilings: a Data Collection rule can lower what is collected, never raise it. `sampleRate`, `baselineSampleRate` and `recordAllSessions` are defaults a rule can change either way.

| Option | Default | Description |
|---|---|---|
| `sampleRate` | `1.0` | Share of exceptions and messages reported |
| `debug` | `false` | Log SDK activity with `NSLog` |
| `version` | bundle `CFBundleShortVersionString` | `service.version`; release comparison and rules use it |
| `serviceName` | bundle identifier | `service.name` |
| `environment` | none | `deployment.environment.name` |
| `attributes` | `[:]` | Attributes on every record |
| `screenCapture` | `false` | Record a wireframe replay |
| `replayImages` | `.always` | Most the replay may do with pictures: `.off`, `.wifi`, `.always` |
| `fps` | `4` | Most wireframe frames per second (1 to 15) |
| `maxBufferFrames` | `150` | Frames in the replay ring; the upper bound of every clip |
| `persistToDisk` | `true` | Keep state and undelivered telemetry on disk; needed for hard-crash capture |
| `captureLogs` | `true` | `Traceway.log` and the unified log (`NSLog`, `os_log`, `Logger`) |
| `captureNetwork` | `true` | URLSession requests as client spans |
| `captureNavigation` | `true` | Screens |
| `captureUserActions` | `true` | Taps, and the click steps of recording triggers |
| `captureMetrics` | `true` | App performance metrics |
| `metricDimensions` | `[:]` | Up to two dimensions of your own on every metric (`custom.<key>`) |
| `recordAllSessions` | `false` | Stream breadcrumbs and replay all the time |
| `collection` | none | Local collection settings, the same document a Data Collection rule holds |
| `sessions` | `true` | Send `session.start` / `session.end` |
| `baselineSampleRate` | `0` | Share of sessions whose breadcrumbs stream continuously |
| `remoteConfig` | `true` | Poll for the Data Collection rules over OpAMP |
| `exportDelayMs` | `1000` | How long finished records wait to be sent as one batch |
| `telemetry` | none | Another OTLP/HTTP receiver for logs, spans and metrics (see below) |
| `propagateTraceHeaderUrls` | `[]` | URL prefixes whose requests carry W3C `traceparent` |
| `ignoreErrors` | `nil` (`defaultIgnoreErrors`) | Strings (substring) or `.regex(...)` matched against `Type: message`; `[]` reports everything |
| `beforeCapture` | none | `@Sendable (TracewayCapturedError) throws -> Bool`, called on the thread that captured the issue (often not the main one); `false` drops the issue, a throw keeps it |
| `captureExitReasons` | `true` | MetricKit crashes and watchdog terminations, and the `app.oom` count |
| `captureAppHangs` | `true` | `AppHang` issues with the main thread's stack |
| `appHangThresholdMs` | `2000` | How long the main thread must be blocked to be an `AppHang` (250 to 60,000) |
| `reportHangsInDebug` | `false` | Report hangs (`app.hang`, `AppHang`) while a debugger is attached |
| `captureStdout` | `false` | Record `print` and standard error lines; redirects file descriptors 1 and 2 |

`token` and `baseUrl` are parameters of `initialize`, not options. `token` is optional: `Traceway.initialize(options: …)` with a `telemetry` receiver runs standalone.

## Session replay

`screenCapture: true` records the view hierarchy as an HTML wireframe that the dashboard replays with its rrweb player. No screenshot or video is ever taken. Up to `fps` frames a second, and only when the screen changed. Labels, buttons, fields, controls, SF Symbols and Liquid Glass are drawn from their state; web views, maps, video and Metal views are labeled placeholders. SwiftUI text shows as its words only while an assistive technology runs (VoiceOver, Voice Control); otherwise it is a neutral bar. The keyboard is a plain box with the word "Keyboard", never its keys. A session uploads at most 50 MiB and 10,000 replay events.

**Masks.** Typed text is replaced with `*` (secure fields always show bullets) unless a rule sets `replay.maskAllInputs: false`. Hide more with:

```swift
// UIKit
cardNumberLabel.tracewayMask = .blank()          // a box, nothing inside recorded
profileView.tracewayMask = .blur                 // layout kept, text as *, images as placeholders

// UIKit, without code (also in Interface Builder's Identity inspector)
balanceView.accessibilityIdentifier = "traceway-mask"    // as .blur
pinView.accessibilityIdentifier = "traceway-blank"       // as .blank()

// SwiftUI
Text(card.number).tracewayMask(.blank(color: .gray))
ProfileView().tracewayMask()                     // .blur
```

SwiftUI's `accessibilityIdentifier` cannot be read from the view hierarchy, so SwiftUI uses the modifier. A masked view's text is also left out of recorded taps. `tracewayPrivacyMask` (property and modifier) still works and is deprecated.

**Images.** The pictures image views show can be part of the replay, never a screenshot of the window. Each is drawn at its displayed size (one pixel per point): an opaque picture as JPEG at quality 0.45, at most 384 px a side; one with transparency always as PNG, at most 192 px a side. A picture over 48 KiB is tried once at half size, then dropped. The recorder keeps up to 128 pictures, at most 4 MiB per session. The project's `replay.images` setting decides when:

| `replay.images` | Pictures |
|---|---|
| `off` | never: images are placeholders |
| `wifi` (default) | on an unmetered connection only (not cellular, a personal hotspot or Low Data Mode) |
| `always` | on any connection |

`replayImages` (`.always` by default) is the ceiling: `.off` keeps every image a placeholder whatever the rules say. Images under a mask are never included.

## Send telemetry to another OpenTelemetry provider

`telemetry` sends logs, spans and metrics to any OTLP/HTTP receiver (an OpenTelemetry Collector, Grafana, Honeycomb). With a token, exceptions, messages, session records, OpAMP and replays stay on Traceway, and the Traceway token is never sent to the receiver:

```swift
var options = TracewayOptions()
options.telemetry = TracewayTelemetry(
    url: "https://otlp.your-provider.com",
    headers: ["x-api-key": otlpKey]
)
Traceway.initialize(token: token, options: options)
```

The shared `url` gets `/v1/logs`, `/v1/traces` and `/v1/metrics` appended. `logs`, `traces` and `metrics` each take a `TracewayTelemetryEndpoint(url:headers:)` with a complete URL. Without a token the SDK runs standalone and sends only there (it then needs a `url` or all three endpoints): `Traceway.initialize(options: TracewayOptions(telemetry: TracewayTelemetry(url: "https://collector.example.com/otlp")))`. No remote config, replay or session records, and exceptions are OTLP log records. Any key built into the app can be read out of it: use an ingestion key that can only write. Set this up only when the user asks for it.

## Symbol upload

Release stacks arrive as binary image UUIDs and offsets. They stay unreadable until the build's **dSYMs** are uploaded; the server then symbolicates crashes, `NativeCrash` and `AppHang` stacks against them. Upload them for every release build: a crash only resolves against the exact build it came from.

- **Upload token.** Uploads use the project's upload token (Connection page, Symbol Upload), not the project token. `traceway setup` creates it with the project. Keep it in the gitignored `Traceway.xcconfig` (the plan's `uploadTokenEnvFile` can write it there) or a CI secret; never in a committed `.xcconfig`. A `readonly` member cannot see it.
- **The uploader** is `Scripts/upload_symbols.sh` in the SDK package. It reads `TRACEWAY_UPLOAD_TOKEN` and `TRACEWAY_URL` (unset or blank is `https://cloud.tracewayapp.com`, so set it only for a self-hosted server), finds `*.dSYM/Contents/Resources/DWARF/*` under `DWARF_DSYM_FOLDER_PATH` (or `--dsym-dir`) and posts each file. It skips builds that are not Release (unless `TRACEWAY_UPLOAD_DEBUG = YES`) and never fails a build: no token, no dSYM or a failed upload (with its HTTP status) are Xcode warnings (`warning: traceway: …`). `--dry-run` lists what it would send.

From Xcode, add a **Run Script** build phase, last in the app target's Build Phases, that runs the script from the package checkout:

```sh
"${BUILD_DIR%/Build/*}/SourcePackages/checkouts/traceway-ios/Scripts/upload_symbols.sh"
```

(or a copy in the app repo, `"${SRCROOT}/Scripts/upload_symbols.sh"`). Set **User Script Sandboxing** to **No** for the app target (`ENABLE_USER_SCRIPT_SANDBOXING = NO`; Xcode 15+ turns it on for new projects, and the build log then says `Sandbox: bash deny file-read-data`); with it on, the phase must list `$(DWARF_DSYM_FOLDER_PATH)/$(DWARF_DSYM_FILE_NAME)` as an input file and only the app's own dSYM is uploaded. Xcode passes build settings (not scheme environment variables) to the phase as environment variables, so `TRACEWAY_UPLOAD_TOKEN` and `TRACEWAY_URL` from `Traceway.xcconfig` reach the script (the `$()` in the URL expands to nothing). The token never reaches the app, because `Info.plist` does not reference it. Debug builds have no dSYM (Debug Information Format `DWARF`): to symbolicate Debug crashes too, set **DWARF with dSYM File** for Debug and `TRACEWAY_UPLOAD_DEBUG = YES`. Without Xcode, add the phase to the project generator (XcodeGen `postBuildScripts`) or give the user these steps with the install steps above.

From CI, after archiving:

```bash
xcodebuild -scheme MyApp -configuration Release -archivePath build/MyApp.xcarchive archive
TRACEWAY_UPLOAD_TOKEN="$TRACEWAY_UPLOAD_TOKEN" ./Scripts/upload_symbols.sh --dsym-dir build/MyApp.xcarchive/dSYMs
```

The dashboard's Symbol Upload card shows the same upload as one `curl -X POST https://cloud.tracewayapp.com/api/symbols/upload -H "Authorization: Bearer <upload token>" -F "files=@<dSYM DWARF file>"`. Self-hosted servers need blob storage (S3 or a persistent volume), or uploaded symbols disappear when the container is recreated.

## Verify

Set the dashboard's time picker to the last 15 minutes.

1. Run with `TracewayOptions(debug: true)`: the Xcode console logs `[Traceway] started (2.1.0) for https://…`. A `[Traceway] not started: …` line says why it did not start (most often an empty token: the `.xcconfig` is not the target's configuration file, or the `Info.plist` key is missing).
2. In a Debug build, call `Traceway.captureException(SomeError())` from a button, then `Traceway.flush(timeout: 5)`. The issue appears on `/issues` within seconds, with its breadcrumbs and, with `screenCapture: true`, the replay clip.
3. `/sessions` lists the session, with its timeline and replay.
4. Crash path: run a Release build **without the debugger** (install from Xcode, stop, launch from the home screen), crash it (`fatalError()`, a force-unwrapped `nil`), then launch it again. The crash appears on `/issues`. With the debugger attached, lldb catches the signal first and nothing is recorded.
5. After uploading that build's dSYMs, the crash's stack shows function names and `file:line`, not bare addresses.
6. Send the app to the background (that uploads metrics), then open the **Mobile Performance** dashboard on `/dashboards`: app start, frames and sessions have data.
7. Add a temporary rule on the project's **Data Collection** page (for example console `stream`). It applies at the next poll: at start, when the app returns to the foreground, every five minutes, or at once with `Traceway.refreshConfig()`. Remove the rule afterwards.

## Objective-C apps

The API is Swift-only. An Objective-C app adds one Swift file to the app target and calls it:

```swift
import Foundation
import Traceway

@objc final class TracewayBridge: NSObject {
    @objc static func start(token: String) {
        Traceway.initialize(token: token, options: TracewayOptions(screenCapture: true))
    }

    @objc static func captureException(_ error: NSError) {
        Traceway.captureException(error)
    }
}
```

Call `[TracewayBridge startWithToken:token]` from `application:didFinishLaunchingWithOptions:`, with the token read from `Info.plist` as above (`[[NSBundle mainBundle] objectForInfoDictionaryKey:@"TracewayToken"]`), and `#import "MyApp-Swift.h"` (the header Xcode generates from the target's module name) where it is used. Add a bridge method for each other call the app needs.

## Non-Swift apps: OpenTelemetry into Traceway

An iOS or Apple app the SDK does not target (a cross-platform stack with no Traceway mobile SDK, or a codebase standardized on OpenTelemetry) has no native Traceway SDK. An Objective-C app does not belong here: it uses the SDK through a Swift bridge file (see "Objective-C apps"). Use an OpenTelemetry distribution such as **Honeycomb's** and point its OTLP exporter at Traceway. This is the OTLP/HTTP path the backends use (SKILL.md "Backend OTel Setup"), so the rules in `data-model.md` apply. The project still uses the framework **iOS**.

Override the distribution's endpoint and auth:

- **Endpoint**: `https://cloud.tracewayapp.com/api/otel` (a self-hosted server: its own address plus `/api/otel`). The SDK appends `/v1/traces`, `/v1/metrics`, `/v1/logs`.
- **Auth header**: `Authorization: Bearer YOUR_TOKEN`. Traceway reads this header, not Honeycomb's `x-honeycomb-team`, so set it explicitly.
- **`service.name`** and **`service.version`**: set both; they name the app and enable release comparison.

Both `build()` and `Honeycomb.configure(options:)` throw, so call them in a `do` / `catch`, once, as early as possible:

```swift
import Honeycomb

do {
    let options = try HoneycombOptions.Builder()
        .setServiceName("my-ios-app")
        .setServiceVersion("1.0.0")
        .setAPIEndpoint("https://cloud.tracewayapp.com/api/otel")
        .setHeaders(["Authorization": "Bearer \(tracewayToken)"])
        .build()
    try Honeycomb.configure(options: options)
} catch {
    NSLog("Honeycomb configure failed: \(error)")
}
```

The method is `setAPIEndpoint`, with a capital `API`; `setApiEndpoint` does not compile. Do not drop the line: without it the exporter falls back to Honeycomb's own endpoint and nothing reaches Traceway. No Honeycomb API key is needed, because `build()` asks for one only when the endpoint is a Honeycomb host. Read `tracewayToken` from a build setting as in "Where the token comes from", never a committed literal. Builder names vary by distribution version; what matters is the endpoint, the `Authorization: Bearer` header and the service name. A plain OpenTelemetry Swift setup with an OTLP/HTTP exporter pointed at the same endpoint and header is equivalent.

The backend OTLP rules apply: OTLP/HTTP only (protobuf or JSON), no gRPC, gzip is fine, at most 10 MB per request. Recorded `exception` events become **Issues**. An iOS project treats its data as an end user's device: spans stay spans instead of becoming **Endpoints**. Verify on `/issues` first: record a test error and confirm it appears. This path has no replay, Data Collection rules or `app.*` metrics.
