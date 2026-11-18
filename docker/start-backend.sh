#!/bin/bash
set -e

# The backend loads .env without overwriting Docker's environment. Sourcing it
# here would replace operator-provided secrets before the backend sees them.
clickhouse_server=${CLICKHOUSE_SERVER:-localhost:9000}
postgres_host=${POSTGRES_HOST:-localhost}

if [[ "$clickhouse_server" == localhost* ]] || [[ "$clickhouse_server" == 127.0.0.1* ]]; then
    echo "Waiting for ClickHouse to be ready..."
    /usr/local/bin/wait-for-clickhouse.sh
fi

if [[ "$postgres_host" == "localhost" ]] || [[ "$postgres_host" == "127.0.0.1" ]]; then
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
