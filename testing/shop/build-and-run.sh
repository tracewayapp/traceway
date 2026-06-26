#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "==> Building frontend (with source maps)"
cd frontend
npm install
npm run build
cd ..

echo "==> Embedding frontend into backend"
rm -rf backend/dist
mkdir -p backend/dist
cp -R frontend/dist/. backend/dist/
touch backend/dist/.gitkeep

echo "==> Building backend"
cd backend
CGO_ENABLED=1 go build -o shop .

echo "==> Starting shop on http://localhost:8090"
set -a
[ -f .env ] && source .env
set +a
exec ./shop
