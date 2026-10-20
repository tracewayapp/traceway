# storefront-api

The API behind the storefront web app: customer, catalog and cart. Node 24,
NestJS 11 on Express, PostgreSQL through `pg`, Redis through `ioredis`.
Traces, logs and metrics go to Traceway over OTLP (`src/tracing.ts`, with
`@opentelemetry/auto-instrumentations-node`).

```
npm install
npm run build && npm start
```

The storefront edge validates the session cookie and forwards the signed-in
customer to the API as `x-customer-id`.

Feature flags live in the `feature_flags` table and are edited from the
back office; changes should reach the API within a minute.
