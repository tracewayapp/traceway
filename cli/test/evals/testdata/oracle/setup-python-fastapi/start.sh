#!/bin/sh
set -e
# Traceway receives OTLP over HTTP only. The Python SDK resolves "otlp" to
# gRPC unless the protocol is set, and then exports nothing.
export OTEL_SERVICE_NAME="${OTEL_SERVICE_NAME:-inventory-api}"
export OTEL_RESOURCE_ATTRIBUTES="${OTEL_RESOURCE_ATTRIBUTES:-service.version=2.3.1}"
export OTEL_EXPORTER_OTLP_ENDPOINT="${TRACEWAY_URL%/}/api/otel"
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${TRACEWAY_PROJECT_TOKEN}"
export OTEL_TRACES_EXPORTER=otlp OTEL_METRICS_EXPORTER=otlp OTEL_LOGS_EXPORTER=otlp
export OTEL_PYTHON_LOG_CORRELATION=true OTEL_PYTHON_LOG_LEVEL=info
exec opentelemetry-instrument uvicorn main:app --host 0.0.0.0 --port "${PORT:-8000}"
