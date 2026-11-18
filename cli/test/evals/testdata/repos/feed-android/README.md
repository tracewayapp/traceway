# Pinboard for Android

Kotlin, AndroidX, RecyclerView, Retrofit + Gson, Coil. Crashes go to
Traceway through `com.tracewayapp:traceway`; release builds are minified with
R8 and the `com.tracewayapp.symbols` plugin uploads `mapping.txt`, so traces
arrive deobfuscated.

The home feed (`feed/`) pages through `GET /v2/feed` and binds each post in
`FeedAdapter`.

Releases are tagged in the commit message (`5.8.0: ...`) and rolled out in
stages from the Play Console.
