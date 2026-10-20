# admin-api

The Ledgerly admin console backend: members, settings, invoices and the audit
log. Java 21, Spring Boot 3.5, JDBC (`JdbcTemplate`) on PostgreSQL, Flyway
migrations. Instrumented with the OpenTelemetry Java agent (see `Dockerfile`:
`OTEL_SERVICE_NAME=admin-api`, controller spans and the access log's
key-value attributes turned on). The console gateway validates the session
and forwards the user and organization as `X-Ledgerly-User-Id` and
`X-Ledgerly-Org-Id`.

```
./mvnw spring-boot:run
```

Routes:

- `GET /api/admin/audit-log` – the Audit log page: the organization's events, newest first
- `GET /api/admin/dashboard/activity` – the dashboard widget: the last 20 events
- `GET /api/admin/members` – members of the organization
- `GET /api/admin/settings`, `PUT /api/admin/settings` – organization settings (updates are audited)
- `GET /api/admin/invoices` – the latest 50 invoices
