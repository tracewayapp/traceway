# inventory-api

Stock levels and cart reservations for the fulfillment platform. Go,
`net/http`, PostgreSQL through pgx v5 (`pgxpool`, traced with `otelpgx`).
Traces, logs and metrics go to Traceway over OTLP.

```
go test ./...
go run -tags pgx ./cmd/server
```

Routes:

- `POST /api/reservations` – reserve stock for every line of a cart (15 minute hold)
- `POST /api/reservations/{id}/release` – release a hold
- `GET /api/inventory/{sku}`, `GET /api/warehouses` (served by the stock package, not shown)
