# checkout-api

Cart and checkout for the marketplace. Python 3.12, FastAPI, SQLAlchemy 2.0
(sync sessions, psycopg 3) on PostgreSQL, run with uvicorn (one process per
container). Payments go to the provider's REST API with httpx. Telemetry goes
to Traceway over OTLP through the OpenTelemetry auto-instrumentation.

```
pip install -r requirements.txt
opentelemetry-instrument uvicorn app.main:app --port 8080
```
