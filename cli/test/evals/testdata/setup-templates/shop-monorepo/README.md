# shop

| Path | What | Deployed as |
|---|---|---|
| `api/` | Product and cart API, Go standard library | Docker container on our VM (`api/Dockerfile`), env from `api/.env` via `docker run --env-file` |
| `web/` | Storefront SPA, React + Vite | Static files on S3 behind CloudFront: `npm run build` in `web/`, then `aws s3 sync web/dist s3://shop-web` |

The storefront calls the API on the same origin (`/api/*` is routed to the
container by CloudFront; `npm run dev` proxies it to `localhost:8080`).

```
cd api && go test ./... && PORT=8080 go run .
cd web && npm install && npm run dev
```
