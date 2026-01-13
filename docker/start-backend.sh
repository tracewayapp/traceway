#!/bin/bash
set -e

if [[ "$CLICKHOUSE_SERVER" == localhost* ]]; then
    echo "Waiting for ClickHouse to be ready..."
    /usr/local/bin/wait-for-clickhouse.sh
fi

echo "Starting Traceway backend..."
exec /usr/local/bin/traceway-backend
