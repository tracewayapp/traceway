# catalog

The product catalog site and its JSON API (`/api/products/:id`, used by the
mobile app and partners). Next.js App Router.

```
npm install
npm run dev            # http://localhost:3000
npm run build && npm start
```

Production builds the image from `Dockerfile` and runs `npm start` behind our
load balancer; the environment comes from the deployment's env file.
