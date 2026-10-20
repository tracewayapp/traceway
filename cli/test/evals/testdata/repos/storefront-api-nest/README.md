# storefront-api

The API behind the storefront web app: customer, catalog and cart. Node 24,
NestJS 11 on Express, PostgreSQL through `pg`, Redis through `ioredis`.
Telemetry goes to Traceway over OTLP with
`@opentelemetry/auto-instrumentations-node`.

```
npm install
npm run build && npm start
```

Feature flags live in the `feature_flags` table and are edited from the
back office; changes should reach the API within a minute.
