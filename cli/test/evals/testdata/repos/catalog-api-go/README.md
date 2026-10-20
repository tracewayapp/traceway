# catalog-api

Product catalog for the storefront: category listings and product pages.
Go, `net/http`, PostgreSQL through pgx. Runs as three instances (catalog-api-a/b/c)
behind the load balancer; a release restarts all three. Traces, logs and
metrics go to Traceway over OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`); production
builds (`go build -tags otel`) install otelhttp and otelpgx through
`internal/telemetry`.

```
go test ./...
go run ./cmd/server
```

Routes:

- `GET /api/categories/{slug}/products` – the category listing, served from an
  in-process cache that is preloaded at startup and refreshed every 10 minutes
- `GET /api/products/{id}` – one product
- `GET /healthz` – the load balancer's health check
