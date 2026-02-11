#!/bin/bash
set -e

PG_DATA="/var/lib/postgresql/data"
PG_BIN=$(ls -d /usr/lib/postgresql/*/bin | head -1)

if [ -z "$PG_BIN" ]; then
    echo "ERROR: Could not find PostgreSQL binaries"
    exit 1
fi

echo "Using PostgreSQL binaries from: $PG_BIN"

if [ ! -s "$PG_DATA/PG_VERSION" ]; then
    echo "Initializing PostgreSQL data directory..."

    mkdir -p "$PG_DATA"
    chown postgres:postgres "$PG_DATA"
    chmod 700 "$PG_DATA"

    runuser -u postgres -- "$PG_BIN/initdb" -D "$PG_DATA"

    cat > "$PG_DATA/pg_hba.conf" << 'EOF'
local   all   all                 trust
host    all   all   127.0.0.1/32  trust
host    all   all   ::1/128       trust
EOF

    cat >> "$PG_DATA/postgresql.conf" << 'EOF'
listen_addresses = '127.0.0.1'
port = 5432
max_connections = 100
shared_buffers = 128MB
log_destination = 'stderr'
logging_collector = off
EOF

    runuser -u postgres -- "$PG_BIN/pg_ctl" -D "$PG_DATA" -w start

    runuser -u postgres -- psql -c "CREATE USER traceway WITH PASSWORD '';"
    runuser -u postgres -- psql -c "CREATE DATABASE traceway OWNER traceway;"

    runuser -u postgres -- "$PG_BIN/pg_ctl" -D "$PG_DATA" -w stop

    echo "PostgreSQL initialized successfully."
fi

echo "Starting PostgreSQL..."
exec runuser -u postgres -- "$PG_BIN/postgres" -D "$PG_DATA"
