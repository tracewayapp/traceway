module github.com/acme/listings-api

go 1.26

require (
	github.com/exaring/otelpgx v0.9.3
	github.com/jackc/pgx/v5 v5.7.6
	github.com/redis/go-redis/extra/redisotel/v9 v9.14.0
	github.com/redis/go-redis/v9 v9.14.0
	go.opentelemetry.io/contrib/bridges/otelslog v0.13.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0
	go.opentelemetry.io/otel v1.38.0
)
