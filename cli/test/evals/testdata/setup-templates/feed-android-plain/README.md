# Pinboard for Android

Kotlin, AndroidX, RecyclerView, Retrofit + Gson, Coil. The home feed
(`feed/`) pages through `GET /v2/feed` and binds each post in `FeedAdapter`.

`minSdk` is 24 (Android 7.0): a product requirement, we still have users
there.

```
gradle :app:assembleDebug
gradle :app:testDebugUnitTest
```

Release builds are minified with R8 and built by
`.github/workflows/release.yml` on every `v*` tag, then rolled out in stages
from the Play Console. Build configuration that differs per machine or holds
a secret is read from the environment or the (gitignored) `local.properties`,
never committed.
