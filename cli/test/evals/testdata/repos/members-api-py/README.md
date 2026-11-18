# members-api

Backend of the members app: accounts, the account page, contacts, imports,
reports and recommendations. Python 3.12, FastAPI, SQLAlchemy Core on
PostgreSQL. Traces, logs and metrics go to Traceway over OTLP through the
OpenTelemetry Python distro (`opentelemetry-instrument uvicorn app.main:app`,
with `OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true` for the logs).

```
python -m unittest discover -s tests
OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true opentelemetry-instrument uvicorn app.main:app
```
