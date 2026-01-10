#!/bin/bash
set -e

echo "Waiting for ClickHouse to be ready..."
/usr/local/bin/wait-for-clickhouse.sh

echo "Starting Traceway backend..."
exec /usr/local/bin/traceway-backend
