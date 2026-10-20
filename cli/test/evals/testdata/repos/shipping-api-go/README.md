# shipping-api

Creates shipments for paid orders: validates the destination, rates the
parcel against the carrier rate tables and books it. Go, `net/http`,
PostgreSQL. Telemetry goes to Traceway over OTLP; the production binary is
built with `-tags otel` (see `internal/telemetry`).

```
go test ./...
go run ./cmd/server
```

Releases are tagged in the commit message (`v3.8.0: ...`) and deployed from `main`.
