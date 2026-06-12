#!/usr/bin/env bash
PID="$1"
OUT="$2"
echo "ts,rss_kb" > "$OUT"
while kill -0 "$PID" 2>/dev/null; do
  RSS=$(ps -o rss= -p "$PID" 2>/dev/null | tr -d ' ')
  [ -n "$RSS" ] && echo "$(date +%s),$RSS" >> "$OUT"
  sleep 1
done
