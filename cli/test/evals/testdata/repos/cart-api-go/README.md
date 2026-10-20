# cart-api

The cart page's backend: cart contents, and the shipping and tax quote,
which comes from the pricing service. Go, `net/http`, PostgreSQL.
Telemetry goes to Traceway over OTLP; the production binary is built with
`-tags otel` (see `internal/telemetry`).

Configuration is read from the environment (`internal/config`).

```
go test ./...
go run ./cmd/server
```

Releases are tagged in the commit message (`v1.14.0: ...`) and deployed from `main`.
