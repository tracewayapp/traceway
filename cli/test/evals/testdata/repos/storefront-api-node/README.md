# storefront-api

The web shop's API: product search, product pages, cart and checkout.
Node 24, Express 5, PostgreSQL through `pg`. Traces, logs and metrics go to
Traceway over OTLP via the OpenTelemetry Node SDK (`src/otel.js`).

```
npm test
npm start
```

The products table holds about 25k active products.
