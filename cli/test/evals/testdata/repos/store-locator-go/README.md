# store-locator-api

Store finder for the shop (Go, `net/http`).
`GET /api/stores/nearby?q=<postcode or address>` geocodes the query with the
maps provider, loads the closest stores from PostGIS and asks the provider for
driving distances.

- `internal/maps`: maps provider client (geocode, distance matrix)
- `internal/inventory`: client for the internal inventory service
- `internal/stores`: HTTP handlers

Outgoing HTTP is traced with otelhttp plus otelhttptrace (connection, DNS,
TLS timings), installed through `maps.Instrument` in production builds.

```
go test ./...
go run ./cmd/server
```
