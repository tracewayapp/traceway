# inventory-api

Stock levels and cart reservations for the fulfillment platform. Go,
`net/http`, PostgreSQL through pgx v5 (`pgxpool`, traced with `otelpgx`).
Traces and logs go to Traceway over OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`,
`OTEL_SERVICE_NAME=inventory-api`); logs are written with `log/slog` through
the OpenTelemetry slog bridge.

```
go test ./...
go run ./cmd/server
```

Routes:

- `POST /api/reservations` – reserve stock for every line of a cart (15 minute hold)
- `POST /api/reservations/{id}/release` – release a hold
- `GET /api/inventory/{sku}` – stock of a SKU per warehouse
- `GET /api/warehouses` – the warehouses
