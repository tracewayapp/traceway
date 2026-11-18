# storefront

- `api/`: the storefront API (Node.js, Express, PostgreSQL), served at
  `https://shop.example.com/api`. It is also the API of the iOS and Android
  apps, so its JSON contract is stable: field names are snake_case
  (`created_at`, `item_count`, `total_cents`) and are never renamed in place;
  a breaking change ships as a new versioned route.
- `web/`: the React storefront (Vite). It reports errors to Traceway with the
  browser SDK; source maps are uploaded on deploy, so stack traces point at
  `web/src`.

Both deploy from `main`. The pure modules (`api/src/orders`, `web/src/lib`)
are tested with Node's built-in runner, no install needed:

```
node --test
```
