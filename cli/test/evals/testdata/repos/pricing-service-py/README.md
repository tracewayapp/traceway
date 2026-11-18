# pricing-service

Prices carts for the storefront and the wholesale portal: unit prices,
promotions (volume and bundle deals) and tax. FastAPI on Python 3.12,
PostgreSQL through psycopg 3. Traces, logs and metrics go to Traceway over
OTLP via `opentelemetry-instrument`.

```
python3 -m unittest discover -s tests
OTEL_SERVICE_NAME=pricing-service OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true \
  opentelemetry-instrument uvicorn app.main:app --port 8000
```

Routes:

- `POST /api/quotes` – price a cart `{"items": [{"sku": ..., "quantity": ...}]}`
- `GET /api/price-lists/{id}` – a price list
