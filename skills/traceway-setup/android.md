# Android SDK Reference

Native Android apps (Kotlin or Java) use the Traceway Android SDK, `com.tracewayapp:traceway`, published on Maven Central. **2.x** (the prerelease, `2.0.0-beta.1`) is built on the official OpenTelemetry Java SDK, like the browser SDK 2.x and the Flutter SDK 2.x: it exports exceptions, logs, request spans and `app.*` performance metrics through OTLP (`/api/otel/v1/logs`, `/traces`, `/metrics`), uploads wireframe session replays of the app's views and Jetpack Compose screens to `/api/recordings`, and polls OpAMP (`/api/otel/v1/opamp`) for the Android project's **Data Collection** rules. It needs a Traceway server that accepts Android projects on those endpoints. **1.x** (`1.0.1`) reports errors and crashes only over `/api/report`; keep an existing 1.x app on 1.x unless the user asks to move or needs a 2.x feature (sessions, replay, remote config, captures, performance metrics, trace propagation, another OpenTelemetry backend).

For a cross-platform stack with no native code (React Native, Flutter) use that platform's path instead; this file is for apps with a real Android (Kotlin/Java) module.

## Install

Add the dependency to the app module's `build.gradle.kts`:

```kotlin
dependencies {
    implementation("com.tracewayapp:traceway:2.0.0-beta.1")
}
```

Maven Central is enabled by default in modern Android projects. Minimum SDK is API 21. OpenTelemetry Java needs `java.time` and `java.util.function`, so an app whose `minSdk` is below 26 must enable core library desugaring:

```kotlin
android {
    compileOptions { isCoreLibraryDesugaringEnabled = true }
}
dependencies {
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.5")
}
```

## Initialize

Call `Traceway.init` from `Application.onCreate()`. This wraps the whole process, so every uncaught exception on every thread is captured automatically:

```kotlin
import android.app.Application
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions

class MyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Traceway.init(
            application = this,
            connectionString = "<project-token>@https://<instance>",
            options = TracewayOptions(
                version = BuildConfig.VERSION_NAME,
                screenCapture = true, // wireframe session replay
            ),
        )
    }
}
```

Register the `Application` class in `AndroidManifest.xml`, or `onCreate()` never runs and the SDK is never initialized:

```xml
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application android:name=".MyApp" ...>
        ...
    </application>
</manifest>
```

No permission entry is needed. The SDK's own manifest inside the AAR already declares `INTERNET` and `ACCESS_NETWORK_STATE`, and the Android manifest merger folds both into the app.

Wire the token through a build setting or `BuildConfig` field, not a committed literal. The connection string is `<project-token>@https://<instance>`, as the Connection page shows it; the 1.x form ending in `/api/report` works too. An invalid connection string makes `Traceway.init` log an error (tag `Traceway`) and return `null`; the app keeps running.

## Manual capture

```kotlin
try {
    riskyOperation()
} catch (e: Throwable) {
    Traceway.captureException(e, mapOf("checkout.step" to "pay"))
}

Traceway.captureMessage("User completed onboarding")
Traceway.track("checkout_failed", mapOf("reason" to "card_declined"))
Traceway.setAttribute("user.id", user.id)   // targets rules and captures at this user
Traceway.flush()   // blocks; call off the main thread
```

## What gets captured automatically

After `init`, with no extra wiring:

- **Uncaught Java/Kotlin exceptions** on every thread, via the default `UncaughtExceptionHandler`: the exception, its breadcrumbs and its replay clip are written to disk before the process ends and sent at once or at the next start.
- **Sessions**, and as breadcrumbs kept in an on-device ring buffer and uploaded with each error: `System.out` / `System.err` lines, screens (activities and AndroidX fragments as `/ClassName`), taps with a CSS-like selector of the view (`MaterialButton#pay`), and custom events.
- **App performance metrics**: start time, time to initial and full display, slow and frozen frames, hangs, crashes, sessions and users, HTTP requests.
- With `screenCapture = true`, a **wireframe replay** of the windows (views and Compose semantics as HTML, typed text masked) for the seconds before each error.

Two things need wiring: HTTP calls are recorded through `OkHttpClient.Builder().addInterceptor(TracewayOkHttpInterceptor())` (other stacks call `startRequest` / `endRequest`), and `android.util.Log` lines are not mirrored, so forward them from a Timber tree with `Traceway.recordLog(message, level)`. Compose Navigation reports its routes with `Traceway.recordNavigation("/$route")` from a destination listener. Mask sensitive views with `Traceway.mask(view)`, `android:tag="traceway-mask"` or `Modifier.tracewayMask()`. What is sent, buffered or dropped is changed on the project's Data Collection page; full options and selector rules are at https://docs.tracewayapp.com/client/android.

## Symbolication: upload R8 `mapping.txt` with the Gradle plugin

A release build with `minifyEnabled true` runs R8, which renames classes and methods and rewrites the line-number table, so production crash traces arrive obfuscated and stay unreadable until Traceway retraces them server-side against the build's `mapping.txt` (the Android equivalent of a JavaScript source map). Set this up whenever the app ships a minified release.

**Token.** Symbol uploads authenticate with the dedicated **upload token** (Connection page > Source Maps / Symbol Upload), NOT the project token from the connection string. It is a CI secret, never committed. `readonly` members cannot generate one. Using the connection-string token here is rejected with a 401.

**Apply the plugin.** It is published to Maven Central, so add `mavenCentral()` to the `pluginManagement` repositories in `settings.gradle.kts`:

```kotlin
pluginManagement {
  repositories {
    gradlePluginPortal()
    mavenCentral()   // resolves com.tracewayapp.symbols
  }
}
```

Then apply it to the app module and point it at the instance:

```kotlin
plugins {
  id("com.android.application")                // must already be applied
  id("com.tracewayapp.symbols") version "1.0.1"
}

android {
  buildFeatures { buildConfig = true }   // required for the injected UUID field
  buildTypes {
    release { isMinifyEnabled = true }
  }
}

traceway {
  // The plugin reads no environment variables itself — wire the secret in here.
  uploadToken = System.getenv("TRACEWAY_UPLOAD_TOKEN") ?: ""
  url = "https://<instance>"            // instance base URL
  autoUpload = false                    // true runs upload after assembleRelease
  // proguardUuid = "..."               // optional: pin the build UUID yourself
}
```

Order matters in the `plugins { }` block. Gradle applies entries top to bottom, and `com.tracewayapp.symbols` looks for the Android extension the moment it is applied, so it has to come **after** `com.android.application` (or `com.android.library`). Declared above it, the build fails at configuration time with `com.tracewayapp.symbols must be applied after an Android application/library plugin`. The app module already has the Android plugin line, so add the Traceway line under it rather than pasting a new block above it.

**Wire the UUID into the SDK.** The plugin embeds a ProGuard UUID into `BuildConfig.TRACEWAY_PROGUARD_UUID` and names the uploaded mapping `<uuid>.txt`. The UUID is derived from the module path, variant, and app version (`versionName` + `versionCode`), so **bump the version each release** or a new upload overwrites the previous one (or set `proguardUuid` explicitly). Pass the injected value to the SDK so each reported crash carries the matching UUID:

```kotlin
Traceway.init(
    application = this,
    connectionString = "<project-token>@https://<instance>",
    options = TracewayOptions(
        version = BuildConfig.VERSION_NAME,
        screenCapture = true,
        proguardUuid = BuildConfig.TRACEWAY_PROGUARD_UUID,
    ),
)
```

**Per release.** With `autoUpload = false` (the better fit for CI, so a release build never depends on backend availability) upload explicitly after the build:

```bash
./gradlew assembleRelease uploadReleaseTracewaySymbols
```

With `autoUpload = true` the upload runs automatically after `assembleRelease`. Keep the upload token in a CI secret. The `System.getenv` wiring in the `traceway { ... }` block above is what feeds it to the plugin (there is no built-in environment lookup):

```bash
TRACEWAY_UPLOAD_TOKEN=<upload-token> ./gradlew assembleRelease uploadReleaseTracewaySymbols
```

**Without the plugin.** The upload is a single multipart POST, so any pipeline can do it by hand: send the `mapping.txt` and its build UUID to `/api/symbols/upload`.

```bash
curl -fsS -H "Authorization: Bearer <upload-token>" \
  -F "files=@app/build/outputs/mapping/release/mapping.txt" \
  -F "proguard_uuid=<uuid>" \
  https://<instance>/api/symbols/upload
```

The `proguard_uuid` must match the value the SDK sends. Self-hosted instances must have blob storage (S3 or a persistent volume) configured, or uploaded mappings disappear when the container is recreated.

## Verify

1. Add a button that throws (`throw RuntimeException("Test error from Traceway")`), tap it, open the app again, and confirm the error appears on the Issues page with its full stack trace, the breadcrumbs before it and, with `screenCapture = true`, the replay. The Sessions page lists the session with **Android SDK** and its version.
2. For a minified release build: trigger a crash, then after `uploadReleaseTracewaySymbols` has run for that build, confirm the stack trace on Issues shows real class, method, and `file:line` rather than obfuscated `a.a.a(SourceFile)` frames.

For the symbolicator internals (how a trace is recognized, the retrace, and the `.tw` cache), see https://docs.tracewayapp.com/symbolicator/android.
