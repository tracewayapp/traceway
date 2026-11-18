# statements-api

Account statements for the billing dashboard: a customer picks a period and
gets a PDF of every invoice issued in it. Go, `net/http`, PostgreSQL through
`database/sql` and the `pgx` driver, PDFs stored in S3. Traces and logs go to
Traceway over OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`); the production binary is
built with `-tags otel` (see `internal/telemetry`). Host metrics come from the
OTel Collector's hostmetrics receiver.

```
go test ./...
go run ./cmd/server
```

Routes:

- `POST /api/statements` – create a statement for `{accountId, periodStart, periodEnd}`
- `GET /api/statements/{id}` – a statement's status and object key
- `GET /api/statements/{id}/download` – redirect to a presigned S3 URL
- `GET /api/invoices?accountId=` – the account's latest invoices

The dashboard client is in `web/statements.js`.
