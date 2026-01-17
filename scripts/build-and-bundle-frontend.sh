#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# Build frontend
cd "$ROOT_DIR/frontend"
npm install
npm run build

# Clean and copy to backend static folder
rm -rf "$ROOT_DIR/backend/static/dist"
mkdir -p "$ROOT_DIR/backend/static/dist"
cp -r "$ROOT_DIR/frontend/build/"* "$ROOT_DIR/backend/static/dist/"

echo "Frontend built and bundled into backend/static/dist/"
