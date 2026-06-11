#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

export TW_PROJECT_TOKEN="${TW_PROJECT_TOKEN:-5638c2de607f45169bcf98aa8774fe5c}"

mkdir -p captures

if [ ! -x bin/otelcol-contrib ]; then
  echo "bin/otelcol-contrib not found, run ./download.sh first" >&2
  exit 1
fi

exec ./bin/otelcol-contrib --config config.yaml
