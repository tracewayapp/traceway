# insights-api

Product analytics for the insights dashboard: retention cohorts, funnels and
saved dashboards over the `events` table. Spring Boot 3.5 on Java 21,
PostgreSQL through `JdbcTemplate`. The OpenTelemetry Java agent ships traces
and the JVM runtime metrics to Traceway (see `Dockerfile`).

```
./gradlew test
./gradlew bootRun
```

Routes:

- `GET /api/insights/retention?workspace=&from=&to=&interval=week` – weekly retention cohorts
- `GET /api/insights/funnels?workspace=&funnel=` – funnel conversion per step
- `GET /api/insights/dashboards/{id}` – a saved dashboard
