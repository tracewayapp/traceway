# storefront-gateway

The storefront's API gateway (Express). It owns the session and composes the
pages from the services behind it:

| Service | Project | Used for |
|---|---|---|
| cart-service | storefront | cart lines |
| catalog-service | catalog | product title, brand, image |
| pricing-service | catalog | current price per currency |
| inventory-service | fulfillment | stock per warehouse region |

Clients live in `src/clients`, routes in `src/routes`. Traces and logs go to
Traceway over OTLP (`src/tracing.js`).

```
npm test
npm start
```
