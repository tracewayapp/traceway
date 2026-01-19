#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$ROOT_DIR/frontend"

# Get current version from package.json
CURRENT_VERSION=$(node -p "require('./package.json').version")
echo "Current version: $CURRENT_VERSION"

# Ask if user wants to update version
read -p "Do you want to update the version? (y/N): " UPDATE_VERSION
if [[ "$UPDATE_VERSION" =~ ^[Yy]$ ]]; then
    read -p "Enter new version (current: $CURRENT_VERSION): " NEW_VERSION
    if [ -n "$NEW_VERSION" ]; then
        # Update version in package.json using npm
        npm version "$NEW_VERSION" --no-git-tag-version
        echo "Version updated to $NEW_VERSION"
    else
        echo "No version entered, keeping $CURRENT_VERSION"
    fi
fi

# Build frontend
npm install
npm run build

# Clean and copy to backend static folder
rm -rf "$ROOT_DIR/backend/static/dist"
mkdir -p "$ROOT_DIR/backend/static/dist"
cp -r "$ROOT_DIR/frontend/build/"* "$ROOT_DIR/backend/static/dist/"

echo "Frontend built and bundled into backend/static/dist/"
