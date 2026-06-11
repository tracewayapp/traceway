#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

VERSION="${OTELCOL_VERSION:-0.154.0}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi

mkdir -p bin
curl -sL -o bin/otelcol-contrib.tar.gz \
  "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${VERSION}/otelcol-contrib_${VERSION}_${OS}_${ARCH}.tar.gz"
tar -xzf bin/otelcol-contrib.tar.gz -C bin otelcol-contrib
rm bin/otelcol-contrib.tar.gz
./bin/otelcol-contrib --version
