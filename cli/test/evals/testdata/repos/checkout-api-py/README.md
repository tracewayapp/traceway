# checkout-api

Cart and checkout for the marketplace. Python 3.12, FastAPI, SQLAlchemy 2.0
(sync sessions, psycopg 3) on PostgreSQL, run with uvicorn (one process per
container). Payments go to the provider's REST API with httpx. Traces, logs
and the SQLAlchemy pool metrics go to Traceway over OTLP through the
OpenTelemetry auto-instrumentation, with Python logging exported
(`OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true`); host metrics come from
the OTel Collector's hostmetrics receiver.

```
pip install -r requirements.txt
OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true opentelemetry-instrument uvicorn app.main:app --port 8080
```
