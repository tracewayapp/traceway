#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# 1. Ask for version
read -p "Enter release version (e.g. 1.5.0): " VERSION
if [ -z "$VERSION" ]; then
    echo "Error: version is required"
    exit 1
fi

if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: version must be in X.Y.Z format"
    exit 1
fi

TAG="backend/v${VERSION}"

if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "Error: tag $TAG already exists"
    exit 1
fi

echo "Will create tag: $TAG"
echo ""

# 2. Build frontend
echo "==> Building frontend..."
cd "$ROOT_DIR/frontend"
npm install
npm run build
cd "$ROOT_DIR"

# 3. Copy into backend embed directory
echo "==> Bundling frontend into backend/static/frontend/..."
rm -rf "$ROOT_DIR/backend/static/frontend"
mkdir -p "$ROOT_DIR/backend/static/frontend"
cp -r "$ROOT_DIR/frontend/build/"* "$ROOT_DIR/backend/static/frontend/"

# The backend imports the cli module, and importers of the backend ignore its
# replace directive: pin the cli version that release-cli.yml tags from this commit.
echo "==> Pinning the cli module to v${VERSION}..."
(cd "$ROOT_DIR/backend" && go mod edit -require="github.com/tracewayapp/traceway/cli/v2@v${VERSION}")

# 4. Stage and commit
echo "==> Committing..."
git add backend/static/frontend/
git add -A backend/
git commit -m "release: backend v${VERSION}"

# 5. Tag
echo "==> Tagging $TAG..."
git tag "$TAG"

# 6. Push
BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "==> Pushing $BRANCH + tag $TAG..."
git push origin "$BRANCH"
git push origin "$TAG"

echo ""
echo "Released $TAG"
echo "Users can now: go get github.com/tracewayapp/traceway/backend/v2@v${VERSION}"
