# payments-webhooks

Receives the payment provider's webhooks (`POST /api/webhooks/payments`),
verifies their signature and applies them: paid invoices are recorded as
payments and linked to the customer's account, subscriptions are activated
or cancelled. Python 3.11+, FastAPI, PostgreSQL. Telemetry goes to Traceway
through the OpenTelemetry FastAPI instrumentation and the logging handler.

The event handling in `app/webhooks.py` has no framework dependency, so the
tests run with the standard library only:

```
python3 -m unittest
OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true opentelemetry-instrument uvicorn app.main:app
```

Releases are tagged in the commit message (`v2.6.0: ...`) and deployed from `main`.
