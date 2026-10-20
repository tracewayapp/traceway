# catalog-api

The shopfront's product catalog: category grids, product pages, search and
prices. Node 24, Express 5, PostgreSQL through `pg`. Telemetry goes to
Traceway over OTLP with `@opentelemetry/auto-instrumentations-node`
(`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME=catalog-api`).

```
npm install
npm start
```

Migrations live in `migrations/` and are applied in order with `psql -f`.
