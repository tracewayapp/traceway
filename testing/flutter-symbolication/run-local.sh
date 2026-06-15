#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE="$ROOT/backend/app/symbolicator/dart/fixtures/flutter-macos-arm64-dart3.10.1"
SYMBOLS="$FIXTURE/app.darwin-arm64.symbols"
TRACE="$FIXTURE/trace.txt"

INGEST_TOKEN="flutter-dev-token"
UPLOAD_TOKEN="flutter-upload-token"
BASE="http://localhost:8082"

for f in "$SYMBOLS" "$TRACE"; do
  [ -f "$f" ] || { echo "missing fixture: $f"; exit 1; }
done

WORK="$(mktemp -d)"
mkdir -p "$WORK/storage"
echo "work dir: $WORK"

echo "==> building devtesting-embedded"
(cd "$ROOT/examples/devtesting-embedded" && go build -o "$WORK/devtesting-embedded" .)

echo "==> starting backend (port 8082)"
( cd "$WORK" && "$WORK/devtesting-embedded" ) > "$WORK/server.log" 2>&1 &
SERVER_PID=$!
cleanup() { kill "$SERVER_PID" 2>/dev/null || true; }
trap cleanup EXIT

ready=
for _ in $(seq 1 60); do
  if curl -fsS "$BASE/api/has-organizations" >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[ -n "$ready" ] || { echo "FAIL: backend did not come up"; tail -40 "$WORK/server.log"; exit 1; }
echo "    backend is up"

DEBUG_ID="fe664295997135e7b67b648ba66ca9eb"
echo "==> uploading symbols (arch=arm64 debug_id=$DEBUG_ID)"
UP="$(curl -fsS -X POST "$BASE/api/symbols/upload" \
  -H "Authorization: Bearer $UPLOAD_TOKEN" \
  -F "files=@$SYMBOLS" -F "arch=arm64" -F "debug_id=$DEBUG_ID")"
echo "    response: $UP"
echo "$UP" | grep -q '"uploaded":1' || { echo "FAIL: symbols upload"; exit 1; }

post_report() {
  python3 - "$1" "$BASE/api/report" "$INGEST_TOKEN" <<'PY'
import sys, json, gzip, urllib.request
trace = open(sys.argv[1]).read()
url, token = sys.argv[2], sys.argv[3]
body = {
    "collectionFrames": [{
        "stackTraces": [{
            "stackTrace": trace,
            "recordedAt": "2026-06-13T00:00:00Z",
            "isMessage": False,
            "attributes": {},
        }],
        "metrics": [], "traces": [], "sessions": [],
    }],
    "appVersion": "1.0.0", "serverName": "",
}
data = gzip.compress(json.dumps(body).encode())
req = urllib.request.Request(url, data=data, method="POST", headers={
    "Content-Type": "application/json",
    "Content-Encoding": "gzip",
    "Authorization": "Bearer " + token,
})
with urllib.request.urlopen(req) as r:
    print("    report HTTP", r.status)
PY
}

echo "==> posting report #1 (raw non-symbolic trace)"
post_report "$TRACE"

echo "==> posting report #2 (same crash, different load address)"
sed -E 's/abs [0-9a-fA-F]+/abs 0000000700000000/' "$TRACE" > "$WORK/trace2.txt"
post_report "$WORK/trace2.txt"

TELDB="$WORK/storage/traceway_telemetry.db"
echo "==> reading back from $TELDB"

got=
for _ in $(seq 1 25); do
  if [ -f "$TELDB" ]; then
    got="$(sqlite3 "$TELDB" "SELECT stack_trace FROM exception_stack_traces WHERE stack_trace LIKE '%chargeCard%' LIMIT 1;" 2>/dev/null || true)"
    [ -n "$got" ] && break
  fi
  sleep 1
done

if [ -z "$got" ]; then
  echo "FAIL: no symbolicated exception stored"
  echo "--- tables ---"; sqlite3 "$TELDB" ".tables" 2>/dev/null || true
  echo "--- server log (tail) ---"; tail -40 "$WORK/server.log"
  exit 1
fi

echo "--- stored symbolicated stack trace ---"
echo "$got" | head -6
echo "$got" | grep -q "main.dart:20:3" || { echo "FAIL: chargeCard not resolved to main.dart:20:3"; exit 1; }

counts="$(sqlite3 "$TELDB" "SELECT COUNT(*), COUNT(DISTINCT exception_hash) FROM exception_stack_traces WHERE stack_trace LIKE '%chargeCard%';")"
total="${counts%%|*}"
distinct="${counts##*|}"
echo "==> grouping: $total row(s), $distinct distinct hash(es)"
[ "$distinct" = "1" ] || { echo "FAIL: expected 1 distinct exception hash, got $distinct"; exit 1; }

echo
echo "PASS: Flutter symbolication works end-to-end (upload -> report -> symbolicated + grouped)"
