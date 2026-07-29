#!/bin/sh
# Traceway CLI installer (Linux / macOS).
#
#   curl -fsSL https://cli.tracewayapp.com/install.sh | sh
#
# Served from cli.tracewayapp.com (Cloudflare, uploaded manually); this file
# is the source of truth — re-upload it after changing it. New CLI releases
# need no re-upload: the script resolves the latest release at runtime.
# For Windows, download the .zip from https://github.com/tracewayapp/traceway/releases.
#
# Optional:
#   TRACEWAY_CLI_VERSION   Version to install, e.g. 1.9.3 (default: latest CLI release)
#   TRACEWAY_INSTALL_DIR   Target directory (default: /usr/local/bin if writable,
#                          else ~/.local/bin)

set -eu

REPO="tracewayapp/traceway"
API="https://api.github.com/repos/${REPO}/releases"

err() { printf 'traceway-cli-install: error: %s\n' "$*" >&2; exit 1; }
log() { printf 'traceway-cli-install: %s\n' "$*"; }

command -v curl >/dev/null 2>&1 || err "curl is required"
command -v tar  >/dev/null 2>&1 || err "tar is required"

UNAME_S="$(uname -s)"
UNAME_M="$(uname -m)"
case "$UNAME_S" in
  Linux)  OS="linux"  ;;
  Darwin) OS="darwin" ;;
  *) err "unsupported OS: $UNAME_S (Linux and macOS only; for Windows download the .zip from https://github.com/${REPO}/releases)" ;;
esac
case "$UNAME_M" in
  x86_64|amd64)  ARCH="x86_64" ;;
  arm64|aarch64) ARCH="arm64"  ;;
  *) err "unsupported arch: $UNAME_M" ;;
esac
log "detected $OS/$ARCH"

# CLI releases are tagged cli/vX.Y.Z; the same repo also publishes backend
# releases, so "latest" must be resolved by scanning tags, not /releases/latest.
VERSION="${TRACEWAY_CLI_VERSION:-}"
VERSION="${VERSION#v}"
if [ -z "$VERSION" ]; then
  VERSION="$(curl -fsSL "${API}?per_page=30" \
    | grep -o '"tag_name":[[:space:]]*"cli/v[^"]*"' \
    | head -1 | sed 's|.*cli/v||; s|"$||')"
  [ -n "$VERSION" ] || err "could not determine the latest CLI release from ${API}"
fi
log "installing traceway CLI v${VERSION}"

RELEASE_JSON="$(curl -fsSL "${API}/tags/cli%2Fv${VERSION}")" \
  || err "no CLI release found for v${VERSION} (see https://github.com/${REPO}/releases)"

TARBALL="traceway_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="$(printf '%s' "$RELEASE_JSON" \
  | grep -o "\"browser_download_url\":[[:space:]]*\"[^\"]*/${TARBALL}\"" \
  | head -1 | sed 's|.*"\(https://[^"]*\)"|\1|')"
[ -n "$URL" ] || err "release v${VERSION} has no asset ${TARBALL}"
CHECKSUMS_URL="$(printf '%s' "$RELEASE_JSON" \
  | grep -o '"browser_download_url":[[:space:]]*"[^"]*/checksums\.txt"' \
  | head -1 | sed 's|.*"\(https://[^"]*\)"|\1|')"

TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

log "downloading $URL"
curl -fsSL -o "${TMP}/${TARBALL}" "$URL" || err "failed to download $URL"

if [ -n "$CHECKSUMS_URL" ]; then
  log "verifying sha256"
  curl -fsSL -o "${TMP}/checksums.txt" "$CHECKSUMS_URL" \
    || err "failed to download checksums.txt"
  EXPECTED="$(awk -v f="$TARBALL" '$2==f || $2=="*"f {print $1}' "${TMP}/checksums.txt")"
  [ -n "$EXPECTED" ] || err "no checksum entry for $TARBALL in checksums.txt"
  if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL="$(sha256sum "${TMP}/${TARBALL}" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    ACTUAL="$(shasum -a 256 "${TMP}/${TARBALL}" | awk '{print $1}')"
  else
    err "no sha256sum or shasum available on this host"
  fi
  [ "$EXPECTED" = "$ACTUAL" ] || err "checksum mismatch for $TARBALL (expected $EXPECTED, got $ACTUAL)"
fi

log "unpacking"
tar -xzf "${TMP}/${TARBALL}" -C "$TMP"
[ -f "${TMP}/traceway" ] || err "binary not found in $TARBALL (malformed release archive)"

if [ -n "${TRACEWAY_INSTALL_DIR:-}" ]; then
  BIN_DIR="$TRACEWAY_INSTALL_DIR"
  mkdir -p "$BIN_DIR" || err "cannot create $BIN_DIR"
elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  BIN_DIR="/usr/local/bin"
else
  BIN_DIR="${HOME}/.local/bin"
  mkdir -p "$BIN_DIR"
fi

log "installing → ${BIN_DIR}/traceway"
install -m 0755 "${TMP}/traceway" "${BIN_DIR}/traceway"

"${BIN_DIR}/traceway" version >/dev/null 2>&1 \
  || err "installed binary failed to run (${BIN_DIR}/traceway version)"

case ":${PATH}:" in
  *":${BIN_DIR}:"*) ;;
  *) log "note: ${BIN_DIR} is not on your PATH; add it to your shell profile" ;;
esac

log "done — installed $("${BIN_DIR}/traceway" version --output table)"
log "next: traceway login --url https://cloud.tracewayapp.com   (or your own instance)"
