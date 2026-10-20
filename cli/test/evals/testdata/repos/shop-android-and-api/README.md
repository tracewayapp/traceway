# shop (Android app + checkout API)

- `android/`: the Android shopping app (Kotlin, Retrofit + OkHttp,
  coroutines). Reports crashes to Traceway with the Android SDK (project
  **shop-android**), with the network calls `TracewayOkHttpInterceptor`
  records.
- `backend/`: the checkout API (Java 21, Spring Boot 3, JPA and JdbcTemplate on
  PostgreSQL, HikariCP). Traced with the OpenTelemetry Java agent (project
  **commerce-api**); logs go through the agent's Logback appender with
  `otel.instrumentation.logback-appender.experimental.capture-key-value-pair-attributes=true`,
  so the access log (`web/AccessLogFilter.java`) carries its fields.
