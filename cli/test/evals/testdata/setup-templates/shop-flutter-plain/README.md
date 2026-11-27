# Shop (Flutter)

The storefront app for iOS and Android. Flutter 3.38, Dart 3.10.

Product data comes from the catalog API (`lib/core/api_client.dart`): the
home screen lists `GET /v1/products` and every product page loads
`GET /v1/products/{id}`.

```
flutter pub get
flutter run
flutter test
```

Store builds are made by `.github/workflows/release.yml` on every `v*` tag:
an app bundle for Google Play and an iOS build for TestFlight. Build-time
configuration is passed with `--dart-define` (read with
`String.fromEnvironment`); secrets come from the repository's GitHub Actions
secrets and never live in the repo.
