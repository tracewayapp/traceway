# home-api

Serves the storefront home feed (`GET /api/home`): banners, collections,
top sellers, new arrivals and the other home sections, assembled from
PostgreSQL and cached in Redis as one JSON document (`home:feed:v3`).

- `internal/home/service.go` - cache lookup and rebuild
- `internal/home/build.go` - the section queries
- `internal/home/handler.go` - HTTP handler
- `internal/home/stories.go` - `GET /api/home/stories/{id}`, the seasonal stories

Redis is go-redis with redisotel (`cmd/server/redis_otel.go`), PostgreSQL is
`database/sql` with the pgx driver and otelsql. Traces, logs and the
`cache.hit_ratio` gauge go to Traceway over OTLP; the production binary is
built with `-tags otel` (see `internal/telemetry`).

```
go test ./...
go run ./cmd/server
```
