#!/bin/bash
set -e

set -a
source /app/.env
set +a

if [[ "$CLICKHOUSE_SERVER" == localhost* ]] || [[ "$CLICKHOUSE_SERVER" == 127.0.0.1* ]]; then
    echo "Waiting for ClickHouse to be ready..."
    /usr/local/bin/wait-for-clickhouse.sh
fi

if [[ "$POSTGRES_HOST" == "localhost" ]] || [[ "$POSTGRES_HOST" == "127.0.0.1" ]]; then
    echo "Waiting for PostgreSQL to be ready..."
    MAX_RETRIES=30
    RETRY_INTERVAL=2
    for i in $(seq 1 $MAX_RETRIES); do
        if pg_isready -h 127.0.0.1 -p 5432 > /dev/null 2>&1; then
            echo "PostgreSQL is ready!"
            break
        fi
        if [ $i -eq $MAX_RETRIES ]; then
            echo "ERROR: PostgreSQL failed to become ready after $MAX_RETRIES attempts"
            exit 1
        fi
        echo "Attempt $i/$MAX_RETRIES: PostgreSQL not ready yet, waiting ${RETRY_INTERVAL}s..."
        sleep $RETRY_INTERVAL
    done
fi

echo "Starting Traceway backend..."
exec /usr/local/bin/traceway
