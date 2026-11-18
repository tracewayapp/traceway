# invoicing-api

Invoices for the billing product: create, list and show invoices, and the
monthly invoice export accounting downloads at month end. Go, `net/http`,
PostgreSQL through `database/sql`. Traces, logs and metrics go to Traceway
over OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`); production builds
(`go build -tags otel`) install otelhttp and otelsql through
`internal/telemetry`.

```
go test ./...
go run ./cmd/server
```

Routes:

- `GET /api/invoices`, `GET /api/invoices/{id}`, `POST /api/invoices`
- `GET /api/exports/invoices?month=2026-09` – every invoice of the month with
  its lines and tax breakdown as CSV
