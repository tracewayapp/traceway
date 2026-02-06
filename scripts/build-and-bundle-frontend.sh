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

# Ask for billing path
read -p "Enter billing path (e.g., ../../traceway-cloud/frontend/src/lib/billing) or leave empty: " BILLING_PATH_INPUT
if [ -n "$BILLING_PATH_INPUT" ]; then
    export BILLING_PATH="$BILLING_PATH_INPUT"
    echo "BILLING_PATH set to: $BILLING_PATH"
else
    echo "Building without billing module"
fi

# Ask for cloud mode
read -p "Enable cloud mode? (y/N): " CLOUD_MODE_INPUT
if [[ "$CLOUD_MODE_INPUT" =~ ^[Yy]$ ]]; then
    export CLOUD_MODE=true
    echo "CLOUD_MODE enabled"
fi

# Ask for Turnstile site key
read -p "Enter Cloudflare Turnstile site key or leave empty to disable captcha: " TURNSTILE_SITE_KEY_INPUT
if [ -n "$TURNSTILE_SITE_KEY_INPUT" ]; then
    export PUBLIC_TURNSTILE_SITE_KEY="$TURNSTILE_SITE_KEY_INPUT"
    echo "Turnstile captcha enabled"
else
    echo "Building without Turnstile captcha"
fi

# Ask for Traceway URL
read -p "Enter Traceway connection string (e.g., token@https://host/api/report) or leave empty: " TRACEWAY_URL_INPUT
if [ -n "$TRACEWAY_URL_INPUT" ]; then
    export TRACEWAY_URL="$TRACEWAY_URL_INPUT"
    echo "Traceway self-monitoring enabled"
else
    echo "Building without Traceway self-monitoring"
fi

# Build frontend
npm install
npm run build

# Clean and copy to backend static folder
rm -rf "$ROOT_DIR/backend/static/dist"
mkdir -p "$ROOT_DIR/backend/static/dist"
cp -r "$ROOT_DIR/frontend/build/"* "$ROOT_DIR/backend/static/dist/"

echo "Frontend built and bundled into backend/static/dist/"
