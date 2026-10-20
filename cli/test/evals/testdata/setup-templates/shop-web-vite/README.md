# shop-web

The storefront SPA. React 19 + Vite, deployed as static files to S3 by
`.github/workflows/deploy.yml` on every push to `main`.

```
npm install
npm run dev     # http://localhost:5173
npm run build   # dist/
```

Build-time configuration comes from `VITE_*` environment variables (see
`src/api.js`).
