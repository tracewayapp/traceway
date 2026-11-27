# Shop for iOS

SwiftUI (iOS 17+), Xcode 26.4. One app target, `Shop`, whose sources are the
`Shop/` folder (Xcode picks up new files there on its own).

Build settings live in `Config/Shop.xcconfig`, and the app reads them through
`Shop/Info.plist` (`APIBaseURL` is `$(API_BASE_URL)`, read in
`Core/APIClient.swift`). Keys and tokens go in `Config/Secrets.xcconfig`,
which `Shop.xcconfig` includes: it is gitignored, so create it locally, and
the release workflow writes it from the repository's secrets before it
archives. In an `.xcconfig` file `//` starts a comment, so a URL is written
`https:/$()/host`.

Releases are archived by `.github/workflows/release.yml` on every `v*` tag
and uploaded to TestFlight.
