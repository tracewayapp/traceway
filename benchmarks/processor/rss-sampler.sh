#!/usr/bin/env bash
PID="$1"
OUT="$2"
echo "ts,rss_kb,cpu_pct" > "$OUT"
while kill -0 "$PID" 2>/dev/null; do
  LINE=$(ps -o rss=,pcpu= -p "$PID" 2>/dev/null | awk '{print $1","$2}')
  [ -n "$LINE" ] && echo "$(date +%s),$LINE" >> "$OUT"
  sleep 1
done
