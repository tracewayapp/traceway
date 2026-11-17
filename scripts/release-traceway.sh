#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$ROOT_DIR"

# 1. Get current version
CURRENT_VERSION=$(node -p "require('./frontend/package.json').version")
echo "Current frontend version: $CURRENT_VERSION"
echo ""

# 2. Ask for new version
read -p "Enter release version (e.g. $CURRENT_VERSION): " VERSION
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

echo ""
echo "Will release: v${VERSION} (tag: $TAG)"
echo ""

# 3. Bump frontend version
echo "==> Bumping frontend version to $VERSION..."
cd "$ROOT_DIR/frontend"
npm version "$VERSION" --no-git-tag-version
cd "$ROOT_DIR"

# 4. Commit version bump
echo "==> Committing version bump..."
git add frontend/package.json frontend/package-lock.json
git commit -m "v to $VERSION"

# 5. Build frontend
echo "==> Building frontend..."
cd "$ROOT_DIR/frontend"
npm install
npm run build
cd "$ROOT_DIR"

# 6. Bundle into backend
echo "==> Bundling frontend into backend/static/frontend/..."
rm -rf "$ROOT_DIR/backend/static/frontend"
mkdir -p "$ROOT_DIR/backend/static/frontend"
cp -r "$ROOT_DIR/frontend/build/"* "$ROOT_DIR/backend/static/frontend/"

# The backend imports the cli module, and importers of the backend ignore its
# replace directive: pin the cli version that release-cli.yml tags from this commit.
echo "==> Pinning the cli module to v${VERSION}..."
(cd "$ROOT_DIR/backend" && go mod edit -require="github.com/tracewayapp/traceway/cli/v2@v${VERSION}")

# 7. Commit bundled frontend
echo "==> Committing release..."
git add backend/static/frontend/
git add -A backend/
git commit -m "release: backend v${VERSION}"

# 8. Tag
echo "==> Tagging $TAG..."
git tag "$TAG"

# 9. Push
BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "==> Pushing $BRANCH + tag $TAG..."
git push origin "$BRANCH"
git push origin "$TAG"

echo ""
echo "Released $TAG (v${VERSION})"
echo "Users can now: go get github.com/tracewayapp/traceway/backend/v2@v${VERSION}"
