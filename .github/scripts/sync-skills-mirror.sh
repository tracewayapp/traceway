#!/usr/bin/env bash
# Publishes the skills and plugin marketplace of a tracewayapp/traceway checkout
# to a tracewayapp/skills checkout. Run by .github/workflows/release-skills.yml.
#
#   sync-skills-mirror.sh <source-checkout> <mirror-checkout> <version>
#
# Commits, tags v<version> and pushes only when the content changed. Plugin
# versions are stamped to <version> on those commits because Claude Code only
# updates a plugin whose version changed. SYNC_NO_PUSH=1 stops before the push.

set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <source-checkout> <mirror-checkout> <version>" >&2
  exit 2
fi

src="$(cd "$1" && pwd)"
mirror="$(cd "$2" && pwd)"
version="$3"

if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must be X.Y.Z (got '$version')" >&2
  exit 2
fi

manifests=(.claude-plugin/plugin.json plugins/tw-write/.claude-plugin/plugin.json)

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

cp -R "$src/.claude-plugin" "$src/skills" "$src/plugins" "$stage/"
cp "$src/LICENSE" "$stage/"
cp "$src/.github/skills-mirror/README.md" "$stage/README.md"
find "$stage" -name .DS_Store -delete

rsync -a --delete --exclude .git "$stage/" "$mirror/"
cd "$mirror"

set_version() {
  jq --arg v "$2" '.version = $v' "$1" > "$1.tmp"
  mv "$1.tmp" "$1"
}

current=""
for f in "${manifests[@]}"; do
  prev="$(git show "HEAD:$f" 2>/dev/null | jq -r '.version // empty' 2>/dev/null || true)"
  if [[ -n "$prev" ]]; then
    set_version "$f" "$prev"
    current="$prev"
  fi
done

if [[ -z "$(git status --porcelain)" ]]; then
  echo "tracewayapp/skills already matches backend/v$version, nothing to publish"
  exit 0
fi

if [[ -n "$current" && "$(printf '%s\n%s\n' "$current" "$version" | sort -V | tail -n1)" != "$version" ]]; then
  echo "the mirror is at $current, newer than $version: refusing to publish an older release" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/v$version" >/dev/null; then
  echo "tag v$version already exists in the mirror but the content differs: refusing" >&2
  exit 1
fi

for f in "${manifests[@]}"; do
  set_version "$f" "$version"
done

source_sha="$(git -C "$src" rev-parse HEAD)"
git add -A
git -c user.name="github-actions[bot]" \
  -c user.email="41898282+github-actions[bot]@users.noreply.github.com" \
  commit -q -m "Sync from tracewayapp/traceway backend/v$version (${source_sha:0:12})"
git tag "v$version"
git --no-pager show --stat --format='%h %s' HEAD

if [[ "${SYNC_NO_PUSH:-}" == "1" ]]; then
  echo "SYNC_NO_PUSH=1: committed and tagged v$version locally, not pushed"
  exit 0
fi
git push --atomic origin HEAD:main "refs/tags/v$version"
