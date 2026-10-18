# accounts-api

Users, organizations and invites for Teamspace. Python 3.12, FastAPI,
SQLAlchemy 2.0 on PostgreSQL (asyncpg), Alembic migrations. Traces, logs and
metrics go to Traceway over OTLP through the OpenTelemetry auto-instrumentation
(`opentelemetry-instrument uvicorn app.main:app`).

```
pip install -r requirements.txt
alembic upgrade head
opentelemetry-instrument uvicorn app.main:app --port 8080
```

Routes:

- `GET /api/me`, `PATCH /api/me` – the signed-in user
- `GET /api/users/lookup?email=` – find a user by email (invite dialog, admin search)
- `GET /api/orgs/{org_id}/members` – members of an organization
- `GET /api/invites`, `POST /api/invites`, `POST /api/invites/{token}/accept`
