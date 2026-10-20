# catalog-api

Product pages: product details, prices and "You may also like"
recommendations (served by the recommendations service). Python 3.11+,
FastAPI, PostgreSQL. Telemetry goes to Traceway through the OpenTelemetry
FastAPI instrumentation and the logging handler.

Settings live in `config/settings.toml` (deployed with the code) and are
validated at startup by `app/config.py`.

```
python3 -m unittest
OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true opentelemetry-instrument uvicorn app.main:app
```

Releases are tagged in the commit message (`v3.4.0: ...`) and deployed from `main`.
