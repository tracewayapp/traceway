# Shop (Flutter)

The storefront app for iOS and Android. Flutter 3.35, Dart 3.9. Errors go to
Traceway through the `traceway` package (`Traceway.run` in `lib/main.dart`);
release builds are not obfuscated, so traces are readable as reported.

Product data comes from the catalog API (`lib/core/api_client.dart`): every
product page loads `GET /v1/products/{id}`.

Releases are tagged in the commit message (`4.3.0: ...`).
