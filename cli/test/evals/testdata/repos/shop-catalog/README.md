# shop

The shop's catalog API and web storefront.

- `api/` - catalog-api (Express, PostgreSQL, Redis). `GET /api/catalog`
  returns every active product with its category and price for a locale
  (~1200 products). Merchandising publishes catalog changes twice a day.
- `web/` - the storefront SPA (React, Vite) at shop.example.com, reporting
  errors and sessions to Traceway with `@tracewayapp/react`.

```
cd api && npm test
cd web && npm run dev
```
