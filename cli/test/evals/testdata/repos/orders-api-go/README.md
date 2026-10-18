# orders-api

The storefront's order service: order history and order detail for signed-in
customers. Go, `net/http`, PostgreSQL through `database/sql`. Traces, logs and
metrics go to Traceway over OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`).

```
go test ./...
go run ./cmd/server
```

Routes:

- `GET /api/orders` – the signed-in customer's orders, newest first, with their line items
- `GET /api/orders/{id}` – one order with its line items
