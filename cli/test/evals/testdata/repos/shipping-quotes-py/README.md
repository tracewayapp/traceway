# quotes-api

Shipping quotes for checkout (FastAPI). `POST /api/shipping/quotes` loads the
parcel, asks the carrier's rate API for the available services and stores
the quote checkout shows.

Run with the OpenTelemetry distro so traces and logs reach Traceway:

```
OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true opentelemetry-instrument uvicorn app.main:app --host 0.0.0.0 --port 8000
python -m unittest
```
