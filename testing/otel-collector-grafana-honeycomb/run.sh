#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

source .env

export GRAFANA_OTLP_BASIC="$(printf '%s:%s' "$GRAFANA_OTLP_INSTANCE_ID" "$GRAFANA_CLOUD_TOKEN" | base64)"

if [ ! -x build/otelcol-honeycomb-symbolicator ]; then
  echo "build/otelcol-honeycomb-symbolicator not found, run ./bin/builder --config manifest.yaml first" >&2
  exit 1
fi

exec ./build/otelcol-honeycomb-symbolicator --config config.yaml
