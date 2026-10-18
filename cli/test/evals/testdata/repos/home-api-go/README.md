# home-api

Serves the storefront home feed (`GET /api/home`): banners, collections,
top sellers, new arrivals and the other home sections, assembled from
PostgreSQL and cached in Redis as one JSON document (`home:feed:v3`).

- `internal/home/service.go` - cache lookup and rebuild
- `internal/home/build.go` - the section queries
- `internal/home/handler.go` - HTTP handler

Redis is go-redis with redisotel, PostgreSQL is pgx with otelpgx; traces go
to Traceway over OTLP.

```
go test ./...
go run ./cmd/server
```
