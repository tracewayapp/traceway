# shop

The web shop: `web/` is the React storefront (Vite, `@tracewayapp/react`,
Traceway project **shop-web**); `api/` is its backend (Node 24, Express,
Traceway project **shop-api**; traces and logs go to Traceway over OTLP,
`api/src/otel.js`). Product search goes through the API to the hosted search
index.

```
cd api && npm test
cd web && npm run build
```
