#!/bin/bash
set -e

MAX_RETRIES=30
RETRY_INTERVAL=2

echo "Checking ClickHouse availability..."

for i in $(seq 1 $MAX_RETRIES); do
    if /usr/bin/clickhouse-client --host localhost --port 9000 --query "SELECT 1" > /dev/null 2>&1; then
        echo "ClickHouse is ready!"

        # Create database if it doesn't exist
        echo "Creating traceway database if not exists..."
        /usr/bin/clickhouse-client --host localhost --port 9000 --query "CREATE DATABASE IF NOT EXISTS traceway"
        echo "Database ready!"

        exit 0
    fi
    echo "Attempt $i/$MAX_RETRIES: ClickHouse not ready yet, waiting ${RETRY_INTERVAL}s..."
    sleep $RETRY_INTERVAL
done

echo "ERROR: ClickHouse failed to become ready after $MAX_RETRIES attempts"
exit 1
