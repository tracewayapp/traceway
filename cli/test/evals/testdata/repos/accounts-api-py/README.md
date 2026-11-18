# accounts-api

Users, organizations and invites for Teamspace. Python 3.12, FastAPI,
SQLAlchemy 2.0 on PostgreSQL (asyncpg), Alembic migrations. Traces, logs and
metrics go to Traceway over OTLP through the OpenTelemetry auto-instrumentation
(`opentelemetry-instrument uvicorn app.main:app`, with
`OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true` so the access log in
`app/main.py` is exported too). Host metrics come from the OTel Collector's
hostmetrics receiver.

```
pip install -r requirements.txt
alembic upgrade head
OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true opentelemetry-instrument uvicorn app.main:app --port 8080
```

Routes:

- `GET /api/me`, `PATCH /api/me` – the signed-in user
- `GET /api/users/lookup?email=` – find a user by email (invite dialog, admin search)
- `GET /api/orgs/{org_id}/members` – members of an organization
- `GET /api/invites`, `POST /api/invites`, `POST /api/invites/{token}/accept`
