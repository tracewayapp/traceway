# fulfillment

The label worker: it consumes paid orders from the `fulfillment.orders` topic
(through the queue gateway's HTTP API) and buys shipping labels from the
carrier. Go, PostgreSQL. Telemetry goes to Traceway over OTLP; the production
binary is built with `-tags otel` (see `internal/telemetry`).

```
go test ./...
go run ./cmd/worker
```

Releases are tagged in the commit message (`v2.4.0: ...`) and deployed from `main`.
