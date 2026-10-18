# statements-api

Account statements for the billing dashboard: a customer picks a period and
gets a PDF of every invoice issued in it. Go, `net/http`, PostgreSQL through
`pgx`, PDFs stored in S3. Traces, logs and metrics go to Traceway over OTLP
(`OTEL_EXPORTER_OTLP_ENDPOINT`).

```
go test ./...
go run ./cmd/server
```

Routes:

- `POST /api/statements` – create a statement for `{accountId, periodStart, periodEnd}`
- `GET /api/statements/{id}` – a statement's status and object key
- `GET /api/statements/{id}/download` – redirect to a presigned S3 URL
- `GET /api/invoices` – the account's latest invoices

The dashboard client is in `web/statements.js`.
