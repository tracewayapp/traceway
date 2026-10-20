# listings-api

Marketplace listings: category search, listing pages and view counts. Go,
`net/http`, PostgreSQL through pgx, Redis for the search cache. Traces and
logs go to Traceway over OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`); logs are
written with `log/slog` through the OpenTelemetry slog bridge.

Configuration comes from the environment; production values live in
`deploy/production.env`, which the deploy pipeline turns into the pods'
ConfigMap. A change to it rolls the deployment like any release.

```
go run ./cmd/server
```

Routes:

- `GET /api/listings?category=` – active listings of a category, ranked
- `GET /api/listings/{id}` – one listing with its images
- `POST /api/listings/{id}/views` – count a view
- `GET /api/categories` – the category tree (cached)
