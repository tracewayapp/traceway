# checkout-api

Checkout and refunds for the shop (Go, `net/http`, PostgreSQL via pgx).
`POST /api/checkout` charges the order through the payment provider
(api.payments.example) and marks it paid; `POST /api/refunds` refunds a
charge; `GET /api/orders/{id}` reads an order.

- `internal/checkout` - handlers
- `internal/payments` - payment provider client

Traces, logs and Go runtime metrics go to Traceway over OTLP (otelhttp,
otelpgx, the contrib runtime instrumentation).

```
go test ./...
go run ./cmd/server
```
