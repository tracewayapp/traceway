# Android SDK Reference

Native Android apps (Kotlin or Java) use the Traceway Android SDK, `com.tracewayapp:traceway`, published on Maven Central. It reports **errors and crashes only**: there is no session or video replay (unlike Flutter). It speaks the same `/api/report` wire format as the iOS, Flutter, and JS SDKs, so the backend ingests it with no server changes.

For a cross-platform stack with no native code (React Native, or a team standardized on OpenTelemetry) use that platform's path instead; this file is for apps with a real Android (Kotlin/Java) module.

## Install

Add the dependency to the app module's `build.gradle.kts`:

```kotlin
dependencies {
    implementation("com.tracewayapp:traceway:1.0.1")
}
```

Maven Central is enabled by default in modern Android projects, so no extra `repositories { ... }` entry is needed. Minimum SDK is API 21.

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
            connectionString = "<project-token>@https://<instance>/api/report",
            options = TracewayOptions(version = "1.0.0"),
        )
    }
}
```

Register the `Application` class and add the `INTERNET` permission in `AndroidManifest.xml`, or reports cannot leave the device:

```xml
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-permission android:name="android.permission.INTERNET" />
    <application android:name=".MyApp" ...>
        ...
    </application>
</manifest>
```

Wire the token through a build setting or `BuildConfig` field, not a committed literal. The connection string format is `<project-token>@https://<instance>/api/report`, identical to the other mobile SDKs.

## Manual capture

```kotlin
try {
    riskyOperation()
} catch (e: Throwable) {
    Traceway.captureException(e)
}

Traceway.captureMessage("User completed onboarding")
Traceway.flush()   // force-send pending events
```

## What gets captured automatically

After `init`, with no extra wiring:

- **Uncaught Java/Kotlin exceptions** on every thread, via `Thread.setDefaultUncaughtExceptionHandler`.
- **View click handler throws**, **background thread throws**, and **main `Handler.post` throws**.
- **Activity lifecycle transitions** recorded as navigation actions.

The last ~10 seconds of logs and actions ride along with each captured exception. `println` / `System.out` / `System.err` are mirrored automatically; `android.util.Log` lines are not, so install a Timber tree that calls `TracewayClient.instance?.recordLog(...)` if you need Logcat. Full option list (`sampleRate`, `debug`, channel toggles, on-disk persistence) is at https://docs.tracewayapp.com/client/android.

## Symbolication: upload R8 `mapping.txt` with the Gradle plugin

A release build with `minifyEnabled true` runs R8, which renames classes and methods and rewrites the line-number table, so production crash traces arrive obfuscated and stay unreadable until Traceway retraces them server-side against the build's `mapping.txt` (the Android equivalent of a JavaScript source map). Set this up whenever the app ships a minified release.

**Token.** Symbol uploads authenticate with the dedicated **upload token** (Connection page > Source Maps / Symbol Upload), NOT the project token from the connection string. Get it from Step 1; it is a CI secret, never committed. `readonly` members cannot generate one. Using the connection-string token here is rejected with a 401.

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

**Wire the UUID into the SDK.** The plugin embeds a ProGuard UUID into `BuildConfig.TRACEWAY_PROGUARD_UUID` and names the uploaded mapping `<uuid>.txt`. The UUID is derived from the module path, variant, and app version (`versionName` + `versionCode`), so **bump the version each release** or a new upload overwrites the previous one (or set `proguardUuid` explicitly). Pass the injected value to the SDK so each reported crash carries the matching UUID:

```kotlin
Traceway.init(
    application = this,
    connectionString = "<project-token>@https://<instance>/api/report",
    options = TracewayOptions(
        version = "1.0.0",
        proguardUuid = BuildConfig.TRACEWAY_PROGUARD_UUID,
    ),
)
```

**Per release.** With `autoUpload = false` (the better fit for CI, so a release build never depends on backend availability) upload explicitly after the build:

```bash
./gradlew assembleRelease uploadReleaseTracewaySymbols
```

With `autoUpload = true` the upload runs automatically after `assembleRelease`. Keep the upload token in a CI secret — the `System.getenv` wiring in the `traceway { ... }` block above is what feeds it to the plugin (there is no built-in environment lookup):

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

1. Add a button that throws (`throw RuntimeException("Test error from Traceway")`), tap it, and confirm the error appears on the Issues page within a few seconds with its full stack trace.
2. For a minified release build: trigger a crash, then after `uploadReleaseTracewaySymbols` has run for that build, confirm the stack trace on Issues shows real class, method, and `file:line` rather than obfuscated `a.a.a(SourceFile)` frames.

For the symbolicator internals (how a trace is recognized, the retrace, and the `.tw` cache), see https://docs.tracewayapp.com/symbolicator/android.
