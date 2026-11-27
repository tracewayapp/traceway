# Android SDK Reference

Native Android apps (Kotlin or Java) use the Traceway Android SDK, `com.tracewayapp:traceway` on Maven Central, and its Gradle plugin `com.tracewayapp.symbols`. Both are version **2.1.0**. The SDK is built on the official OpenTelemetry Java SDK: it reports crashes, native crashes, ANRs and app hangs as issues, keeps logs, requests, screens and taps on the device as breadcrumbs, records a wireframe session replay of views and Jetpack Compose screens, sends `app.*` performance metrics, and follows the project's **Data Collection** rules.

For React Native or Flutter use that platform's file instead. This file is for apps with a real Android (Kotlin/Java) module.

## Choose the version

- **New integration**: install 2.1.0. It needs Traceway Cloud or a self-hosted server that accepts Android projects on OTLP and OpAMP. A self-hosted server from before that receives nothing: upgrade the server first.
- **Older projects on 1.x** (`com.tracewayapp:traceway:1.0.1`, errors and crashes only): keep 1.x unless the user asks to move or needs a 2.x feature (sessions, replay, remote config, captures, performance metrics, trace propagation, another OpenTelemetry backend). Moving is a migration, not a version bump: see "Upgrading from 2.0" below and https://docs.tracewayapp.com/client/android.
- **An app on `2.0.0-beta.1`**: move it to 2.1.0. Read the app's Gradle files (or the version catalog) for the resolved version first.

## Install

```kotlin
// app/build.gradle.kts
dependencies {
    implementation("com.tracewayapp:traceway:2.1.0")
}
```

- `minSdk` 21, `compileSdk` 35 or newer.
- OpenTelemetry Java needs `java.time` and `java.util.function`, so an app whose `minSdk` is below 26 must enable core library desugaring. Without it the SDK does not start on older devices: `Traceway.init` logs and returns null.

  ```kotlin
  android {
      compileOptions { isCoreLibraryDesugaringEnabled = true }
  }
  dependencies {
      coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.5")
  }
  ```

- No permission entry is needed. The library's manifest declares `INTERNET` and `ACCESS_NETWORK_STATE`, and the manifest merger adds both.
- The Gradle plugin is recommended for every app: it captures `android.util.Log` lines and makes R8 release crashes readable. See "Symbol upload".

## Initialize

Call `Traceway.init` from `Application.onCreate()`. This is the snippet the dashboard's Connection page shows:

```kotlin
import android.app.Application
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions

class MyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Traceway.init(
            this,
            TracewayOptions(
                token = BuildConfig.TRACEWAY_TOKEN,
                version = "1.0.0",
                screenCapture = true, // wireframe session replay
            ),
        )
    }
}
```

- `version` defaults to the app's `versionName`. Leave the line out unless the app reports another version.
- A self-hosted server adds `baseUrl = BuildConfig.TRACEWAY_URL`. Traceway Cloud (`https://cloud.tracewayapp.com`) is the default, so cloud apps pass only the token.
- Register the class in `AndroidManifest.xml`, or `onCreate()` never runs:

  ```xml
  <application android:name=".MyApp" ...>
  ```

- `Traceway.init` never throws. A bad `baseUrl` or `telemetry`, no token and no `telemetry`, or any failure while starting is logged (tag `Traceway`), `init` returns null, and nothing is installed. Every `Traceway` method is a no-op before `init`. A second `init` returns the running client.
- It runs in the app's main process only and returns null in another process (`:sync`, `:remote`). `allProcesses = true` runs it in every process.

From Java, build the options with `TracewayOptions.builder()` (one setter per option, `null` means the default). Every `Traceway` method is a `@JvmStatic` with overloads for the optional parameters:

```java
Traceway.init(this, TracewayOptions.builder()
        .token(BuildConfig.TRACEWAY_TOKEN)
        .screenCapture(true)
        .build());
```

### Where the token comes from

The token never goes into a committed file. The dashboard and `traceway setup` use the same wiring: `local.properties` at the project root (Android Studio's `.gitignore` already excludes it; confirm it) and a `BuildConfig` field.

```properties
# local.properties
TRACEWAY_TOKEN=YOUR_TOKEN
# Only for a self-hosted server:
TRACEWAY_URL=https://traceway.mycompany.com
```

```kotlin
// app/build.gradle.kts
import java.util.Properties

val localProperties = Properties().apply {
    rootProject.file("local.properties").takeIf { it.exists() }?.reader()?.use { load(it) }
}

android {
    buildFeatures { buildConfig = true }
    defaultConfig {
        buildConfigField("String", "TRACEWAY_TOKEN", "\"${localProperties.getProperty("TRACEWAY_TOKEN", "")}\"")
        // Only for a self-hosted server:
        buildConfigField("String", "TRACEWAY_URL", "\"${localProperties.getProperty("TRACEWAY_URL", "")}\"")
    }
}
```

CI has no `local.properties`: write it from the CI secrets before the build, or read `System.getenv("TRACEWAY_TOKEN")` first in the `buildConfigField` line.

The setup plan entry for an Android app (paths are relative to the plan file):

```json
{"name": "Shop Android", "framework": "android",
 "envFile": "android/local.properties", "envVar": "TRACEWAY_TOKEN"}
```

- `traceway setup` writes `TRACEWAY_TOKEN`. On a server other than Traceway Cloud it also writes `TRACEWAY_URL` (the plan's `urlEnvVar` names another key; `.properties` files also accept dotted keys such as `traceway.token`, whose URL key is then `traceway.url`).
- The project also gets an upload token. It is written only when the plan names `uploadTokenEnvFile` or `uploadTokenEnvVar` (default key `TRACEWAY_UPLOAD_TOKEN`), for example `"uploadTokenEnvFile": "android/local.properties"`. Otherwise it stays on the project's Connection page.

A local server (`traceway setup` then writes `TRACEWAY_URL=http://localhost:8082`, the server's own address): run `adb reverse tcp:8082 tcp:8082`, and the emulator or a USB device reaches the host at that same `localhost` address, so the app and the Gradle plugin (which uploads from the host) share the one value. Without it the emulator reaches the host as `10.0.2.2`; put that only into the app's `buildConfigField` (for example in `buildTypes { debug { … } }`), not into `TRACEWAY_URL`, which the plugin reads. Android 9+ blocks plain `http://`: allow it for that host in debug builds only, with `app/src/debug/res/xml/network_security_config.xml`

```xml
<network-security-config>
    <domain-config cleartextTrafficPermitted="true">
        <domain includeSubdomains="false">localhost</domain>
        <domain includeSubdomains="false">10.0.2.2</domain>
    </domain-config>
</network-security-config>
```

and `android:networkSecurityConfig="@xml/network_security_config"` on the `<application>` of `app/src/debug/AndroidManifest.xml`. Traceway Cloud needs none of this.

### Upgrading from 2.0

- The 2.0 start `Traceway.init(application, connectionString, options)` still works. It is deprecated and logs one warning that names the new form: the token goes into `TracewayOptions(token = …)` and a self-hosted server into `baseUrl`. Every older spelling of the string keeps working.
- `TracewayOptions` now starts with `token` and `baseUrl`, so pass its parameters by name (Java callers use the builder). `legacyConnectionString` is gone; reports a 1.x build left on the device are sent once at the first start.
- `recordLog` is deprecated in favour of `Traceway.log`. Network errors (`UnknownHostException`, timeouts, 4xx) are no longer reported by default (`ignoreErrors`).
- Apply the Gradle plugin at `version "2.1.0"` (it was `1.0.1`); `proguardUuid = BuildConfig.TRACEWAY_PROGUARD_UUID` is no longer needed.
- From 1.x: enable desugaring below `minSdk` 26, and expect issues to regroup once, because stack traces now arrive as OpenTelemetry `exception.*` attributes.

## What is captured automatically

- **Uncaught exceptions** on every thread. The exception, its breadcrumbs and its replay clip are written to disk before the previous handler ends the process, then sent (off the main thread, within 2.5 s) or at the next start. The crash still reaches the handler that was installed before. Crashes bypass `ignoreErrors` and `beforeCapture`.
- **Exit reasons of earlier runs** (`captureExitReasons`, default `true`; Android 11 / API 30+, read once per start):
  - a native crash is an issue of type `NativeCrash` (on Android 12+ with the tombstone's signal, abort message and crashing-thread frames), counted in `app.crash`;
  - an ANR is an issue of type `ApplicationNotResponding` with the main thread's stack, plus an `app.anr` count;
  - a low-memory kill is an `app.oom` count, no issue.
  Below Android 11, native crashes and ANRs are not reported. There is no in-process native signal handler.
- **App hangs** (`captureAppHangs`, default `true`): a main thread blocked for `appHangThresholdMs` (default 2000) in the foreground is an `AppHang` issue (`Main thread blocked for 3.2 s`) with the stack it was stuck in and the screen. A hang the process does not survive is reported at the next start with `app.hang.fatal = true`, unless Android recorded an ANR for that run. Not under a debugger unless `reportHangsInDebug = true`.
- **Sessions**: `session.start`, `session.end` and updates. A session ends after 15 minutes without activity or after 60 minutes.
- **Breadcrumbs**, kept on the device and uploaded with an error:
  - log lines: `System.out` / `System.err` (`log.source` `stdout` / `stderr`), `Traceway.log` (`api`) and, with the Gradle plugin, `android.util.Log` and Timber (`logcat`, with `log.tag`);
  - screens: activities and AndroidX fragments as `/ClassName`; Compose destinations once the `NavController` is tracked (see "Manual capture");
  - taps, named by a CSS-like selector of the view (`MaterialButton#pay`);
  - HTTP calls made through `TracewayOkHttpInterceptor` (add it as an application interceptor: `OkHttpClient.Builder().addInterceptor(TracewayOkHttpInterceptor())`, with `import com.tracewayapp.traceway.network.TracewayOkHttpInterceptor`). Other HTTP stacks use `Traceway.client?.startRequest(method, url)` and `endRequest(...)`.
- **App performance metrics** (`captureMetrics`): `app.start.duration`, `app.screen.ttid`, `app.screen.ttfd`, `app.frames.slow` / `app.frames.frozen`, `app.hang`, `app.crash`, `app.anr`, `app.oom`, `app.session`, `app.user`, `app.user.affected`, `app.memory.peak`, `app.request.*`. They upload every 5 minutes and when the app goes to the background, and fill the project's **Mobile Performance** dashboard.
- **Replay**, with `screenCapture = true` (see "Session replay").

The SDK never takes the app down. Every public method contains its own failures and accepts Java nulls, every hook it installs (activity, frame and draw callbacks, the hang probe, the OkHttp interceptor, encoders) contains its failures, and the app's own exceptions still reach the app unchanged. `debug = true` logs what was contained.

## Manual capture

```kotlin
try {
    riskyOperation()
} catch (e: Exception) {
    Traceway.captureException(e, mapOf("checkout.step" to "pay"))
}
Traceway.captureMessage("User completed checkout")            // an issue
Traceway.log("Payment declined", TracewayLogLevel.WARN, mapOf("order.id" to order.id))

Traceway.track("checkout_failed", mapOf("reason" to "card_declined"))   // triggers can match it
Traceway.recordAction("cart", "add_item", mapOf("sku" to "SKU-123"))

Traceway.setAttribute("user.id", user.id)   // the user; rules and captures can target it
Traceway.setAttributes(mapOf("plan" to "pro"))
Traceway.currentSessionId                   // Java: Traceway.getCurrentSessionId()

Traceway.metric("checkout.duration", 1234.0, mapOf("plan" to "pro"))
Traceway.count("cart.add")
Traceway.reportFullyDisplayed()             // ends the screen's time to full display

Traceway.flush()                            // blocks; on the main thread at most 2 s
```

`TracewayLogLevel` is `TRACE`, `DEBUG`, `INFO` (default), `WARN`, `ERROR` or `FATAL`. There is no `setUser`: the user is the `user.id` attribute.

Interactions trace one user action; requests made inside `run` are its child spans:

```kotlin
val checkout = Traceway.startInteraction("checkout", mapOf("cart.items" to 3))
try {
    checkout.run { api.placeOrder(cart) }
} catch (e: IOException) {
    checkout.captureException(e)   // marks it failed
} finally {
    checkout.end()
}
```

`Traceway.tracer` (Java `Traceway.getTracer()`) is the SDK's OpenTelemetry `Tracer` for spans of your own. It is null before the SDK started and after `shutdown`. Its spans carry `session.id` and nest under the interaction they start in. The SDK never registers itself as `GlobalOpenTelemetry`, so an app's own OpenTelemetry setup is untouched.

```kotlin
val span = Traceway.tracer?.spanBuilder("decode-image")?.startSpan()
try { decode(bytes) } finally { span?.end() }
```

Screens and routes:

- **Compose Navigation**: track the `NavController` where it is created. Each destination becomes a screen with its path and route template (`/orders/8841`, `/orders/:id`).

  ```kotlin
  import com.tracewayapp.traceway.navigation.trackNavigation

  val nav = rememberNavController()
  LaunchedEffect(nav) { Traceway.trackNavigation(nav) }   // Java: TracewayNavigation.track(nav)
  ```

- A Compose screen no controller shows: `Modifier.tracewayScreen("/orders/$id", route = "/orders/:id")` (import `com.tracewayapp.traceway.compose.tracewayScreen`).
- Any other router: `Traceway.recordNavigation(path, action = "push", from = null, route = null)`, e.g. `Traceway.recordNavigation("/orders/8841", route = "/orders/:id")`. From Java the parameters are positional in that order.
- `Traceway.setRoute("/orders/:id")` names the current screen's route template; `setRoute(null)` removes it.

Exceptions, messages and logs carry the screen: `app.screen.name`, `app.screen.route`, `app.screen.previous`, `app.screen.duration_ms` and `viewport`.

## Options

`TracewayOptions`, passed by name in Kotlin, or through `TracewayOptions.builder()` in Java:

| Option | Default | What it does |
|---|---|---|
| `token` | none | The project token. Without one the SDK runs standalone (`telemetry` only) |
| `baseUrl` | `https://cloud.tracewayapp.com` | The Traceway server; set it only for a self-hosted one |
| `sampleRate` | `1.0` | Share of exceptions and messages reported |
| `debug` | `false` | Log what the SDK does to logcat (tag `Traceway`) |
| `version` | the app's `versionName` | `service.version` |
| `serviceName` | the application id | `service.name` |
| `environment` | none | `deployment.environment.name` |
| `attributes` | `{}` | Attributes on every record; rules can target them |
| `screenCapture` | `false` | Record a wireframe replay. A ceiling |
| `replayImages` | `ALWAYS` | The most `replay.images` may allow: `TracewayImagePolicy.OFF`, `WIFI` or `ALWAYS` |
| `fps` | `10` | Most wireframe frames per second (1 to 30) |
| `maxBufferFrames` | `150` | Wireframe frames held on the device; the upper bound of a clip |
| `persistToDisk` | `true` | Keep state and unsent telemetry on disk across restarts (exit reasons need it) |
| `captureLogs` | `true` | `System.out`/`err`, `Traceway.log` and logcat lines. A ceiling for `console` |
| `captureNetwork` | `true` | OkHttp requests through the interceptor. A ceiling for `network` |
| `captureNavigation` | `true` | Activities, fragments, `recordNavigation`. A ceiling for `navigation` |
| `captureUserActions` | `true` | Taps with their selector. A ceiling for `userActions` |
| `captureMetrics` | `true` | The `app.*` performance metrics |
| `metricDimensions` | `{}` | Up to two dimensions of your own on every metric |
| `recordAllSessions` | `false` | Stream breadcrumbs and replay all the time instead of keeping them for errors |
| `collection` | none | Local collection settings, the same document as a Data Collection rule |
| `sessions` | `true` | Send session records |
| `baselineSampleRate` | `0` | Share of sessions whose breadcrumbs stream continuously |
| `remoteConfig` | `true` | Poll the server's Data Collection rules over OpAMP |
| `exportDelayMs` | `1000` | Batch delay for logs and spans |
| `telemetry` | none | Send logs, spans and metrics to another OTLP/HTTP receiver |
| `propagateTraceHeaderUrls` | `[]` | URL prefixes whose requests get a W3C `traceparent` header |
| `ignoreErrors` | `null` (`DEFAULT_IGNORE_ERRORS`) | Patterns (`String`, `Regex`, `Pattern`) matched against `Type: message`. The default drops network noise and 4xx; an empty list reports everything |
| `beforeCapture` | none | `(TracewayCapturedError) -> Boolean`; return false to drop |
| `proguardUuid` | the Gradle plugin's | Set only to pin your own build UUID |
| `propagateTraceHeaderUrlPatterns` | `[]` | Like `propagateTraceHeaderUrls`, as regular expressions |
| `captureExitReasons` | `true` | Native crashes, ANRs and low-memory kills of earlier runs (Android 11+) |
| `captureAppHangs` | `true` | `AppHang` issues for a blocked main thread |
| `appHangThresholdMs` | `2000` | How long the main thread must be blocked for an `AppHang` (at least 250) |
| `reportHangsInDebug` | `false` | Report hangs while a debugger is attached |
| `allProcesses` | `false` | Run in every process of the app, not only the main one |

The `capture*` options, `screenCapture`, `replayImages` and `sessions` are ceilings: a Data Collection rule can lower what is collected, never raise it. Keep the production defaults; do not set `recordAllSessions = true` just to prove setup works.

What is sent, buffered or dropped is changed on the project's **Data Collection** page. The SDK polls at start, every 5 minutes (60 seconds while a capture targets the device), when the app returns to the foreground, and about 2 seconds after `setAttribute` or a new session.

## Session replay

`screenCapture = true` records the view tree as a wireframe: boxes, text, colors and scroll positions as HTML, replayed by the dashboard's rrweb player. No screenshot is taken and no video is encoded; a clip is tens of kilobytes. Jetpack Compose screens are recorded from their semantics. `WebView`, `SurfaceView`, video and maps are labelled placeholders. While the soft keyboard shows, the replay draws a neutral "Keyboard" box; its keys are never recorded. By default an exception uploads the last 30 seconds; captures and rules can stream whole sessions.

**Masks** change only the recording:

| Where | Blur (layout kept, text as `*`, images as placeholders) | Blank (a box, nothing inside recorded) |
|---|---|---|
| XML | `android:tag="traceway-mask"` | `android:tag="traceway-blank"` |
| Kotlin | `view.tracewayMask = TracewayMaskMode.blur()` (import `com.tracewayapp.traceway.tracewayMask`) | `view.tracewayMask = TracewayMaskMode.blank(Color.BLACK)` |
| Java | `Traceway.mask(view, TracewayMaskMode.blur())`, `Traceway.unmask(view)` | `Traceway.mask(view, TracewayMaskMode.blank())` |
| Compose | `Modifier.tracewayMask()` or `Modifier.testTag("traceway-mask")` | `Modifier.tracewayMask(TracewayMaskMode.blank())` or `Modifier.testTag("traceway-blank")` |

A mask covers the view and everything inside it. Text typed into fields is recorded as `*` unless a rule sets `replay.maskAllInputs: false`; password fields always are.

**Images.** The rule's `replay.images` chooses when the pictures image views show are recorded, and `replayImages` caps it (the lower applies):

| `replay.images` | Pictures |
|---|---|
| `off` | never: placeholders |
| `wifi` (default) | on an unmetered connection (Wi-Fi, Ethernet) |
| `always` | on any connection |

Only `ImageView` bitmaps and bitmap backgrounds are recorded, never a screenshot and never anything under a mask; Compose images stay placeholders. Each picture is copied at its displayed size (one pixel per dp): JPEG at quality 0.45 up to 384 px on the longest side, or PNG up to 192 px when it has transparency, so it never replays as a black box. One over 48 KiB after a retry at half size is left out. At most 128 pictures are kept per recorder (the one shown longest ago makes room) and 4 MiB per session.

## Send telemetry to another OpenTelemetry provider

`telemetry` sends logs, spans and metrics to any OTLP/HTTP receiver. With a token, errors, replays, captures and remote config stay on Traceway:

```kotlin
TracewayOptions(
    token = BuildConfig.TRACEWAY_TOKEN,
    telemetry = TracewayTelemetry(
        url = "https://otlp.your-provider.com",              // gets /v1/logs, /v1/traces, /v1/metrics
        headers = mapOf("x-api-key" to BuildConfig.OTLP_KEY),
        // metrics = TracewayTelemetryEndpoint(url = "https://metrics.example.com/v1/metrics"),
    ),
)
```

- Per-signal `logs`, `traces` and `metrics` (`TracewayTelemetryEndpoint(url, headers)`) take complete URLs. Without a shared `url`, the signals not listed stay on Traceway.
- Exceptions, messages and session records still go only to Traceway. Ordinary logs, request spans and metrics go to the receiver, so Traceway's session timeline and Mobile Performance dashboard do not get them.
- Without a token the SDK runs standalone and sends everything to the receiver: no remote config, replay or session records. `telemetry` then needs a shared `url` or all three signal endpoints.
- The receiver must accept OTLP/HTTP protobuf. A key built into the app can be read out of it: use a write-only ingestion key or a collector. Java: `TracewayTelemetry.builder()`.

Set this up only when the user asks for another destination.

## Symbol upload

The `com.tracewayapp.symbols` Gradle plugin (version 2.1.0, the SDK's version; AGP 8.0+ and JDK 17) does two jobs: it rewrites `android.util.Log` calls for logcat capture, and it makes R8 release crashes readable. It is on Maven Central:

```kotlin
// settings.gradle.kts
pluginManagement {
    repositories {
        gradlePluginPortal()
        google()
        mavenCentral()   // resolves com.tracewayapp.symbols
    }
}
```

```kotlin
// app/build.gradle.kts (localProperties as in "Where the token comes from")
plugins {
    id("com.android.application")
    id("com.tracewayapp.symbols") version "2.1.0"
}

android {
    buildTypes {
        release { isMinifyEnabled = true }
    }
}

traceway {
    // The upload token, not the project token. The plugin reads no environment itself.
    uploadToken = System.getenv("TRACEWAY_UPLOAD_TOKEN") ?: localProperties.getProperty("TRACEWAY_UPLOAD_TOKEN", "")
    // The server traceway setup wrote off Cloud; blank or unset is Traceway Cloud, so keep this line on Cloud too.
    url = System.getenv("TRACEWAY_URL") ?: localProperties.getProperty("TRACEWAY_URL", "")
    autoUpload = false                          // true uploads after assembleRelease

    logcat {
        enabled = true            // default true
        minLevel = "info"         // verbose, debug, info (default), warn, error or assert
        includeDependencies = true // default true; Timber lives in a dependency
        // disabledFor("benchmark")
    }
}
```

- The plugin may be applied before or after the Android plugin.
- **Mappings.** The plugin gives each variant and app version a ProGuard UUID, writes it into the app's assets (and `BuildConfig.TRACEWAY_PROGUARD_UUID` when the module builds a `BuildConfig`), and the SDK reads it by itself. Applying the plugin is enough; set `proguardUuid` only to pin your own. The UUID comes from the module path, variant, `versionName` and `versionCode`, so bump the version every release or a new upload overwrites the last mapping.
- Upload after each release build, preferably in CI with the upload token as a secret:

  ```bash
  TRACEWAY_UPLOAD_TOKEN=... ./gradlew :app:assembleRelease :app:uploadReleaseTracewaySymbols
  ```

  The task is `upload<Variant>TracewaySymbols`. A project token is rejected with 401. A minified build with no UUID logs one warning at start: its crashes stay obfuscated.
- The upload token comes from the project's Connection page (Symbol Upload), or `traceway setup` writes it as `TRACEWAY_UPLOAD_TOKEN` when the plan asks. Never commit it.
- **Logcat.** The rewrite calls the real `Log` first, then records the line (`log.source = "logcat"`). It needs the SDK 2.1.0 or newer on the runtime classpath; with an older SDK the build warns once per variant and leaves `Log` alone. Calls below `minLevel` stay plain `Log` calls. R8 rules that strip `Log` calls also strip the rewritten ones. Without the plugin only `System.out`/`System.err` and `Traceway.log` are captured; forward Timber lines with `Traceway.log` then.
- **Without the plugin**, upload by hand and pin the same UUID in `TracewayOptions(proguardUuid = ...)`:

  ```bash
  curl -fsS -X POST https://cloud.tracewayapp.com/api/symbols/upload \
    -H "Authorization: Bearer $TRACEWAY_UPLOAD_TOKEN" \
    -F "proguard_uuid=<build uuid>" \
    -F "files=@app/build/outputs/mapping/release/mapping.txt"
  ```

A self-hosted server needs blob storage (S3 or a persistent volume), or uploaded mappings disappear when the container is recreated. Internals: https://docs.tracewayapp.com/symbolicator/android.

## Verify

Set the dashboard's time picker to the last 15 minutes.

1. **Crash.** Add a button that throws (`throw RuntimeException("Test error from Traceway integration")`), tap it, open the app again. `/issues` shows it with its stack trace, the breadcrumbs before it and, with `screenCapture = true`, the replay clip. It must appear only in the Android project.
2. **Session.** `/sessions` lists the session with **Android SDK** 2.1.0, its timeline (screens, taps, log lines) and the replay.
3. **Metrics.** After the app went to the background once, the **Mobile Performance** dashboard shows app starts, screens and sessions. `traceway metrics discover --since 15m` lists `app.*` names.
4. **Data Collection.** Add a temporary rule (for example `console` to `stream` for this app version), wait for the next poll (bring the app to the foreground), check that log lines now arrive without an error, then delete the rule.
5. **Release mapping.** For a minified build, after `uploadReleaseTracewaySymbols` ran for that build, a crash shows real class, method and `file:line`, not `a.a.a(SourceFile)` frames.
6. **Logcat.** With the plugin, a `Log.i` line appears on the session timeline after an error.

Docs: https://docs.tracewayapp.com/client/android
