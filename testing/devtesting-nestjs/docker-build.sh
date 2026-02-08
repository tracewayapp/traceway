#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
JS_CLIENT="$SCRIPT_DIR/../../../js-client"

echo "Packing @tracewayapp packages..."

cd "$JS_CLIENT/packages/core"
npm pack --pack-destination "$SCRIPT_DIR" 2>/dev/null
mv "$SCRIPT_DIR"/tracewayapp-core-*.tgz "$SCRIPT_DIR/tracewayapp-core.tgz"

cd "$JS_CLIENT/packages/backend"
npm pack --pack-destination "$SCRIPT_DIR" 2>/dev/null
mv "$SCRIPT_DIR"/tracewayapp-backend-*.tgz "$SCRIPT_DIR/tracewayapp-backend.tgz"

cd "$JS_CLIENT/packages/nestjs"
npm pack --pack-destination "$SCRIPT_DIR" 2>/dev/null
mv "$SCRIPT_DIR"/tracewayapp-nestjs-*.tgz "$SCRIPT_DIR/tracewayapp-nestjs.tgz"

echo "Building Docker image..."
cd "$SCRIPT_DIR"
docker build -t devtesting-nestjs .

echo "Cleaning up tarballs..."
rm -f "$SCRIPT_DIR"/tracewayapp-*.tgz

echo ""
echo "Done! Run with:"
echo '  docker run -p 3001:3001 -e TRACEWAY_ENDPOINT="your-token@http://host.docker.internal:8082/api/report" devtesting-nestjs'
