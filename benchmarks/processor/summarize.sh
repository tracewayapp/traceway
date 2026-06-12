#!/usr/bin/env bash
set -euo pipefail

RESULTS="${1:-./results}"

printf '| %-20s | %-6s | %12s | %10s | %12s | %8s | %7s | %s |\n' impl scenario max_stacks/s p99_ms_max peak_rss_mb avg_cpu% symb% outcome
printf '|----------------------|--------|--------------|------------|--------------|----------|---------|---------|\n'
for d in "$RESULTS"/*/; do
  [ -f "$d/loadgen.json" ] || continue
  name=$(basename "$d")
  impl="${name%-*}"
  scenario="${name##*-}"
  maxr=$(jq '[.[].stacks_per_sec] | max | floor' "$d/loadgen.json")
  p99=$(jq '([.[].stacks_per_sec] | max) as $m | [.[] | select(.stacks_per_sec == $m)][0].p99_ms' "$d/loadgen.json")
  peak=$(tail -n +2 "$d/rss.csv" 2>/dev/null | cut -d, -f2 | sort -n | tail -1)
  peak_mb=$(( ${peak:-0} / 1024 ))
  cpu=$(tail -n +2 "$d/rss.csv" 2>/dev/null | cut -d, -f3 | awk '{s+=$1; n++} END {if (n>0) printf "%.0f", s/n; else print 0}')
  pct=$(jq -r 'if .requests > 0 then (100 * .symbolicated / .requests | floor) else 0 end' "$d/drain.json" 2>/dev/null || echo 0)
  outcome=survived
  if [ -f "$d/died" ]; then
    secs=$(cat "$d/died")
    outcome="died@${secs}s"
  fi
  printf '| %-20s | %-6s | %12s | %10s | %12s | %8s | %7s | %s |\n' "$impl" "$scenario" "$maxr" "$p99" "$peak_mb" "$cpu" "$pct" "$outcome"
done
