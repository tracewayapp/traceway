# shop (iOS app + mobile API)

- `ios/`: the iOS shopping app (Swift, UIKit). Crashes go to Traceway with
  the `traceway-ios` SDK (project **shop-ios**); launch watchdog terminations
  are forwarded from MetricKit by `Diagnostics/CrashDiagnostics.swift`. dSYMs
  are uploaded on every release build.
- `api/`: the mobile API the app talks to (Go, `net/http`, PostgreSQL through
  `database/sql`, OpenTelemetry; project **mobile-api**).
