# feed-api

The home feed of the social app (Go, `net/http`, PostgreSQL). Serves the web
app, the iOS app and the Android app.

- `GET /api/feed` - the latest 50 cards for the signed-in member
- `GET /api/feed/cards/{id}` - one card
- `GET /api/authors/{id}` - an author's profile

Card images are stored with the card (`feed_cards.image_data`, a data URI
written by the importer) and also uploaded to the media CDN under
`cards/<id>.jpg` (`CDN_BASE_URL`).

Traces go to Traceway over OTLP (otelhttp, installed by `withTracing`).

```
go test ./...
go run ./cmd/server
```
