# catalog-api

The shopfront's product catalog: category grids, product pages, search and
prices. Node 24, Express 5, PostgreSQL through `pg`. Traces, logs and metrics
go to Traceway over OTLP (`src/tracing.js`, with
`@opentelemetry/auto-instrumentations-node`; `OTEL_EXPORTER_OTLP_ENDPOINT`).

```
npm install
npm start
```

Migrations live in `migrations/` and are applied in order with `psql -f`.
