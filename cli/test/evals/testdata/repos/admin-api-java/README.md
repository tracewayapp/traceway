# admin-api

The Ledgerly admin console backend: members, settings, invoices and the audit
log. Java 21, Spring Boot 3.5, JDBC (`JdbcTemplate`) on PostgreSQL, Flyway
migrations. Instrumented with the OpenTelemetry Java agent
(`-javaagent:opentelemetry-javaagent.jar`, `OTEL_SERVICE_NAME=admin-api`).

```
./mvnw spring-boot:run
```

Audit log routes:

- `GET /api/admin/audit-log` – the Audit log page: the organization's events, newest first
- `GET /api/admin/dashboard/activity` – the dashboard widget: the last 20 events
