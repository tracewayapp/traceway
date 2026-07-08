#!/usr/bin/env bash
# Render every scene in diagrams/scenes/*.yaml to a self-contained SVG in
# public/diagrams/. Scenes inline the Traceway brand theme (canonical copy:
# diagrams/traceway.yaml) so a stock isotopo binary renders them unchanged.
#
#   go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest
#   ./diagrams/render.sh                 # render all
#   ./diagrams/render.sh mcp-flow        # render one scene
#
# Set ISOTOPO to point at a specific binary if it is not on PATH.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
ISOTOPO="${ISOTOPO:-isotopo}"
SCENES="$HERE/scenes"
OUT="$ROOT/public/diagrams"
mkdir -p "$OUT"

if ! command -v "$ISOTOPO" >/dev/null 2>&1 && [ ! -x "$ISOTOPO" ]; then
  echo "isotopo not found. Install: go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest" >&2
  echo "or set ISOTOPO=/path/to/isotopo" >&2
  exit 1
fi

render_one() {
  local name="$1"
  local src="$SCENES/$name.yaml"
  local tmp
  tmp="$(mktemp -d)"
  "$ISOTOPO" render "$src" "$tmp" >/dev/null
  cp "$tmp/topology.svg" "$OUT/$name.svg"
  rm -rf "$tmp"
  echo "  $name.svg"
}

echo "Rendering scenes -> public/diagrams/"
if [ "$#" -gt 0 ]; then
  render_one "$1"
else
  for f in "$SCENES"/*.yaml; do
    render_one "$(basename "$f" .yaml)"
  done
fi
