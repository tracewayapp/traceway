#!/usr/bin/env bash
# Download the pinned prebuilt libduckdb into benchmarks/metricsdb/lib/ so the
# duckdb crate links against it (DUCKDB_LIB_DIR / DUCKDB_INCLUDE_DIR) instead
# of building DuckDB from source. The version comes from
# bench/duckdb-version.txt and must equal the crate version.
#
# Usage: fetch-libduckdb.sh <linux-amd64|linux-arm64|osx-universal>
set -euo pipefail

PLATFORM="${1:?usage: fetch-libduckdb.sh <linux-amd64|linux-arm64|osx-universal>}"
case "${PLATFORM}" in
    linux-amd64|linux-arm64) LIB_FILE="libduckdb.so" ;;
    osx-universal) LIB_FILE="libduckdb.dylib" ;;
    *) echo "unknown platform '${PLATFORM}' (expected linux-amd64|linux-arm64|osx-universal)" >&2; exit 2 ;;
esac

MDB_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "${MDB_ROOT}/bench/duckdb-version.txt")"
LIB_DIR="${MDB_ROOT}/lib"
MARKER="${LIB_DIR}/.version"
WANT="${VERSION} ${PLATFORM}"

if [[ -f "${MARKER}" && "$(cat "${MARKER}")" == "${WANT}" && -f "${LIB_DIR}/${LIB_FILE}" && -f "${LIB_DIR}/duckdb.h" ]]; then
    echo "libduckdb ${VERSION} (${PLATFORM}) already in ${LIB_DIR}" >&2
    exit 0
fi

# The duckdb crate encodes the lib version: lib a.b.c is crate a.<a><bb><cc>.<n>,
# so 1.5.5 is 1.10505.0.
cargo_toml="${MDB_ROOT}/bench/Cargo.toml"
IFS='.' read -r v_major v_minor v_patch <<< "${VERSION}"
crate_prefix="${v_major}.$(printf '%d%02d%02d' "${v_major}" "${v_minor}" "${v_patch}")."
if [[ -f "${cargo_toml}" ]] && grep -qE '^duckdb[[:space:]]*=.*version' "${cargo_toml}" \
    && ! grep -E '^duckdb[[:space:]]*=.*version' "${cargo_toml}" | grep -qF "${crate_prefix}"; then
    echo "WARN: bench/Cargo.toml does not pin duckdb crate ${crate_prefix}x for lib ${VERSION}; the lib and crate versions must match" >&2
fi

url="https://github.com/duckdb/duckdb/releases/download/v${VERSION}/libduckdb-${PLATFORM}.zip"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

echo "downloading ${url}" >&2
curl -fsSL --retry 3 -o "${tmp}/libduckdb.zip" "${url}"
unzip -q -o "${tmp}/libduckdb.zip" -d "${tmp}/unpacked"

rm -rf "${LIB_DIR}"
mkdir -p "${LIB_DIR}"
cp "${tmp}/unpacked/${LIB_FILE}" "${tmp}/unpacked/duckdb.h" "${LIB_DIR}/"
echo "${WANT}" > "${MARKER}"
echo "libduckdb ${VERSION} (${PLATFORM}) -> ${LIB_DIR}/${LIB_FILE}" >&2
