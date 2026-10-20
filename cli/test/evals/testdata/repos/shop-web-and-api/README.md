# shop

The web shop: `web/` is the React storefront (Vite, `@tracewayapp/react`,
Traceway project **shop-web**); `api/` is its backend (Node 24, Express,
OpenTelemetry, Traceway project **shop-api**). Product search goes through
the API to the hosted search index.

```
cd api && npm test
cd web && npm run build
```
