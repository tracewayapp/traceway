#!/usr/bin/env python3
"""Render firebolt-bench JSON reports into a markdown summary.

Usage: python3 summarize.py <dir-or-json...>
"""
import json
import statistics
import sys
from pathlib import Path


def load(paths):
    files = []
    for p in map(Path, paths):
        if p.is_dir():
            files.extend(sorted(p.glob("*.json")))
        else:
            files.append(p)
    reports = []
    for f in files:
        try:
            reports.append(json.loads(f.read_text()))
        except (json.JSONDecodeError, OSError) as e:
            print(f"skipping {f}: {e}", file=sys.stderr)
    return reports


def fmt_rows(n):
    if n >= 1_000_000:
        return f"{n / 1_000_000:g}M"
    if n >= 1_000:
        return f"{n / 1_000:g}k"
    return str(n)


def main():
    reports = load(sys.argv[1:] or ["."])
    if not reports:
        print("no reports found", file=sys.stderr)
        sys.exit(1)

    print(f"# Firebolt direct benchmark\n")
    eng = {r.get("engineVersion", "?") for r in reports}
    print(f"Engine: `{', '.join(sorted(eng))}` — target {reports[0].get('target')}\n")

    print("## Ingest ramp (multi-row INSERT over HTTP)\n")
    print("| engine | signal | batch | rows/sec | req p50 | req p95 | req p99 | errors |")
    print("|---|---|---|---|---|---|---|---|")
    for r in reports:
        eng = r.get("dialect", "firebolt") + ("-tuned" if "tuned" in r.get("scenario", "") else "")
        for s in r.get("ingestRamp") or []:
            print(
                f"| {eng} | {r['signal']} | {s['batchSize']} | {s['rowsPerSec']:,.0f} "
                f"| {s['p50Ms']:.0f}ms | {s['p95Ms']:.0f}ms | {s['p99Ms']:.0f}ms | {s['errors']} |"
            )
    print()

    print("## Read probes\n")
    print("run0 is the first execution after fill; with --cache-bust every run is a real execution (no result cache), otherwise later runs may be cache hits.\n")
    print("| engine | signal | table rows | query | run0 | best | median | rows |")
    print("|---|---|---|---|---|---|---|---|")
    for r in reports:
        eng = r.get("dialect", "firebolt") + ("-tuned" if "tuned" in r.get("scenario", "") else "")
        for fl in r.get("fillLevels") or []:
            for q in fl.get("queries") or []:
                if q.get("error"):
                    print(f"| {eng} | {r['signal']} | {fmt_rows(fl['tableRows'])} | {q['name']} | ERROR | — | — | — |")
                    continue
                runs = q["runsMs"]
                best = min(runs[1:]) if len(runs) > 1 else runs[0]
                print(
                    f"| {eng} | {r['signal']} | {fmt_rows(fl['tableRows'])} | {q['name']} "
                    f"| {runs[0]:.0f}ms | {best:.1f}ms | {q.get('medianMs', 0):.1f}ms | {q['rowsReturned']} |"
                )
    print()

    print("## Fill throughput\n")
    print("| engine | signal | filled to | fill rows/sec | write rows/sec during probes |")
    print("|---|---|---|---|---|")
    for r in reports:
        eng = r.get("dialect", "firebolt") + ("-tuned" if "tuned" in r.get("scenario", "") else "")
        for fl in r.get("fillLevels") or []:
            if fl.get("fillSeconds"):
                uw = fl.get("writeRowsPerSecDuringProbes")
                print(f"| {eng} | {r['signal']} | {fmt_rows(fl['tableRows'])} | {fl['fillRowsPerSec']:,.0f} | {f'{uw:,.0f}' if uw else '—'} |")


if __name__ == "__main__":
    main()
