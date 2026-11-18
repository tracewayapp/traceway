# orders-api

Order lookup and checkout API behind the storefront. Node + Express 5, ES modules.

```
npm install
npm test
PORT=3000 npm start
curl localhost:3000/api/orders/1001
```

Production runs the container from `Dockerfile` (`npm start`) on our VM; the
environment comes from the deployment's env file.
