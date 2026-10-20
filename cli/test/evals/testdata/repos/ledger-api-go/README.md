# ledger-api

General ledger service: account balances, journal entries and the finance
team's quarterly ledger export. Go, `net/http`, PostgreSQL through pgx.
Traces, logs and metrics go to Traceway over OTLP
(`OTEL_EXPORTER_OTLP_ENDPOINT`).

```
go test ./...
go run ./cmd/server
```

Routes:

- `GET /api/accounts/{id}/balance` – current balance of one account
- `GET /api/accounts/{id}/entries` – the latest 50 entries of one account
- `POST /api/journal-entries` – post a journal entry
- `GET /api/reports/export?from=&to=` – every ledger entry of the organization
  in the range, as CSV (finance pulls one quarter at a time)
