# checkout-api

Checkout and refunds for the shop (Go, `net/http`, PostgreSQL via pgx).
`POST /api/checkout` charges the order through the payment provider
(api.payments.example) and marks it paid; `POST /api/refunds` refunds a
charge; `GET /api/orders/{id}` reads an order.

- `internal/checkout` - handlers and the orders store
- `internal/payments` - payment provider client
- `internal/telemetry` - OpenTelemetry wiring

Traces, logs and Go runtime metrics go to Traceway over OTLP; production
builds (`go build -tags otel`) install otelhttp, otelpgx and the contrib
runtime instrumentation.

```
go test ./...
go run ./cmd/server
```
