#!/usr/bin/env bash
set -euo pipefail

# --------------------------------------------------------------------------
# Start PostgreSQL
# --------------------------------------------------------------------------
pg_ctlcluster 15 main start || pg_ctlcluster 16 main start || pg_ctlcluster 17 main start

# Wait for PostgreSQL to be ready
for i in $(seq 1 30); do
    if su - postgres -c "pg_isready" >/dev/null 2>&1; then break; fi
    sleep 0.5
done

# Create test database and user
su - postgres -c "psql -c \"CREATE USER traceway WITH PASSWORD 'traceway';\"" 2>/dev/null || true
su - postgres -c "psql -c \"CREATE DATABASE traceway_test OWNER traceway;\"" 2>/dev/null || true

# --------------------------------------------------------------------------
# Start ClickHouse
# --------------------------------------------------------------------------
sudo -u clickhouse clickhouse-server --config-file=/etc/clickhouse-server/config.xml --daemon

# Wait for ClickHouse to be ready
for i in $(seq 1 30); do
    if clickhouse-client --query "SELECT 1" >/dev/null 2>&1; then break; fi
    sleep 0.5
done

# Create test database
clickhouse-client --query "CREATE DATABASE IF NOT EXISTS traceway_test"

# --------------------------------------------------------------------------
# Apply ClickHouse migrations
# --------------------------------------------------------------------------
echo "Applying ClickHouse migrations..."
for f in $(ls /app/backend/app/migrations/ch/*.up.sql | sort); do
    echo "  $(basename "$f")"
    clickhouse-client --database traceway_test --multiquery < "$f" || true
done

# --------------------------------------------------------------------------
# Apply PostgreSQL migrations
# --------------------------------------------------------------------------
echo "Applying PostgreSQL migrations..."
for f in $(ls /app/backend/app/migrations/pg/*.up.sql | sort); do
    echo "  $(basename "$f")"
    PGPASSWORD=traceway psql -h localhost -U traceway -d traceway_test -f "$f" 2>/dev/null || true
done

# --------------------------------------------------------------------------
# Run tests
# --------------------------------------------------------------------------
echo ""
echo "Running pgch tests..."
cd /app/backend

TEST_CLICKHOUSE_SERVER=localhost:9000 \
TEST_CLICKHOUSE_DATABASE=traceway_test \
TEST_POSTGRES_HOST=localhost \
TEST_POSTGRES_DATABASE=traceway_test \
TEST_POSTGRES_USERNAME=traceway \
TEST_POSTGRES_PASSWORD=traceway \
go test -tags "transactional_pg telemetry_ch" -v -count=1 ./app/repositories/...
