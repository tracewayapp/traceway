# Shop for iOS

SwiftUI (iOS 17+), with a couple of UIKit screens. Crashes go to Traceway
through the `traceway-ios` package; the release build phase uploads the
dSYMs, so traces arrive symbolicated.

Remote configuration (`Config/RemoteConfig.swift`) is fetched at launch from
the config service; values are set per audience (market and app version) in
the config console, and a key that is not set for an audience is absent.

Releases are tagged in the commit message (`3.12.0: ...`).
