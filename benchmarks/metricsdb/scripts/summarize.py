#!/usr/bin/env python3
"""Turn metricsdb-bench result JSON files into summary.md, report.html (inline
SVG charts), chart-<tier>-*.svg and, when matplotlib is importable,
chart-<tier>-*.png. Usage: summarize.py <results-dir>"""
import datetime
import glob
import html
import json
import math
import os
import statistics
import sys

DB_ORDER = ["victoriametrics", "clickhouse", "clickhouse-map", "duckdb", "firebolt"]
DB_LABEL = {
    "victoriametrics": "VictoriaMetrics",
    "clickhouse": "ClickHouse",
    "clickhouse-map": "ClickHouse (map baseline)",
    "duckdb": "DuckDB",
    "firebolt": "Firebolt Core",
}
DB_COLOR = {
    "victoriametrics": "#7c5cff",
    "clickhouse": "#e0a100",
    "clickhouse-map": "#9a6b00",
    "duckdb": "#1f9d8f",
    "firebolt": "#e5484d",
}
QUERY_IDS = ["a", "b", "c", "d", "e1", "e2", "f"]
QUERY_INTENT = {
    "a": "one metric, avg per host, 1m buckets, last 1h",
    "b": "one series, 1m buckets, last 6h",
    "c": "latest value per host, top 20, last 10m",
    "d": "pod cpu by cluster, 5m buckets, last 24h",
    "e1": "discovery: active metric names, last 1h",
    "e2": "discovery: distinct hosts for one metric, last 1h",
    "f": "alert: count + p95 of latest per series, last 5m",
}
DB_HOW = {
    "victoriametrics": ("Prometheus remote write (protobuf + snappy), 20k samples per request, 8 writers; labels pre-encoded per series", "none: series = metric name + sorted labels", "/internal/force_flush, /internal/force_merge, wait for vm_active_merges = 0", "du of the storage dir (data + indexdb + cache)"),
    "clickhouse": ("RowBinary INSERTs over HTTP, 500k rows each, 4 writers, async_insert off", "points(series_id, ts, value) with Delta/DoubleDelta/Gorilla + ZSTD codecs, ORDER BY (series_id, ts), daily partitions; series dimension table", "wait for system.merges empty and part count stable", "du of /var/lib/clickhouse and sum(bytes_on_disk) from system.parts"),
    "clickhouse-map": ("RowBinary INSERTs over HTTP, 200k rows each, 4 writers", "Traceway's metric_points: name, value, tags Map, recorded_at; no codecs", "same as clickhouse", "same as clickhouse"),
    "duckdb": ("in-process Appender (Arrow record batches), 1M rows per append, batches sorted by (series_id, ts); threads = cores minus 2", "points(series_id, ts, value), series(series_id, name, tags MAP); no ART index", "CHECKPOINT until the WAL is empty", "points.duckdb + points.duckdb.wal"),
    "firebolt": ("Parquet batches of 1M rows uploaded inline (multipart, READ_PARQUET('upload://batch')), 2 writers; VACUUM every 20 inserts or above 50 tablets", "FACT TABLE points PRIMARY INDEX (series_id, ts) PARTITION BY day; series table with its own primary index", "VACUUM until the tablet count stops falling", "du of /var/lib/firebolt"),
}


def fmt_si(v, digits=2):
    if v is None:
        return "-"
    a = abs(v)
    if a >= 1e9:
        return f"{v / 1e9:.{digits}f}G"
    if a >= 1e6:
        return f"{v / 1e6:.{digits}f}M"
    if a >= 1e3:
        return f"{v / 1e3:.{digits}f}k"
    if a >= 100 or v == int(v):
        return f"{v:.0f}"
    return f"{v:.{digits}f}"


def fmt_bytes(v):
    if v is None:
        return "-"
    if v >= 1e12:
        return f"{v / 1e12:.2f} TB"
    if v >= 1e9:
        return f"{v / 1e9:.1f} GB"
    if v >= 1e6:
        return f"{v / 1e6:.0f} MB"
    return f"{v / 1e3:.0f} kB"


def fmt_ms(v):
    if v is None:
        return "-"
    if v >= 10000:
        return f"{v / 1000:.1f}s"
    return f"{v:.0f}ms"


def fmt_secs(s):
    if s is None:
        return "-"
    if s >= 3600:
        return f"{s / 3600:.1f}h"
    if s >= 60:
        return f"{s / 60:.0f}m"
    return f"{s:.0f}s"


def load_runs(d):
    runs = []
    for f in sorted(glob.glob(os.path.join(d, "*.json"))):
        try:
            with open(f) as fh:
                j = json.load(fh)
        except (OSError, ValueError):
            continue
        if not isinstance(j, dict) or "db" not in j or "tier" not in j:
            continue
        j["_file"] = os.path.basename(f)
        runs.append(j)
    runs.sort(key=lambda r: (r["tier"], DB_ORDER.index(r["db"]) if r["db"] in DB_ORDER else 99))
    return runs


def windows(run, phases=("warmup", "ingest")):
    return [w for w in run.get("timeline", []) if w.get("phase") in phases]


def rolling(vals, n):
    out = []
    for i in range(len(vals)):
        chunk = vals[max(0, i - n + 1): i + 1]
        out.append(sum(chunk) / len(chunk))
    return out


def points_at(run, t_s):
    last = 0
    for w in run.get("timeline", []):
        if w["t_s"] <= t_s:
            last = w.get("acked_total", last)
    return last


def headline(run):
    h = {"db": run["db"], "tier": run["tier"], "stub": bool(run.get("stub")), "outcome": run.get("verdict", {}).get("stopped_reason", "?"), "error": run.get("error")}
    if h["stub"]:
        return h
    ing = windows(run, ("ingest",))
    # A run that finishes inside the warm-up window (tiny local runs) has no
    # ingest-phase windows; fall back to every window that moved data.
    if not ing:
        ing = [w for w in windows(run) if w.get("acked_points", 0) > 0]
        h["short_run"] = True
    pps = [w["acked_pps"] for w in ing]
    h["peak_pps"] = max(rolling(pps, 6)) if pps else 0.0
    fb = run.get("verdict", {}).get("fell_behind_at_s")
    steady = [w["acked_pps"] for w in ing if w["t_s"] >= 300 and (fb is None or w["t_s"] < fb)]
    h["sustained_pps"] = statistics.median(steady) if steady else (statistics.median(pps) if pps else 0.0)
    th = run.get("throughput", {})
    h["acked_points"] = th.get("acked_points", 0)
    h["points_lost"] = th.get("points_lost", 0)
    h["plateau_pps"] = th.get("plateau_pps", 0.0)
    h["hit_max_ingest"] = th.get("hit_max_ingest", False)
    h["bottleneck"] = th.get("bench_bottleneck_suspected", False)
    disk = run.get("disk", {})
    h["disk_bytes"] = disk.get("total_bytes", 0)
    h["bytes_per_point"] = disk.get("bytes_per_point", 0.0)
    h["ratio_raw16"] = disk.get("ratio_vs_raw16", 0.0)
    h["ratio_logical"] = disk.get("ratio_vs_logical", 0.0)
    settle = run.get("phases", {}).get("settle") or {}
    h["settle_s"] = settle.get("ms", 0) / 1000.0 if settle else None
    h["settled"] = settle.get("settled") if settle else None
    h["ingest_s"] = run.get("phases", {}).get("ingest_s", 0)
    h["q_ingest"] = run.get("queries_during_ingest", {})
    h["q_cold"] = run.get("queries_cold", {})
    v = run.get("verdict", {})
    h["fell_behind_at_s"] = v.get("fell_behind_at_s")
    h["fell_behind_points"] = points_at(run, fb) if fb is not None else None
    h["fell_behind_reason"] = v.get("fell_behind_reason")
    un = v.get("unusable_at_s")
    h["unusable_at_s"] = un
    h["unusable_points"] = points_at(run, un) if un is not None else None
    h["unusable_reason"] = v.get("unusable_reason")
    h["restarts"] = v.get("process_restarts", 0)
    h["fingerprint"] = run.get("bench", {}).get("data_fingerprint")
    procs = [w["proc"]["peak_rss_bytes"] for w in run.get("timeline", []) if w.get("proc")]
    h["peak_rss"] = max(procs) if procs else 0
    cold_ms = [q.get("warm_median_ms") for q in h["q_cold"].values() if q.get("status") == "ok"]
    h["cold_median_ms"] = statistics.median(cold_ms) if cold_ms else None
    pm = run.get("postmortem") or {}
    h["oom_killed"] = pm.get("oomKilled") or pm.get("oom_killed")
    return h


# ---------------------------------------------------------------- SVG charts

def nice_ticks(lo, hi, n=5):
    if hi <= lo:
        hi = lo + 1
    raw = (hi - lo) / n
    mag = 10 ** math.floor(math.log10(raw))
    for m in (1, 2, 5, 10):
        step = m * mag
        if (hi - lo) / step <= n + 1:
            break
    start = math.floor(lo / step) * step
    ticks = []
    t = start
    while t <= hi + step * 0.5:
        if t >= lo - step * 0.5:
            ticks.append(t)
        t += step
    return ticks


def log_ticks(lo, hi):
    lo = max(lo, 1e-9)
    ticks = []
    e = math.floor(math.log10(lo))
    while 10 ** e <= hi * 1.001:
        if 10 ** e >= lo * 0.999:
            ticks.append(10 ** e)
        e += 1
    return ticks or [lo, hi]


def svg_line_chart(series, title, x_label, y_label, log_y=False, width=900, height=320, y_fmt=fmt_si, markers=None, note=None):
    """series: list of (db, [(x, y), ...])"""
    ml, mr, mt, mb = 64, 20, 40, 46
    pw, ph = width - ml - mr, height - mt - mb
    pts = [(x, y) for _, s in series for x, y in s if y is not None and (not log_y or y > 0)]
    if not pts:
        return f'<svg viewBox="0 0 {width} {height}" width="100%" class="chart"><text x="{width/2}" y="{height/2}" text-anchor="middle" fill="currentColor" opacity="0.6">{html.escape(title)}: no data</text></svg>'
    xmin, xmax = min(p[0] for p in pts), max(p[0] for p in pts)
    ymin, ymax = min(p[1] for p in pts), max(p[1] for p in pts)
    if xmax <= xmin:
        xmax = xmin + 1
    if log_y:
        ymin = 10 ** math.floor(math.log10(max(ymin, 1e-9)))
        ymax = 10 ** math.ceil(math.log10(max(ymax, ymin * 10)))
    else:
        ymin = 0 if ymin >= 0 else ymin
        ymax = ymax * 1.08 if ymax > 0 else 1
    def sx(x):
        return ml + (x - xmin) / (xmax - xmin) * pw
    def sy(y):
        if log_y:
            return mt + ph - (math.log10(y) - math.log10(ymin)) / (math.log10(ymax) - math.log10(ymin)) * ph
        return mt + ph - (y - ymin) / (ymax - ymin) * ph
    out = [f'<svg viewBox="0 0 {width} {height}" width="100%" class="chart" role="img" aria-label="{html.escape(title)}">']
    out.append(f'<text x="{ml}" y="22" font-size="14" font-weight="600" fill="currentColor">{html.escape(title)}</text>')
    yt = log_ticks(ymin, ymax) if log_y else nice_ticks(ymin, ymax)
    for t in yt:
        y = sy(t)
        out.append(f'<line x1="{ml}" x2="{ml+pw}" y1="{y:.1f}" y2="{y:.1f}" stroke="currentColor" stroke-opacity="0.12"/>')
        out.append(f'<text x="{ml-6}" y="{y+4:.1f}" font-size="11" text-anchor="end" fill="currentColor" opacity="0.75">{html.escape(y_fmt(t))}</text>')
    for t in nice_ticks(xmin, xmax, 6):
        x = sx(t)
        out.append(f'<line x1="{x:.1f}" x2="{x:.1f}" y1="{mt}" y2="{mt+ph}" stroke="currentColor" stroke-opacity="0.08"/>')
        out.append(f'<text x="{x:.1f}" y="{mt+ph+16}" font-size="11" text-anchor="middle" fill="currentColor" opacity="0.75">{fmt_si(t, 0)}</text>')
    out.append(f'<text x="{ml+pw/2}" y="{height-8}" font-size="11" text-anchor="middle" fill="currentColor" opacity="0.75">{html.escape(x_label)}</text>')
    out.append(f'<text transform="translate(14,{mt+ph/2}) rotate(-90)" font-size="11" text-anchor="middle" fill="currentColor" opacity="0.75">{html.escape(y_label)}</text>')
    for m in markers or []:
        x = sx(m["x"])
        if xmin <= m["x"] <= xmax:
            out.append(f'<line x1="{x:.1f}" x2="{x:.1f}" y1="{mt}" y2="{mt+ph}" stroke="{m["color"]}" stroke-dasharray="4 3" stroke-opacity="0.8"/>')
            out.append(f'<text x="{x+3:.1f}" y="{mt+12}" font-size="10" fill="{m["color"]}">{html.escape(m["label"])}</text>')
    for db, s in series:
        s = [(x, y) for x, y in s if y is not None and (not log_y or y > 0)]
        if not s:
            continue
        d = " ".join(f'{"M" if i == 0 else "L"}{sx(x):.1f},{sy(y):.1f}' for i, (x, y) in enumerate(s))
        out.append(f'<path d="{d}" fill="none" stroke="{DB_COLOR.get(db, "#888")}" stroke-width="2" stroke-linejoin="round"/>')
    lx = ml + pw - 8
    for i, (db, _) in enumerate(series):
        y = 30 + i * 16
        out.append(f'<rect x="{lx-150}" y="{y-9}" width="12" height="12" fill="{DB_COLOR.get(db, "#888")}" rx="2"/>')
        out.append(f'<text x="{lx-134}" y="{y+1}" font-size="11" fill="currentColor">{html.escape(DB_LABEL.get(db, db))}</text>')
    if note:
        out.append(f'<text x="{ml}" y="{mt-6}" font-size="11" fill="currentColor" opacity="0.7">{html.escape(note)}</text>')
    out.append("</svg>")
    return "\n".join(out)


def svg_bar_chart(groups, title, y_label, log_y=False, width=900, height=300, y_fmt=fmt_si, value_fmt=None):
    """groups: list of (category, [(db, value), ...])"""
    ml, mr, mt, mb = 64, 20, 40, 56
    pw, ph = width - ml - mr, height - mt - mb
    vals = [v for _, bars in groups for _, v in bars if v is not None and (not log_y or v > 0)]
    if not vals:
        return f'<svg viewBox="0 0 {width} {height}" width="100%" class="chart"><text x="{width/2}" y="{height/2}" text-anchor="middle" fill="currentColor" opacity="0.6">{html.escape(title)}: no data</text></svg>'
    vmax = max(vals)
    vmin = min(vals)
    if log_y:
        ymin = 10 ** math.floor(math.log10(max(vmin, 1e-9)))
        ymax = 10 ** math.ceil(math.log10(max(vmax, ymin * 10)))
    else:
        ymin, ymax = 0, vmax * 1.12 if vmax > 0 else 1
    def sy(y):
        if log_y:
            return mt + ph - (math.log10(max(y, ymin)) - math.log10(ymin)) / (math.log10(ymax) - math.log10(ymin)) * ph
        return mt + ph - (y - ymin) / (ymax - ymin) * ph
    out = [f'<svg viewBox="0 0 {width} {height}" width="100%" class="chart" role="img" aria-label="{html.escape(title)}">']
    out.append(f'<text x="{ml}" y="22" font-size="14" font-weight="600" fill="currentColor">{html.escape(title)}</text>')
    for t in (log_ticks(ymin, ymax) if log_y else nice_ticks(ymin, ymax)):
        y = sy(t)
        out.append(f'<line x1="{ml}" x2="{ml+pw}" y1="{y:.1f}" y2="{y:.1f}" stroke="currentColor" stroke-opacity="0.12"/>')
        out.append(f'<text x="{ml-6}" y="{y+4:.1f}" font-size="11" text-anchor="end" fill="currentColor" opacity="0.75">{html.escape(y_fmt(t))}</text>')
    out.append(f'<text transform="translate(14,{mt+ph/2}) rotate(-90)" font-size="11" text-anchor="middle" fill="currentColor" opacity="0.75">{html.escape(y_label)}</text>')
    gw = pw / max(len(groups), 1)
    seen = []
    for gi, (cat, bars) in enumerate(groups):
        n = max(len(bars), 1)
        bw = min(gw * 0.8 / n, 60)
        x0 = ml + gi * gw + (gw - bw * n) / 2
        for bi, (db, v) in enumerate(bars):
            if db not in seen:
                seen.append(db)
            if v is None or (log_y and v <= 0):
                out.append(f'<text x="{x0 + bi*bw + bw/2:.1f}" y="{mt+ph-4}" font-size="10" text-anchor="middle" fill="currentColor" opacity="0.6">n/a</text>')
                continue
            y = sy(v)
            out.append(f'<rect x="{x0 + bi*bw:.1f}" y="{y:.1f}" width="{bw-2:.1f}" height="{mt+ph-y:.1f}" fill="{DB_COLOR.get(db, "#888")}" rx="2"/>')
            label = value_fmt(v) if value_fmt else y_fmt(v)
            out.append(f'<text x="{x0 + bi*bw + bw/2 - 1:.1f}" y="{y-4:.1f}" font-size="10" text-anchor="middle" fill="currentColor">{html.escape(label)}</text>')
        out.append(f'<text x="{ml + gi*gw + gw/2:.1f}" y="{mt+ph+16}" font-size="11" text-anchor="middle" fill="currentColor" opacity="0.8">{html.escape(cat)}</text>')
    lx = ml + pw - 8
    for i, db in enumerate(seen):
        y = 30 + i * 16
        out.append(f'<rect x="{lx-150}" y="{y-9}" width="12" height="12" fill="{DB_COLOR.get(db, "#888")}" rx="2"/>')
        out.append(f'<text x="{lx-134}" y="{y+1}" font-size="11" fill="currentColor">{html.escape(DB_LABEL.get(db, db))}</text>')
    out.append("</svg>")
    return "\n".join(out)


def chart_series(runs):
    """Extract every chart's series once; both SVG and PNG renderers use this."""
    S = {"throughput": [], "disk": [], "rss": [], "lag": [], "queries": {q: [] for q in QUERY_IDS}}
    for r in runs:
        if r.get("stub"):
            continue
        db = r["db"]
        ws = windows(r)
        pps = rolling([w["acked_pps"] for w in ws], 6)
        S["throughput"].append((db, [(w["t_s"] / 60, p) for w, p in zip(ws, pps)]))
        all_w = r.get("timeline", [])
        S["disk"].append((db, [(w["t_s"] / 60, w["disk"]["total_bytes"] / 1e9) for w in all_w if w.get("disk")]))
        S["rss"].append((db, [(w["t_s"] / 60, w["proc"]["rss_bytes"] / 1e9) for w in all_w if w.get("proc") and w["proc"].get("rss_bytes")]))
        S["lag"].append((db, [(w["t_s"] / 60, max(w["visibility"]["points_behind"], 1)) for w in ws if w.get("visibility")]))
        for q in QUERY_IDS:
            pts = []
            for w in ws:
                for o in w.get("queries", []):
                    if o["id"].lower() == q and o["status"] in ("ok", "suspect", "timeout"):
                        pts.append((w["t_s"] / 60, max(o["ms"], 0.1)))
            S["queries"][q].append((db, pts))
    return S


def markers_for(runs):
    ms = []
    for r in runs:
        v = r.get("verdict", {})
        if v.get("fell_behind_at_s") is not None:
            ms.append({"x": v["fell_behind_at_s"] / 60, "color": DB_COLOR.get(r["db"], "#888"), "label": f'{DB_LABEL.get(r["db"], r["db"])} fell behind'})
        if v.get("unusable_at_s") is not None:
            ms.append({"x": v["unusable_at_s"] / 60, "color": DB_COLOR.get(r["db"], "#888"), "label": f'{DB_LABEL.get(r["db"], r["db"])} unusable'})
    return ms


def build_svgs(runs, heads, tier, slow_ms):
    S = chart_series(runs)
    valid = [h for h in heads if not h["stub"]]
    svgs = {}
    svgs["headline"] = svg_bar_chart(
        [("sustained points/s", [(h["db"], h["sustained_pps"]) for h in valid]), ("peak points/s (60s)", [(h["db"], h["peak_pps"]) for h in valid])],
        f"Ingest throughput on {tier}", "points per second")
    svgs["disk-per-point"] = svg_bar_chart(
        [("bytes per point on disk", [(h["db"], h["bytes_per_point"]) for h in valid])],
        f"Disk per stored point after settle on {tier}", "bytes / point", value_fmt=lambda v: f"{v:.2f}")
    svgs["cold"] = svg_bar_chart(
        [(f"{q}", [(h["db"], (h["q_cold"].get(q) or {}).get("warm_median_ms") if (h["q_cold"].get(q) or {}).get("status") == "ok" else None) for h in valid]) for q in QUERY_IDS],
        f"Cold routine queries, warm median of 3 runs on {tier}", "ms (log)", log_y=True, y_fmt=fmt_ms, value_fmt=fmt_ms)
    svgs["ingest-queries"] = svg_bar_chart(
        [(f"{q}", [(h["db"], (h["q_ingest"].get(q) or {}).get("p95_ms") or None) for h in valid]) for q in QUERY_IDS],
        f"Routine queries during ingest, p95 on {tier}", "ms (log)", log_y=True, y_fmt=fmt_ms, value_fmt=fmt_ms)
    svgs["throughput"] = svg_line_chart(S["throughput"], "Acknowledged points/s over the run (60 s rolling mean)", "minutes since ingest start", "points/s", markers=markers_for(runs))
    svgs["disk"] = svg_line_chart(S["disk"], "Data directory size over the run", "minutes since ingest start", "GB", y_fmt=lambda v: f"{v:.0f}")
    svgs["rss"] = svg_line_chart(S["rss"], "Database resident memory", "minutes since ingest start", "GB", y_fmt=lambda v: f"{v:.1f}")
    svgs["lag"] = svg_line_chart(S["lag"], "Visibility lag: acknowledged points not yet queryable", "minutes since ingest start", "points behind (log)", log_y=True)
    for q in QUERY_IDS:
        svgs[f"query-{q}"] = svg_line_chart(S["queries"][q], f"Query {q}: {QUERY_INTENT[q]}", "minutes since ingest start", "ms (log)", log_y=True, y_fmt=fmt_ms, height=240,
                                             markers=[{"x": 0, "color": "#888", "label": ""}], note=f"threshold {fmt_ms(slow_ms)}")
    return svgs, S


def write_pngs(S, heads, tier, out_dir):
    try:
        import matplotlib
        matplotlib.use("Agg")
        import matplotlib.pyplot as plt
    except Exception:
        return []
    written = []
    def lines(key, title, ylabel, fname, log=False):
        fig, ax = plt.subplots(figsize=(10, 4), dpi=130)
        for db, pts in S[key]:
            if pts:
                ax.plot([p[0] for p in pts], [p[1] for p in pts], label=DB_LABEL.get(db, db), color=DB_COLOR.get(db), linewidth=1.6)
        ax.set_title(title)
        ax.set_xlabel("minutes since ingest start")
        ax.set_ylabel(ylabel)
        if log:
            ax.set_yscale("log")
        ax.grid(alpha=0.25)
        if ax.get_legend_handles_labels()[0]:
            ax.legend(fontsize=8)
        fig.tight_layout()
        p = os.path.join(out_dir, f"chart-{tier}-{fname}.png")
        fig.savefig(p)
        plt.close(fig)
        written.append(p)
    lines("throughput", "Acknowledged points/s (60 s rolling mean)", "points/s", "throughput")
    lines("disk", "Data directory size", "GB", "disk")
    lines("rss", "Database RSS", "GB", "rss")
    lines("lag", "Visibility lag", "points behind", "lag", log=True)
    valid = [h for h in heads if not h["stub"]]
    if valid:
        fig, axes = plt.subplots(1, 3, figsize=(12, 4), dpi=130)
        names = [DB_LABEL.get(h["db"], h["db"]) for h in valid]
        cols = [DB_COLOR.get(h["db"]) for h in valid]
        axes[0].bar(names, [h["sustained_pps"] for h in valid], color=cols)
        axes[0].set_title("sustained points/s")
        axes[1].bar(names, [h["bytes_per_point"] for h in valid], color=cols)
        axes[1].set_title("bytes per point")
        axes[2].bar(names, [h["cold_median_ms"] or 0 for h in valid], color=cols)
        axes[2].set_title("cold query median (ms)")
        for ax in axes:
            ax.tick_params(axis="x", labelrotation=20, labelsize=8)
            ax.grid(axis="y", alpha=0.25)
        fig.tight_layout()
        p = os.path.join(out_dir, f"chart-{tier}-headline.png")
        fig.savefig(p)
        plt.close(fig)
        written.append(p)
    return written


# ---------------------------------------------------------------- markdown

def q_cell(qs, key, q):
    e = qs.get(q)
    if not e:
        return "-"
    if key == "p95_ms":
        s = fmt_ms(e.get("p95_ms"))
        if e.get("timeouts"):
            s += f" ({e['timeouts']}to)"
        if e.get("errors"):
            s += f" ({e['errors']}err)"
        return s
    if e.get("status") != "ok":
        return e.get("status") or "-"
    return fmt_ms(e.get("warm_median_ms"))


def render_summary_md(runs, heads, tier, png_files):
    L = []
    valid = [h for h in heads if not h["stub"]]
    sm = next((r.get("series_model") for r in runs if not r.get("stub")), None) or {}
    L.append(f"# Metrics store benchmark: {tier}")
    L.append("")
    L.append(f"Runs: {len(heads)} ({', '.join(DB_LABEL.get(h['db'], h['db']) for h in heads)}). Generated {datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%d %H:%M UTC')}.")
    if sm:
        L.append(f"Workload: {fmt_si(sm.get('series', 0))} series ({sm.get('hosts')} hosts, {sm.get('pods')} pods, {sm.get('templates')} metric templates), "
                 f"{fmt_si(sm.get('points_planned', 0))} points planned, {sm.get('interval_ms', 0)/1000:.0f}s scrape interval, "
                 f"simulated span {sm.get('sim_start', '')[:16]} to {sm.get('sim_end', '')[:16]}, {sm.get('avg_tags_per_series', 0):.1f} tags per series.")
    L.append("")
    L.append("| DB | Peak points/s | Sustained points/s | Points stored | Bytes/point | Disk | Settle | Ingest q p95 (a/b/c/d/e1/e2/f) | Cold warm median (a/b/c/d/e1/e2/f) | Fell behind at | Unusable at | Outcome |")
    L.append("|---|---|---|---|---|---|---|---|---|---|---|---|")
    for h in heads:
        if h["stub"]:
            L.append(f"| {DB_LABEL.get(h['db'], h['db'])} | - | - | - | - | - | - | - | - | - | - | {h['outcome']} |")
            continue
        stored = fmt_si(h["acked_points"]) + (" (hit cap)" if h["hit_max_ingest"] else "")
        fb = f"{fmt_si(h['fell_behind_points'])} pts" if h["fell_behind_at_s"] is not None else "no"
        un = f"{fmt_si(h['unusable_points'])} pts" if h["unusable_at_s"] is not None else "no"
        settle = (fmt_secs(h["settle_s"]) + ("" if h["settled"] else " (cap)")) if h["settle_s"] is not None else "-"
        L.append(f"| {DB_LABEL.get(h['db'], h['db'])} | {fmt_si(h['peak_pps'])} | {fmt_si(h['sustained_pps'])} | {stored} | {h['bytes_per_point']:.2f} | {fmt_bytes(h['disk_bytes'])} | {settle} | "
                 f"{'/'.join(q_cell(h['q_ingest'], 'p95_ms', q) for q in QUERY_IDS)} | {'/'.join(q_cell(h['q_cold'], 'warm', q) for q in QUERY_IDS)} | {fb} | {un} | {h['outcome']}{' (bench bottleneck?)' if h['bottleneck'] else ''} |")
    L.append("")
    fps = {h["fingerprint"] for h in valid if h.get("fingerprint")}
    if len(fps) == 1:
        L.append(f"All runs received the identical corpus (fingerprint `{next(iter(fps))}`).")
    elif len(fps) > 1:
        L.append(f"WARNING: data fingerprints differ across runs ({', '.join(sorted(fps))}); the corpora were not identical.")
    for h in valid:
        notes = []
        if h["hit_max_ingest"]:
            notes.append("stopped at the ingest wall-clock cap, so points stored is a time budget, not a capacity")
        if h["bottleneck"]:
            notes.append("writers idled while the generator kept up in more than 10% of windows: the bench, not the DB, may have been the limit")
        if h["fell_behind_reason"]:
            notes.append(f"fell behind: {h['fell_behind_reason']}")
        if h["unusable_reason"]:
            notes.append(f"unusable: {h['unusable_reason']}")
        if h["restarts"]:
            notes.append(f"database restarted {h['restarts']}x")
        if h["oom_killed"]:
            notes.append("container was OOM-killed")
        if h["points_lost"]:
            notes.append(f"{fmt_si(h['points_lost'])} points lost to failed writes")
        if notes:
            L.append(f"- {DB_LABEL.get(h['db'], h['db'])}: " + "; ".join(notes))
    L.append("")
    L.append("Query intents: " + "; ".join(f"{q} = {QUERY_INTENT[q]}" for q in QUERY_IDS) + ".")
    if png_files:
        L.append("")
        L.append("Charts: " + ", ".join(f"`{os.path.basename(p)}`" for p in png_files) + " (artifact). `report.html` has the same charts inline with the methodology.")
    L.append("")
    L.append("Generated by `benchmarks/metricsdb/scripts/summarize.py`.")
    return "\n".join(L) + "\n"


# ---------------------------------------------------------------- html report

CSS = """
:root{--bg:#f5f6f8;--fg:#1b1f27;--muted:#5b6472;--card:#ffffff;--line:#d6dae2;--accent:#25406b;--ok:#1e7a4c;--warn:#9a5b00;--bad:#b42318;--chart-bg:#ffffff}
:root:not([data-theme="light"]){color-scheme:light dark}
@media (prefers-color-scheme: dark){:root:not([data-theme="light"]){--bg:#14171c;--fg:#e8eaee;--muted:#9aa3b2;--card:#1b1f26;--line:#2c323d;--accent:#8fb0e6;--ok:#4fc98a;--warn:#e2a64a;--bad:#f0776d;--chart-bg:#1b1f26}}
:root[data-theme="dark"]{--bg:#14171c;--fg:#e8eaee;--muted:#9aa3b2;--card:#1b1f26;--line:#2c323d;--accent:#8fb0e6;--ok:#4fc98a;--warn:#e2a64a;--bad:#f0776d;--chart-bg:#1b1f26}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font-family:"IBM Plex Sans",-apple-system,"Segoe UI",Roboto,sans-serif;font-size:15.5px;line-height:1.55}
main{max-width:1080px;margin:0 auto;padding:40px 22px 80px}
h1,h2,h3{font-family:Archivo,"IBM Plex Sans",-apple-system,sans-serif;text-wrap:balance;letter-spacing:-0.015em}
h1{font-size:clamp(28px,4vw,40px);line-height:1.1;margin:0 0 10px;font-weight:700}
h2{font-size:22px;margin:52px 0 12px;padding-top:14px;border-top:1px solid var(--line);font-weight:650}
h3{font-size:16px;margin:28px 0 8px;font-weight:600}
p,li{max-width:72ch}
ul{padding-left:20px}
.sub{color:var(--muted);margin:0 0 22px;max-width:72ch}
.eyebrow{font-family:"IBM Plex Mono",ui-monospace,monospace;font-size:11.5px;letter-spacing:.12em;text-transform:uppercase;color:var(--accent);margin-bottom:12px}
.card{background:var(--card);border:1px solid var(--line);border-radius:6px;padding:16px 18px;margin:14px 0}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(320px,1fr));gap:14px}
.kpis{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:12px;margin:18px 0 6px}
.kpi{background:var(--card);border:1px solid var(--line);border-top:3px solid var(--kpi);border-radius:6px;padding:12px 14px}
.kpi .name{font-weight:600;font-size:14px;margin-bottom:6px}
.kpi dl{display:grid;grid-template-columns:auto 1fr;gap:2px 10px;margin:0;font-size:13px}
.kpi dt{color:var(--muted)}
.kpi dd{margin:0;font-family:"IBM Plex Mono",ui-monospace,monospace;font-variant-numeric:tabular-nums;text-align:right}
table{border-collapse:collapse;width:100%;font-size:13px;font-variant-numeric:tabular-nums}
.tablewrap{overflow-x:auto;border:1px solid var(--line);border-radius:6px;background:var(--card);margin:12px 0}
th,td{text-align:left;padding:8px 10px;border-bottom:1px solid var(--line);vertical-align:top;white-space:nowrap}
td{font-family:"IBM Plex Mono",ui-monospace,monospace;font-size:12.5px}
td:first-child,td.wrap{font-family:"IBM Plex Sans",-apple-system,sans-serif;font-size:13px}
th{font-family:"IBM Plex Mono",ui-monospace,monospace;font-weight:500;color:var(--muted);font-size:11px;letter-spacing:.06em;text-transform:uppercase;background:var(--card)}
td.wrap,th.wrap{white-space:normal;min-width:220px}
tr:last-child td{border-bottom:none}
.chart{display:block;height:auto;background:var(--chart-bg);border:1px solid var(--line);border-radius:6px;margin:12px 0;padding:6px;font-family:"IBM Plex Mono",ui-monospace,monospace}
code{font-family:"IBM Plex Mono",ui-monospace,monospace;font-size:12.5px;background:var(--card);border:1px solid var(--line);border-radius:4px;padding:0 4px}
.pill{display:inline-block;padding:0 7px;border-radius:3px;font-size:11px;border:1px solid var(--line);color:var(--muted);font-family:"IBM Plex Mono",ui-monospace,monospace;letter-spacing:.04em}
.ok{color:var(--ok)}.warn{color:var(--warn)}.bad{color:var(--bad)}
.swatch{display:inline-block;width:10px;height:10px;border-radius:2px;margin-right:7px;vertical-align:baseline}
.callout{border-left:3px solid var(--accent);padding:10px 14px;background:var(--card);border-radius:0 6px 6px 0;margin:14px 0;max-width:80ch}
dl.facts{display:grid;grid-template-columns:max-content 1fr;gap:6px 18px;margin:0}
dl.facts dt{color:var(--muted);font-family:"IBM Plex Mono",ui-monospace,monospace;font-size:12px;letter-spacing:.04em;text-transform:uppercase;padding-top:2px}
dl.facts dd{margin:0;max-width:78ch}
footer{margin-top:56px;color:var(--muted);font-size:13px;border-top:1px solid var(--line);padding-top:14px}
a{color:var(--accent)}
@media (prefers-reduced-motion: no-preference){.kpi{transition:border-color .15s}}
"""


def render_html(runs, heads, tier, svgs, slow_ms):
    valid = [h for h in heads if not h["stub"]]
    sm = next((r.get("series_model") for r in runs if not r.get("stub")), None) or {}
    bench = next((r.get("bench") for r in runs if not r.get("stub")), None) or {}
    dbc = {r["db"]: r.get("db_config", {}) for r in runs if not r.get("stub")}
    is_local = tier == "local"
    fps = {h["fingerprint"] for h in valid if h.get("fingerprint")}
    E = html.escape

    def db_name(db):
        return f'<span class="swatch" style="background:{DB_COLOR.get(db, "#888")}"></span>{E(DB_LABEL.get(db, db))}'

    H = []
    H.append("<title>The Metrics Store Race</title>")
    H.append('<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Archivo:wght@500;600;700&family=IBM+Plex+Sans:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap">')
    H.append(f"<style>{CSS}</style>")
    H.append("<main>")
    H.append('<div class="eyebrow">Traceway benchmark / OTel metrics storage</div>')
    H.append(f"<h1>Which store holds {fmt_si(sm.get('points_planned', 0), 0)} metric points best?</h1>")
    H.append(f'<p class="sub">VictoriaMetrics, ClickHouse, DuckDB and Firebolt Core driven to their ingest limit on one <b>{E(tier)}</b> box each, while routine dashboard queries run against them. '
             f"Generated {datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%d %H:%M UTC')} from {len(heads)} result file(s).</p>")
    if is_local:
        H.append('<div class="callout"><b>Local smoke run.</b> These numbers come from a laptop under Docker Desktop with a tiny corpus, not from the Hetzner ccx33 matrix. They prove the pipeline end to end; the shapes of the charts are meaningful, the absolute values are not.</div>')
    cap = [h for h in valid if h["hit_max_ingest"]]
    if cap:
        H.append(f'<div class="callout">{", ".join(E(DB_LABEL.get(h["db"], h["db"])) for h in cap)} stopped at the ingest wall-clock cap: "points stored" there is a time budget, not a capacity limit.</div>')
    short = [h for h in valid if h.get("short_run")]
    if short:
        H.append(f'<div class="callout">{", ".join(E(DB_LABEL.get(h["db"], h["db"])) for h in short)} finished ingest inside the warm-up window, so throughput there is the mean over the whole (very short) run and no routine query ran against a filling store.</div>')
    bott = [h for h in valid if h["bottleneck"]]
    if bott:
        H.append(f'<div class="callout"><b>Bench bottleneck suspected</b> for {", ".join(E(DB_LABEL.get(h["db"], h["db"])) for h in bott)}: writers idled while the generator kept up in more than 10% of windows, so the store may be faster than measured. Rerun with more bench cores.</div>')

    # results
    H.append("<h2>Result</h2>")
    if valid:
        best_pps = max(valid, key=lambda h: h["sustained_pps"])
        best_disk = min((h for h in valid if h["bytes_per_point"] > 0), key=lambda h: h["bytes_per_point"], default=None)
        best_cold = min((h for h in valid if h["cold_median_ms"] is not None), key=lambda h: h["cold_median_ms"], default=None)
        parts = [f"<b>{E(DB_LABEL[best_pps['db']])}</b> sustained the highest ingest at {fmt_si(best_pps['sustained_pps'])} points/s"]
        if best_disk:
            parts.append(f"<b>{E(DB_LABEL[best_disk['db']])}</b> used the least disk at {best_disk['bytes_per_point']:.2f} bytes per point")
        if best_cold:
            parts.append(f"<b>{E(DB_LABEL[best_cold['db']])}</b> answered the cold routine queries fastest (median {fmt_ms(best_cold['cold_median_ms'])})")
        H.append("<p>" + "; ".join(parts) + ".</p>")
        H.append('<div class="kpis">')
        for h in valid:
            H.append(f'<div class="kpi" style="--kpi:{DB_COLOR.get(h["db"], "#888")}"><div class="name">{E(DB_LABEL.get(h["db"], h["db"]))}</div><dl>'
                     f'<dt>sustained</dt><dd>{fmt_si(h["sustained_pps"])}/s</dd>'
                     f'<dt>bytes/point</dt><dd>{h["bytes_per_point"]:.2f}</dd>'
                     f'<dt>cold median</dt><dd>{fmt_ms(h["cold_median_ms"])}</dd>'
                     f'<dt>stored</dt><dd>{fmt_si(h["acked_points"])}</dd>'
                     f'<dt>outcome</dt><dd>{E(h["outcome"])}</dd></dl></div>')
        H.append("</div>")
    H.append(svgs["headline"])
    H.append('<div class="grid">')
    H.append("<div>" + svgs["disk-per-point"] + "</div>")
    H.append("<div>" + svgs["cold"] + "</div>")
    H.append("</div>")
    H.append('<div class="tablewrap"><table><thead><tr><th>Store</th><th>Peak points/s</th><th>Sustained points/s</th><th>Points stored</th><th>Bytes/point</th><th>vs raw 16 B</th><th>vs logical</th><th>Disk</th><th>Peak RSS</th><th>Settle</th><th>Fell behind at</th><th>Unusable at</th><th>Outcome</th></tr></thead><tbody>')
    for h in heads:
        if h["stub"]:
            H.append(f'<tr><td>{db_name(h["db"])}</td><td colspan="11" class="wrap bad">{E(h["outcome"])}: {E(h.get("error") or "")}</td><td>{E(h["outcome"])}</td></tr>')
            continue
        fb = f"{fmt_si(h['fell_behind_points'])} pts ({fmt_secs(h['fell_behind_at_s'])})" if h["fell_behind_at_s"] is not None else '<span class="ok">no</span>'
        un = f'<span class="bad">{fmt_si(h["unusable_points"])} pts ({fmt_secs(h["unusable_at_s"])})</span>' if h["unusable_at_s"] is not None else '<span class="ok">no</span>'
        settle = (fmt_secs(h["settle_s"]) + ("" if h["settled"] else ' <span class="warn">(cap)</span>')) if h["settle_s"] is not None else "-"
        stored = fmt_si(h["acked_points"]) + (' <span class="warn">(cap)</span>' if h["hit_max_ingest"] else "")
        H.append(f'<tr><td>{db_name(h["db"])}</td><td>{fmt_si(h["peak_pps"])}</td><td><b>{fmt_si(h["sustained_pps"])}</b></td><td>{stored}</td><td><b>{h["bytes_per_point"]:.2f}</b></td>'
                 f'<td>{h["ratio_raw16"]:.1f}x</td><td>{h["ratio_logical"]:.0f}x</td><td>{fmt_bytes(h["disk_bytes"])}</td><td>{fmt_bytes(h["peak_rss"]) if h["peak_rss"] else "-"}</td><td>{settle}</td><td>{fb}</td><td>{un}</td><td>{E(h["outcome"])}</td></tr>')
    H.append("</tbody></table></div>")
    H.append("<h3>Routine queries</h3>")
    H.append(f"<p>Seven dashboard-shaped queries rotate one at a time every 15 s during ingest (p95 of those below, threshold {fmt_ms(slow_ms)}), then run three times each after settle with the database restarted and the page cache dropped (first run cold, then the warm median).</p>")
    H.append('<div class="tablewrap"><table><thead><tr><th>Query</th><th class="wrap">Intent</th>' + "".join(f"<th>{db_name(h['db'])}<br><span class='pill'>ingest p95 / cold / warm</span></th>" for h in valid) + "</tr></thead><tbody>")
    for q in QUERY_IDS:
        cells = []
        for h in valid:
            qi = h["q_ingest"].get(q) or {}
            qc = h["q_cold"].get(q) or {}
            ing = fmt_ms(qi.get("p95_ms")) if qi else "-"
            extra = []
            if qi.get("timeouts"):
                extra.append(f"{qi['timeouts']} timeouts")
            if qi.get("errors"):
                extra.append(f"{qi['errors']} errors")
            if qi.get("suspect"):
                extra.append(f"{qi['suspect']} suspect")
            cold = f"{fmt_ms(qc.get('first_ms'))} / {fmt_ms(qc.get('warm_median_ms'))}" if qc.get("status") == "ok" else f'<span class="bad">{E(qc.get("status") or "-")}</span>'
            cls = "bad" if (qi.get("p95_ms") or 0) > slow_ms or qi.get("timeouts") else ""
            cells.append(f'<td><span class="{cls}">{ing}</span> / {cold}' + (f'<br><small class="warn">{E(", ".join(extra))}</small>' if extra else "") + "</td>")
        H.append(f'<tr><td><b>{q}</b></td><td class="wrap">{E(QUERY_INTENT[q])}</td>{"".join(cells)}</tr>')
    H.append("</tbody></table></div>")
    H.append(svgs["ingest-queries"])
    notes = []
    for h in valid:
        for k in ("fell_behind_reason", "unusable_reason"):
            if h.get(k):
                notes.append(f"<li>{db_name(h['db'])}: {E(k.replace('_reason', '').replace('_', ' '))}: {E(h[k])}</li>")
        if h["restarts"]:
            notes.append(f"<li>{db_name(h['db'])}: the database process restarted {h['restarts']}x during the run</li>")
        if h["oom_killed"]:
            notes.append(f"<li>{db_name(h['db'])}: the container was OOM-killed</li>")
        if h["points_lost"]:
            notes.append(f"<li>{db_name(h['db'])}: {fmt_si(h['points_lost'])} points were lost to writes that failed after retries</li>")
    if notes:
        H.append("<h3>What went wrong where</h3><ul>" + "".join(notes) + "</ul>")

    H.append("<h2>Over the run</h2>")
    H.append(svgs["throughput"])
    H.append('<div class="grid">')
    H.append("<div>" + svgs["disk"] + "</div><div>" + svgs["rss"] + "</div>")
    H.append("</div>")
    H.append(svgs["lag"])
    H.append("<h3>Each query during ingest</h3>")
    H.append('<div class="grid">')
    for q in QUERY_IDS:
        H.append("<div>" + svgs[f"query-{q}"] + "</div>")
    H.append("</div>")

    H.append("<h2>How each store is measured</h2>")
    H.append("<p>One Rust binary generates the same OpenTelemetry-shaped metric stream in memory (already parsed: metric name, resource and data-point attributes, a double value, a millisecond timestamp) and pushes it into each store through that store's most efficient native write path on the same machine. Nothing is decoded from OTLP and nothing crosses a network: the cost being measured is the database's, not the transport's.</p>")
    H.append('<div class="tablewrap"><table><thead><tr><th>Store</th><th class="wrap">Ingest path</th><th class="wrap">Schema</th><th class="wrap">Settle step before measuring disk</th><th class="wrap">Disk measure</th><th class="wrap">A write counts as acknowledged when</th></tr></thead><tbody>')
    for h in heads:
        how = DB_HOW.get(h["db"], ("", "", "", ""))
        ack = next((r.get("ack_semantics", "") for r in runs if r["db"] == h["db"]), "")
        H.append(f'<tr><td>{db_name(h["db"])}</td><td class="wrap">{E(how[0])}</td><td class="wrap">{E(how[1])}</td><td class="wrap">{E(how[2])}</td><td class="wrap">{E(how[3])}</td><td class="wrap">{E(ack)}</td></tr>')
    H.append("</tbody></table></div>")
    H.append("<h3>The workload</h3>")
    if sm:
        H.append('<div class="card"><dl class="facts">')
        H.append(f"<dt>Series</dt><dd>{fmt_si(sm.get('series', 0), 0)}: {sm.get('hosts')} hosts running the hostmetrics receiver (cpu x state, memory, disk x direction, filesystem x mountpoint, network x interface, load, paging, processes), {sm.get('pods')} Kubernetes pods (8 per host: pod and container cpu, memory, network, filesystem) and one HTTP service per pod (http.server.duration by route and method, active requests, process metrics), {sm.get('templates')} metric templates in total</dd>")
        H.append(f"<dt>Attributes</dt><dd>{sm.get('avg_tags_per_series', 0):.1f} per series on average, the same resource attributes Traceway's OTLP converter keeps (host.name, host.id, os.type, cloud.region, cloud.availability_zone, k8s.cluster.name, k8s.namespace.name, k8s.node.name, k8s.pod.name, container.name, process.pid, server_name) plus the metric's own dimensions</dd>")
        H.append(f"<dt>Values</dt><dd>bounded random walks for gauges, monotonic gamma-distributed increments for counters, constants for limits, rare bursts for error counters, spiky latencies for HTTP; 5% of hosts are quiet. Values are quantised the way real exporters emit them, so compression sees realistic data, not white noise</dd>")
        H.append(f"<dt>Time</dt><dd>{sm.get('rounds')} scrape rounds at {sm.get('interval_ms', 0)/1000:.0f}s in simulated time from {E(sm.get('sim_start', ''))} to {E(sm.get('sim_end', ''))}; every series of a host shares the host's timestamp and a fixed per-host jitter. The bench pushes as fast as the store accepts, so simulated time runs far ahead of wall clock</dd>")
        H.append(f"<dt>Points planned</dt><dd>{fmt_si(sm.get('points_planned', 0))} ({sm.get('rounds')} rounds x {fmt_si(sm.get('series', 0), 0)} series), {sm.get('logical_bytes_per_point', 0):.0f} logical bytes per point as Traceway's converter holds it</dd>")
        H.append(f"<dt>Generator</dt><dd>{bench.get('gen_threads')} threads, deterministic from seed {sm.get('seed')}; the value sequence of a series depends only on (seed, series id, round), so every store receives byte-identical data" + (f", proven by the identical fingerprint <code>{E(next(iter(fps)))}</code>" if len(fps) == 1 else "") + "</dd>")
        H.append("</dl></div>")
    H.append("<h3>What the numbers mean</h3>")
    H.append("<ul>")
    H.append("<li><b>Sustained points/s</b>: median acknowledged rate over 10 s windows from minute 5 of ingest until the store fell behind (or the end). <b>Peak</b>: best 60 s rolling mean.</li>")
    H.append("<li><b>Bytes per point</b>: data directory size after the store's own settle step (merges idle, WAL checkpointed, vacuumed) divided by acknowledged points. <b>vs raw 16 B</b>: compression against an 8 byte timestamp plus 8 byte value; <b>vs logical</b>: against the size of the point with its name and attributes spelled out.</li>")
    H.append("<li><b>Fell behind</b>: three consecutive windows where the visibility lag (latest acknowledged timestamp minus the latest timestamp a query can see, via a sentinel series) exceeded the threshold and kept growing, or throughput dropped under half its plateau, or a routine query breached the threshold, or writes failed. Ingest continues; the point count at that moment is reported.</li>")
    H.append("<li><b>Unusable</b>: three consecutive windows where every query timed out or failed, nothing was acknowledged while data was offered, more than half the writes failed, or the process was unreachable or restarted. Ingest runs on for a grace period to show whether it recovers, then stops.</li>")
    H.append("<li><b>Queries</b>: identical intents per store (SQL on ClickHouse, DuckDB and Firebolt; MetricsQL on VictoriaMetrics), same time windows relative to the latest visible timestamp, same 30 s deadline. A result with fewer than half the expected rows is marked <i>suspect</i> rather than counted as fast.</li>")
    H.append("</ul>")

    H.append("<h2>Why the comparison is fair</h2>")
    H.append("<ul>")
    cs = next((v.get("cgroup") for v in dbc.values() if v.get("cgroup")), None)
    H.append(f"<li><b>Same box, same split.</b> Every store runs alone on an identical {E(tier)} machine. The database container is pinned to all cores but two and capped with a cgroup memory limit; the bench runs on the remaining two. DuckDB is in-process, so it gets the same core count as a thread limit instead. RSS and CPU are read from the container's cgroup{' (' + E(cs) + ')' if cs else ''}, or split by thread for DuckDB.</li>")
    H.append("<li><b>Same data.</b> One seeded generator, one fingerprint. Loading the series dimension table is timed separately and excluded from throughput for every store; VictoriaMetrics has no such table, so its per-sample label cost is counted as its ingest cost, because that is its model.</li>")
    H.append("<li><b>Each store gets its own best practice, not a lowest common denominator.</b> Batch sizes follow each vendor's guidance (20k samples per remote-write request, 500k row RowBinary inserts, 1M row appends, 1M row Parquet uploads). ClickHouse gets explicit time-series codecs because it does not choose them itself; DuckDB, Firebolt and VictoriaMetrics pick their own encodings. Firebolt Core has no auto-vacuum, so the bench vacuums for it and counts the time.</li>")
    H.append("<li><b>Maintenance is inside the measurement.</b> ClickHouse merges, VictoriaMetrics merges, DuckDB checkpoints and Firebolt vacuums all happen while ingest runs and show up as throughput dips and write latency. No OPTIMIZE FINAL or other one-off compaction is run before measuring disk, only what the store does on its own plus a flush.</li>")
    H.append("<li><b>Backpressure, not a fixed rate.</b> The generator blocks when the store cannot keep up, so the measured rate is what the store accepts, and a store that acknowledges data before it is queryable is caught by the visibility lag rather than credited for buffering.</li>")
    H.append("<li><b>The bench watches itself.</b> If writers sit idle while the generator keeps up, the run is flagged as bench-limited instead of reported as the store's ceiling.</li>")
    H.append("<li><b>Metric-major series ids</b> (name = a contiguous id range) are what an ingester with a series dictionary produces; they give the three SQL stores identical primary-key pruning. The discovery queries deliberately show where an inverted index (VictoriaMetrics) beats a fact table scan; that is a difference between the stores, not a bias in the bench.</li>")
    H.append("</ul>")

    H.append("<h2>Per-store details</h2>")
    for r in runs:
        h = next((x for x in heads if x["db"] == r["db"] and x["tier"] == r["tier"]), None)
        H.append(f'<div class="card"><h3 style="margin-top:0">{db_name(r["db"])}</h3>')
        if r.get("stub"):
            H.append(f"<p class='bad'>{E(r.get('verdict', {}).get('stopped_reason', ''))}: {E(r.get('error') or '')}</p></div>")
            continue
        cfg = r.get("db_config", {})
        H.append('<dl class="facts">')
        H.append(f"<dt>Run</dt><dd><code>{E(r.get('run_id', ''))}</code>, {E(r.get('started_at', ''))} to {E(r.get('ended_at') or '')}</dd>")
        H.append(f"<dt>Writers / batch</dt><dd>{r.get('bench', {}).get('writers')} writers x {fmt_si(r.get('bench', {}).get('batch_points', 0), 0)} points</dd>")
        H.append(f"<dt>Settings</dt><dd><code>{E(json.dumps(cfg.get('settings', {}), sort_keys=True))}</code></dd>")
        if cfg.get("container"):
            H.append(f"<dt>Container</dt><dd>{E(cfg['container'])}</dd>")
        ph = r.get("phases", {})
        H.append(f"<dt>Phases</dt><dd>setup {ph.get('setup_ms', 0)/1000:.0f}s, series load {r.get('series_model', {}).get('catalog_load_ms', 0)/1000:.0f}s, warmup {ph.get('warmup_s', 0)}s, ingest {fmt_secs(ph.get('ingest_s', 0))}, drain {ph.get('drain_ms', 0)/1000:.0f}s, settle {fmt_secs((ph.get('settle') or {}).get('ms', 0)/1000)}, cold {ph.get('cold_ms', 0)/1000:.0f}s</dd>")
        st = (ph.get("settle") or {}).get("steps") or []
        if st:
            H.append("<dt>Settle steps</dt><dd>" + "; ".join(f"{E(s['name'])} {'ok' if s['ok'] else 'FAILED'} {s['ms']/1000:.0f}s {E(s.get('detail', ''))}" for s in st) + "</dd>")
        dr = r.get("disk", {}).get("db_reported") or {}
        if any(dr.get(k) for k in ("compressed_bytes", "uncompressed_bytes", "rows")):
            H.append(f"<dt>Store-reported size</dt><dd>compressed {fmt_bytes(dr.get('compressed_bytes'))}, uncompressed {fmt_bytes(dr.get('uncompressed_bytes'))}, rows {fmt_si(dr.get('rows') or 0)}</dd>")
        bc = r.get("disk", {}).get("by_class") or {}
        if bc:
            H.append("<dt>Disk by class</dt><dd>" + ", ".join(f"{E(k)} {fmt_bytes(v)}" for k, v in bc.items()) + "</dd>")
        if r.get("notes"):
            H.append("<dt>Notes</dt><dd>" + "; ".join(E(n) for n in r["notes"]) + "</dd>")
        H.append("</dl></div>")

    H.append("<footer>Generated by <code>benchmarks/metricsdb/scripts/summarize.py</code> from the per-store <code>&lt;tier&gt;-&lt;db&gt;.json</code> results written by <code>metricsdb-bench</code>. Raw JSON, logs and PNG charts are in the workflow artifact.</footer>")
    H.append("</main>")
    return "\n".join(H) + "\n"


def main(argv):
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    d = argv[1]
    runs = load_runs(d)
    if not runs:
        print(f"no result JSON in {d}", file=sys.stderr)
        with open(os.path.join(d, "summary.md"), "w") as f:
            f.write("# Metrics store benchmark\n\nNo result files were produced.\n")
        return 1
    tiers = sorted({r["tier"] for r in runs})
    md_parts = []
    html_parts = []
    for tier in tiers:
        tr = [r for r in runs if r["tier"] == tier]
        heads = [headline(r) for r in tr]
        slow_ms = 5000
        svgs, S = build_svgs(tr, heads, tier, slow_ms)
        for name, svg in svgs.items():
            with open(os.path.join(d, f"chart-{tier}-{name}.svg"), "w") as f:
                f.write('<?xml version="1.0" encoding="UTF-8"?>\n' + svg.replace('class="chart"', 'style="background:#fff;color:#111"') + "\n")
        pngs = write_pngs(S, heads, tier, d)
        md_parts.append(render_summary_md(tr, heads, tier, pngs))
        html_parts.append(render_html(tr, heads, tier, svgs, slow_ms))
    with open(os.path.join(d, "summary.md"), "w") as f:
        f.write("\n".join(md_parts))
    with open(os.path.join(d, "report.html"), "w") as f:
        f.write("\n".join(html_parts))
    print(f"wrote {os.path.join(d, 'summary.md')} and report.html for tiers {', '.join(tiers)}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
