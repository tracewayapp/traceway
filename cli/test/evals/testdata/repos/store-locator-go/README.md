# store-locator-api

Store finder for the shop (Go, `net/http`).
`GET /api/stores/nearby?q=<postcode or address>` geocodes the query with the
maps provider, loads the closest stores from PostGIS and asks the provider for
driving distances.

`GET /api/stores/{id}/availability` reads the store's stock from the
inventory service and `GET /api/stores/{id}/hours` its opening hours.

- `internal/maps`: maps provider client (geocode, distance matrix)
- `internal/inventory`: client for the internal inventory service
- `internal/stores`: HTTP handlers and the store directory (PostGIS)
- `internal/telemetry`: OpenTelemetry wiring

Traces and logs go to Traceway over OTLP. Production builds
(`go build -tags otel`) install otelhttp on the server and on outgoing
transports, with otelhttptrace for connection, DNS and TLS timings, and
otelpgx on the database pool.

```
go test ./...
go run ./cmd/server
```
