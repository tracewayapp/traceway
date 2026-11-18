# hello-api

A small greeting API used by the marketing site. Go, standard library only.

```
PORT=8080 go run .
curl localhost:8080/api/hello
curl localhost:8080/api/hello/ada
```

Deployed as a single container (see `Dockerfile`).
