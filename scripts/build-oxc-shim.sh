#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

if ! command -v cargo &> /dev/null; then
    echo "cargo not found - install Rust (https://rustup.rs) to build the oxc shim"
    exit 1
fi

cd "$ROOT_DIR/backend/app/symbolicator/scopes/oxc-shim"
cargo build --release

echo "Built liboxc_shim.a"
echo "Build the backend with the oxc parser enabled:"
echo "  cd backend && go build -tags oxc ."
echo "Select it at runtime with SYMBOLICATOR_PARSER=oxc"
