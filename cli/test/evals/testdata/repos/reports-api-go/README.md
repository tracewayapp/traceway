# reports-api

Saved reports and their exports. An export loads the report's rows,
renders them (csv, xlsx or json), uploads the file to object storage and
answers a signed download URL. Go, `net/http`, PostgreSQL. Telemetry goes to
Traceway over OTLP; the production binary is built with `-tags otel` (see
`internal/telemetry`), and `log/slog` is bridged to OTel there, so slog
records written with a request's context carry its trace id.

```
go test ./...
go run ./cmd/server
```
