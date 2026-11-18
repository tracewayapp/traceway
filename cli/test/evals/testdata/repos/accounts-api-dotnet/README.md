# accounts-api

Accounts and transactions for the finance dashboard. ASP.NET Core 9 minimal
APIs, PostgreSQL through Npgsql + Dapper. Traces, logs and metrics go to
Traceway over OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`); the OpenTelemetry setup is
in `Program.cs`.

```
dotnet test
dotnet run --project src/AccountsApi
```

Routes:

- `GET /api/accounts/{accountId}` – one account with its balance
- `GET /api/accounts/{accountId}/transactions` – a page of transactions, newest first
- `GET /api/accounts/{accountId}/transactions/export?from=&to=` – every transaction in a period as JSON (finance downloads these for reconciliation)
