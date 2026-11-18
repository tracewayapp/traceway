# billing-api

Invoices for the storefront: the invoice page, customers' invoice lists and
invoice e-mails (through the mailer API). Go, `net/http`, PostgreSQL.
Telemetry goes to Traceway over OTLP; the production binary is built with
`-tags otel` (see `internal/telemetry`).

```
go test ./...
go run ./cmd/server
```

Releases are tagged in the commit message (`v1.8.2: ...`) and deployed from `main`.
