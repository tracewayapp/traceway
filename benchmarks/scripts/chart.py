#!/usr/bin/env python3
"""Render benchmark charts from a directory of loadgen JSONs.

Inputs: a directory containing N JSON files (one per matrix entry), each in
the schema emitted by benchmarks/loadgen/. Files without a "signal" field
(old pre-OTLP-split format) are skipped with a log line.

The script auto-detects scenarios per file (each JSON has `scenario`). Run it
against `benchmarks/results-throughput/` for throughput charts, or
`benchmarks/results-probe/` for read-probe charts. Mixed folders also work —
each render function filters to its own scenario.

See benchmarks/charts.md for a reader's guide to each output.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import Patch, Rectangle


TIER_ORDER = ["ccx13", "ccx23", "ccx33", "ccx43"]
MODE_ORDER = ["sqlite", "pgch", "managed-ch"]
SIGNALS = ["spans", "metrics", "logs"]

MODE_COLORS = {
    "sqlite": "#4f9fff",
    "pgch": "#ff9f4f",
    "managed-ch": "#7c4fff",
}
SIGNAL_COLORS = {
    "spans": "#1f77b4",
    "metrics": "#2ca02c",
    "logs": "#d62728",
}
PHASE_LABEL = {"phase1": "P1", "phase2": "P2", "phase3": "P3"}

ITEM_LABEL = {
    "spans": "spans/sec",
    "metrics": "data points/sec",
    "logs": "log records/sec",
}
ROW_LABEL = {
    "spans": "spans rows",
    "metrics": "metric_points rows",
    "logs": "log_records rows",
}
TIER_META = {
    "ccx13": "2 vCPU / 8 GB",
    "ccx23": "4 vCPU / 16 GB",
    "ccx33": "8 vCPU / 32 GB",
    "ccx43": "16 vCPU / 64 GB",
}
TIER_VCPU = {"ccx13": 2, "ccx23": 4, "ccx33": 8, "ccx43": 16}

# Match loadgen defaults so chart thresholds line up with the JSON's `passed` bit.
SOFT_CLIFF_RATIO = 0.7
ERR_THRESHOLD_PCT = 5.0

STATUS_COLOR = {
    "pass": "#4caf50",
    "soft-cliff": "#ffc107",
    "hard-fail": "#f44336",
    "dead": "#616161",
    "missing": "#f5f5f5",
}


# ---------- Loading + small helpers ----------


def load_runs(results_dir: Path) -> list[dict]:
    runs = []
    for p in sorted(results_dir.glob("*.json")):
        with p.open() as fh:
            doc = json.load(fh)
        if "signal" not in doc:
            print(f"skipping {p.name}: missing 'signal' field (pre-OTLP-split format)", file=sys.stderr)
            continue
        if doc["signal"] not in SIGNALS:
            print(f"skipping {p.name}: unknown signal {doc['signal']!r}", file=sys.stderr)
            continue
        doc.setdefault("scenario", "throughput")
        if doc["scenario"] not in ("throughput", "read-probe"):
            print(f"skipping {p.name}: unknown scenario {doc['scenario']!r}", file=sys.stderr)
            continue
        doc["_path"] = p
        runs.append(doc)
    return runs


def tier_rank(t: str) -> int:
    return TIER_ORDER.index(t) if t in TIER_ORDER else len(TIER_ORDER)


def mode_rank(m: str) -> int:
    return MODE_ORDER.index(m) if m in MODE_ORDER else len(MODE_ORDER)


def mode_color(m: str) -> str:
    return MODE_COLORS.get(m, "#777")


def read_path_for_signal(signal: str) -> str:
    return {
        "spans": "/api/endpoints/grouped",
        "metrics": "/api/metrics/application",
        "logs": "/api/logs",
    }.get(signal, "")


def fmt_count(n: float) -> str:
    n = int(n or 0)
    if n >= 1_000_000_000:
        return f"{n/1_000_000_000:.1f}B"
    if n >= 1_000_000:
        return f"{n/1_000_000:.1f}M"
    if n >= 1_000:
        return f"{n/1_000:.0f}K"
    return f"{n}"


def run_label(run: dict) -> str:
    return f"{run['tier']} / {run['mode']}"


def iter_runs(runs: list[dict], signal: str, scenario: str) -> list[dict]:
    out = [r for r in runs if r["signal"] == signal and r.get("scenario", "throughput") == scenario]
    return sorted(out, key=lambda r: (tier_rank(r["tier"]), mode_rank(r["mode"])))


def phase_steps(run: dict, phase_key: str, sort_by: str = "step") -> list[dict]:
    """Return phase steps in a chosen sort order. Phase 2/3 record execution
    order, not rate order, so sorting before plotting keeps lines monotonic."""
    phase = run.get(phase_key) or {}
    steps = phase.get("steps") or []
    if not steps:
        return []
    if sort_by == "rate":
        return sorted(steps, key=lambda s: (s.get("requestRate", 0), s.get("step", 0)))
    if sort_by == "batch":
        return sorted(steps, key=lambda s: (s.get("batchSize", 0), s.get("step", 0)))
    return sorted(steps, key=lambda s: s.get("step", 0))


def step_status(step: dict) -> str:
    """Categorize a step as pass / soft-cliff / hard-fail / dead.
    `pass` = loadgen marked it passed.
    `dead` = no requests completed (actual=0 or attempted=0).
    `soft-cliff` = achieved fell below SOFT_CLIFF_RATIO × attempted with no error spike.
    `hard-fail` = combined error rate above threshold (default 5%).
    """
    if step.get("passed"):
        return "pass"
    actual = step.get("actualItemsPerSec", 0) or 0
    attempted = step.get("attemptedItemsPerSec", 0) or 0
    err_rate_pct = ((step.get("ingest") or {}).get("errRate", 0) or 0) * 100
    if attempted == 0 or actual == 0:
        return "dead"
    if err_rate_pct > ERR_THRESHOLD_PCT:
        return "hard-fail"
    if attempted > 0 and actual / attempted < SOFT_CLIFF_RATIO:
        return "soft-cliff"
    return "hard-fail"


def headline_source(run: dict) -> tuple[str, dict] | None:
    """Return (phase_key, step) whose actualItemsPerSec equals the headline.
    Returns None if the run has no passing step."""
    headline = run.get("maxSustainableItemsPerSec", 0) or 0
    if headline <= 0:
        return None
    best = None
    best_delta = float("inf")
    for phase_key in ("phase1", "phase2", "phase3"):
        for step in phase_steps(run, phase_key):
            if not step.get("passed"):
                continue
            actual = step.get("actualItemsPerSec", 0) or 0
            delta = abs(actual - headline)
            if delta < best_delta:
                best = (phase_key, step)
                best_delta = delta
    return best


def first_failure(run: dict) -> tuple[str, dict] | None:
    for phase_key in ("phase1", "phase2", "phase3"):
        for step in phase_steps(run, phase_key):
            if not step.get("passed"):
                return (phase_key, step)
    return None


def phase1_p99_at_batch(run: dict, batch_size: int) -> float | None:
    for step in phase_steps(run, "phase1"):
        if step.get("batchSize") == batch_size and step.get("passed"):
            return (step.get("ingest") or {}).get("p99")
    return None


def worst_phase_for_run(run: dict) -> str | None:
    """Pick the phase that produced the lowest passing actualItemsPerSec, or the
    phase that first failed. Used by the CH-pressure chart so the visualisation
    targets the phase where the SUT actually struggled."""
    failing = first_failure(run)
    if failing:
        return failing[0]
    worst_phase = None
    worst_value = float("inf")
    for phase_key in ("phase1", "phase2", "phase3"):
        for step in phase_steps(run, phase_key):
            if not step.get("passed"):
                continue
            actual = step.get("actualItemsPerSec", 0) or 0
            if actual < worst_value:
                worst_value = actual
                worst_phase = phase_key
    return worst_phase


# ---------- Throughput charts ----------


def render_headline_bar(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = iter_runs(runs, signal, "throughput")
    if not sig_runs:
        return

    tiers_present = sorted({r["tier"] for r in sig_runs}, key=tier_rank)
    modes_present = sorted({r["mode"] for r in sig_runs}, key=mode_rank)
    if not tiers_present or not modes_present:
        return

    fig, ax = plt.subplots(figsize=(max(8, len(tiers_present) * 2.5), 6))

    values = [r.get("maxSustainableItemsPerSec", 0) or 0 for r in sig_runs]
    ymax = max(values) if values else 1
    ghost_height = max(ymax * 0.02, 1)

    width = 0.8 / max(len(modes_present), 1)
    x_centers = list(range(len(tiers_present)))

    any_ch_restart = False
    for mi, mode in enumerate(modes_present):
        for ti, tier in enumerate(tiers_present):
            match = [r for r in sig_runs if r["tier"] == tier and r["mode"] == mode]
            x = ti + (mi - (len(modes_present) - 1) / 2) * width
            if not match:
                continue
            run = match[0]
            value = run.get("maxSustainableItemsPerSec", 0) or 0
            ch_restart = bool(run.get("chRestarted"))
            if ch_restart:
                any_ch_restart = True
            if value > 0:
                bar_kwargs = {"width": width, "color": mode_color(mode)}
                if ch_restart:
                    bar_kwargs.update({"edgecolor": "#aa3333", "hatch": "xx", "linewidth": 1.2})
                ax.bar(x, value, **bar_kwargs)
                src = headline_source(run)
                phase_tag = f" ({PHASE_LABEL[src[0]]})" if src else ""
                restart_tag = " CH↻" if ch_restart else ""
                ax.annotate(
                    f"{int(value):,}{phase_tag}{restart_tag}",
                    xy=(x, value), xytext=(0, 4), textcoords="offset points",
                    ha="center", va="bottom", fontsize=8,
                    color="#aa3333" if ch_restart else "black",
                )
            else:
                ax.bar(x, ghost_height, width=width, color="#dddddd",
                       edgecolor="#999999", hatch="///", linewidth=0.7)
                fail = first_failure(run)
                if fail:
                    pkey, step = fail
                    label = f"failed @ {PHASE_LABEL[pkey]} step {step.get('step', '?')}"
                else:
                    label = "failed"
                ax.annotate(
                    label, xy=(x, ghost_height), xytext=(0, 4), textcoords="offset points",
                    ha="center", va="bottom", fontsize=7, color="#aa3333",
                )

    ax.set_xticks(x_centers)
    ax.set_xticklabels(tiers_present)
    ax.set_ylabel(f"Max sustainable {ITEM_LABEL[signal]}")
    ax.set_title(
        f"Traceway: max sustainable {signal} ingest by hardware tier\n"
        "Phase tag (P1/P2/P3) shows which workload shape produced the headline"
    )

    legend_handles = [Patch(facecolor=mode_color(m), label=m) for m in modes_present]
    legend_handles.append(Patch(facecolor="#dddddd", edgecolor="#999999", hatch="///", label="all phases failed"))
    if any_ch_restart:
        legend_handles.append(Patch(facecolor="white", edgecolor="#aa3333", hatch="xx", label="CH restart during run (result invalid)"))
    ax.legend(handles=legend_handles, title="DB mode", loc="upper left")

    ax.grid(axis="y", linestyle=":", alpha=0.4)
    ax.set_ylim(0, ymax * 1.3 if ymax > 0 else 1)

    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_phase1_latency(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = [r for r in iter_runs(runs, signal, "throughput") if phase_steps(r, "phase1")]
    if not sig_runs:
        return

    fig, ax = plt.subplots(figsize=(11, 6))
    plotted = False

    for run in sig_runs:
        color = mode_color(run["mode"])
        steps = phase_steps(run, "phase1", sort_by="batch")
        batches = [s["batchSize"] for s in steps]
        p50 = [(s.get("ingest") or {}).get("p50", 0) for s in steps]
        p95 = [(s.get("ingest") or {}).get("p95", 0) for s in steps]
        p99 = [(s.get("ingest") or {}).get("p99", 0) for s in steps]
        passed = [bool(s.get("passed")) for s in steps]

        def _safe(xs, ys):
            return [(x, y) for x, y in zip(xs, ys) if y and y > 0]

        for percs, ls, alpha in [(p50, ":", 0.5), (p95, "-.", 0.7), (p99, "-", 1.0)]:
            pts = _safe(batches, percs)
            if not pts:
                continue
            xs, ys = zip(*pts)
            label = run_label(run) if ls == "-" else None
            ax.plot(xs, ys, linestyle=ls, color=color, alpha=alpha, label=label)
            plotted = True

        for b, lat, p in zip(batches, p99, passed):
            if lat and lat > 0:
                ax.plot(b, lat,
                        marker=("o" if p else "x"),
                        color=color, markersize=8,
                        markeredgewidth=2 if not p else 1, linestyle="")

    if not plotted:
        plt.close(fig)
        return

    ax.set_xlabel("Batch size (items/request, log)")
    ax.set_ylabel("Ingest latency (ms, log)\nP50 ··  P95 -·  P99 —")
    ax.set_xscale("log")
    ax.set_yscale("log")
    ax.set_title(
        f"Phase 1 — batch-size scaling — {signal}\n"
        "Fixed 5 req/sec.  Solid line = P99 per run, dot/dash = P50/P95.  × = step failed."
    )
    ax.grid(True, which="both", linestyle=":", alpha=0.4)
    ax.legend(loc="upper left", fontsize=9, title="P99 line per run")
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def _render_rate_ramp(runs: list[dict], signal: str, phase_key: str, subtitle: str, out: Path) -> None:
    sig_runs = [r for r in iter_runs(runs, signal, "throughput") if phase_steps(r, phase_key)]
    if not sig_runs:
        return

    fig, (ax1, ax2, ax3) = plt.subplots(3, 1, figsize=(11, 12), sharex=True)
    plotted = False

    for run in sig_runs:
        color = mode_color(run["mode"])
        label = run_label(run)
        steps = phase_steps(run, phase_key, sort_by="rate")

        rates = [s["requestRate"] for s in steps]
        actual = [s["actualItemsPerSec"] for s in steps]
        attempted = [s["attemptedItemsPerSec"] for s in steps]
        p50 = [(s.get("ingest") or {}).get("p50", 0) for s in steps]
        p95 = [(s.get("ingest") or {}).get("p95", 0) for s in steps]
        p99 = [(s.get("ingest") or {}).get("p99", 0) for s in steps]
        errs_pct = [((s.get("ingest") or {}).get("errRate", 0) or 0) * 100 for s in steps]
        passed = [bool(s.get("passed")) for s in steps]

        # Top: throughput. Drop zeros for log scale.
        act_pts = [(r, a) for r, a in zip(rates, actual) if a and a > 0]
        att_pts = [(r, a) for r, a in zip(rates, attempted) if a and a > 0]
        if act_pts:
            xs, ys = zip(*act_pts)
            ax1.plot(xs, ys, linestyle="-", color=color, label=label)
            plotted = True
        if att_pts:
            xs, ys = zip(*att_pts)
            ax1.plot(xs, ys, linestyle="--", color=color, alpha=0.4)
        for r, a, p in zip(rates, actual, passed):
            if a and a > 0:
                ax1.plot(r, a, marker=("o" if p else "x"), color=color,
                         markersize=8, markeredgewidth=2 if not p else 1, linestyle="")

        # Middle: latency percentiles.
        for percs, ls, alpha in [(p50, ":", 0.5), (p95, "-.", 0.7), (p99, "-", 1.0)]:
            pts = [(r, l) for r, l in zip(rates, percs) if l and l > 0]
            if pts:
                xs, ys = zip(*pts)
                ax2.plot(xs, ys, linestyle=ls, color=color, alpha=alpha)
        # P1 floor overlay
        fixed_batch = (run.get(phase_key) or {}).get("fixedBatchSize", 0)
        if fixed_batch:
            floor = phase1_p99_at_batch(run, fixed_batch)
            if floor:
                ax2.axhline(floor, linestyle=":", color=color, alpha=0.35, linewidth=1)
        for r, lat, p in zip(rates, p99, passed):
            if lat and lat > 0:
                ax2.plot(r, lat, marker=("o" if p else "x"), color=color,
                         markersize=8, markeredgewidth=2 if not p else 1, linestyle="")

        # Bottom: error %
        ax3.plot(rates, errs_pct, linestyle="-", color=color, alpha=0.5)
        for r, e, p in zip(rates, errs_pct, passed):
            ax3.plot(r, e, marker=("o" if p else "x"), color=color,
                     markersize=8, markeredgewidth=2 if not p else 1, linestyle="")

        # Bottom right twin: rejected
        rej = [s.get("rejected", 0) for s in steps]
        if any(rej):
            if not hasattr(ax3, "_rej_twin"):
                ax3._rej_twin = ax3.twinx()
            ax3._rej_twin.plot(rates, rej, linestyle=":", color=color, alpha=0.4)

    if not plotted:
        plt.close(fig)
        return

    ax1.set_ylabel(f"items/sec (log)\n— actual,  -- attempted")
    ax1.set_xscale("log")
    ax1.set_yscale("log")
    ax1.grid(True, which="both", linestyle=":", alpha=0.4)
    ax1.legend(loc="lower right", fontsize=8)

    ax2.set_ylabel("Ingest latency (ms, log)\nP50 ··  P95 -·  P99 —\n(thin dotted = P1 floor)")
    ax2.set_yscale("log")
    ax2.grid(True, which="both", linestyle=":", alpha=0.4)

    ax3.axhline(ERR_THRESHOLD_PCT, linestyle="--", color="#cc3333", alpha=0.5, linewidth=1)
    ax3.set_ylabel("Error rate (%)\n× = failed step")
    ax3.set_xlabel("Request rate (req/sec, log)")
    ax3.set_xscale("log")
    ax3.grid(True, which="both", linestyle=":", alpha=0.4)
    if hasattr(ax3, "_rej_twin"):
        ax3._rej_twin.set_ylabel("OTLP rejected items (dotted)", color="#666666")

    fig.suptitle(f"{subtitle} — {signal}\n× marker = failed step (err > 5% or soft-cliff). Dotted P1 floor = no-concurrency latency.")
    fig.tight_layout(rect=(0, 0, 1, 0.97))
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_phase2_rate(runs: list[dict], signal: str, out: Path) -> None:
    _render_rate_ramp(runs, signal, "phase2",
                      "Phase 2 — collector-shape rate ramp (fixed fat batch)", out)


def render_phase3_rate(runs: list[dict], signal: str, out: Path) -> None:
    # Only render when at least one run has phase3 data.
    any_p3 = any(phase_steps(r, "phase3") for r in iter_runs(runs, signal, "throughput"))
    if not any_p3:
        return
    _render_rate_ramp(runs, signal, "phase3",
                      "Phase 3 — SDK-fleet rate ramp (small batch, high rate)", out)


def render_pareto(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = iter_runs(runs, signal, "throughput")
    if not sig_runs:
        return

    fig, ax = plt.subplots(figsize=(11, 7))
    plotted = False
    dead_runs = []  # (run_label, list_of_phase_keys+step_n)

    for run in sig_runs:
        color = mode_color(run["mode"])
        label = run_label(run)

        passed_pts = []
        failed_pts = []
        for phase_key in ("phase2", "phase3"):
            for step in phase_steps(run, phase_key):
                p99 = (step.get("ingest") or {}).get("p99", 0)
                actual = step.get("actualItemsPerSec", 0) or 0
                if p99 <= 0 or actual <= 0:
                    if step.get("passed") is False:
                        dead_runs.append((label, f"{PHASE_LABEL[phase_key]}s{step.get('step', '?')}"))
                    continue
                if step.get("passed"):
                    passed_pts.append((p99, actual))
                else:
                    failed_pts.append((p99, actual))

        if passed_pts:
            sorted_pts = sorted(passed_pts, key=lambda p: p[0])
            xs, ys = zip(*sorted_pts)
            ax.plot(xs, ys, linestyle=":", color=color, alpha=0.4)
            ax.scatter(xs, ys, marker="o", color=color, s=70, edgecolor="black",
                       linewidth=0.5, label=label, zorder=3)
            plotted = True
        if failed_pts:
            xs, ys = zip(*failed_pts)
            ax.scatter(xs, ys, marker="x", color=color, s=100, linewidth=2, zorder=4)
            plotted = True

    if not plotted and not dead_runs:
        plt.close(fig)
        return

    ax.set_xlabel("Ingest P99 latency (ms, log)")
    ax.set_ylabel(f"Achieved {ITEM_LABEL[signal]} (log)")
    if plotted:
        ax.set_xscale("log")
        ax.set_yscale("log")
    ax.set_title(
        f"Latency–throughput Pareto — {signal}\n"
        "Every Phase 2/3 step.  ○ = passed,  × = failed.  Up-and-left is better."
    )
    ax.grid(True, which="both", linestyle=":", alpha=0.4)

    # Annotate runs that had only dead (0-actual) failures
    if dead_runs:
        dead_by_run: dict[str, list[str]] = {}
        for lab, key in dead_runs:
            dead_by_run.setdefault(lab, []).append(key)
        text = "Zero-throughput failures: " + "; ".join(
            f"{lab}@{','.join(keys)}" for lab, keys in dead_by_run.items()
        )
        ax.text(0.5, -0.15, text, transform=ax.transAxes, ha="center", va="top",
                fontsize=8, color="#aa3333")
        fig.subplots_adjust(bottom=0.18)

    if plotted:
        ax.legend(loc="upper left", fontsize=9)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_tier_scaling(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = iter_runs(runs, signal, "throughput")
    if not sig_runs:
        return

    tiers_present = sorted({r["tier"] for r in sig_runs}, key=tier_rank)
    modes_present = sorted({r["mode"] for r in sig_runs}, key=mode_rank)
    if not tiers_present:
        return

    fig, ax = plt.subplots(figsize=(10, 6))
    plotted = False
    x_positions = list(range(len(tiers_present)))

    for mode in modes_present:
        xs, ys = [], []
        for i, tier in enumerate(tiers_present):
            match = [r for r in sig_runs if r["tier"] == tier and r["mode"] == mode]
            if not match:
                continue
            value = match[0].get("maxSustainableItemsPerSec", 0) or 0
            if value <= 0:
                continue
            xs.append(i)
            ys.append(value)
        if not xs:
            continue
        ax.plot(xs, ys, marker="o", color=mode_color(mode), markersize=9,
                linewidth=2, label=mode)
        for x, y in zip(xs, ys):
            ax.annotate(fmt_count(y), xy=(x, y), xytext=(0, 8),
                        textcoords="offset points", ha="center", va="bottom", fontsize=9)
        # Linear-vCPU reference, anchored at the smallest tier of THIS mode
        base_x = xs[0]
        base_y = ys[0]
        base_vcpu = TIER_VCPU.get(tiers_present[base_x], 1)
        ref_xs = list(range(len(tiers_present)))
        ref_ys = [base_y * (TIER_VCPU.get(tiers_present[i], 1) / base_vcpu) for i in ref_xs]
        ax.plot(ref_xs, ref_ys, linestyle="--", color=mode_color(mode), alpha=0.3, linewidth=1)
        plotted = True

    if not plotted:
        plt.close(fig)
        return

    ax.set_xticks(x_positions)
    ax.set_xticklabels([f"{t}\n{TIER_META.get(t, '')}" for t in tiers_present])
    ax.set_ylabel(f"Max sustainable {ITEM_LABEL[signal]}")
    ax.set_title(
        f"Tier scaling — {signal}\n"
        "Dashed = linear-vCPU reference (anchored at smallest present tier).  "
        "Distance below dashed = scaling penalty."
    )
    ax.legend(title="DB mode", loc="upper left")
    ax.grid(True, linestyle=":", alpha=0.4)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_cliff_grid(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = iter_runs(runs, signal, "throughput")
    if not sig_runs:
        return

    n_p1 = max((len(phase_steps(r, "phase1")) for r in sig_runs), default=0)
    n_p2 = max((len(phase_steps(r, "phase2")) for r in sig_runs), default=0)
    n_p3 = max((len(phase_steps(r, "phase3")) for r in sig_runs), default=0)
    total_cols = n_p1 + n_p2 + n_p3
    if total_cols == 0:
        return

    n_rows = len(sig_runs)
    fig_w = max(11, total_cols * 1.55 + 4)
    fig_h = max(4.5, n_rows * 0.95 + 2.2)
    fig, ax = plt.subplots(figsize=(fig_w, fig_h))

    for ri, run in enumerate(sig_runs):
        y = n_rows - ri - 1
        ax.text(-0.5, y + 0.5, run_label(run), ha="right", va="center", fontsize=10, fontweight="bold")

        col_x = 0
        for pk, n_cols in [("phase1", n_p1), ("phase2", n_p2), ("phase3", n_p3)]:
            steps = phase_steps(run, pk, sort_by="step")
            for ci in range(n_cols):
                x = col_x + ci
                if ci < len(steps):
                    step = steps[ci]
                    color = STATUS_COLOR[step_status(step)]
                    bs = step.get("batchSize", 0)
                    r = step.get("requestRate", 0)
                    rect = Rectangle((x, y), 1, 1, facecolor=color,
                                     edgecolor="white", linewidth=1.5)
                    ax.add_patch(rect)
                    ax.text(x + 0.5, y + 0.5, f"bs={fmt_count(bs)}\nr={r:g}",
                            ha="center", va="center", fontsize=7.5)
                else:
                    rect = Rectangle((x, y), 1, 1, facecolor=STATUS_COLOR["missing"],
                                     edgecolor="white", linewidth=1.5)
                    ax.add_patch(rect)
            col_x += n_cols

        headline = run.get("maxSustainableItemsPerSec", 0) or 0
        margin = total_cols + 0.35
        if headline > 0:
            ax.text(margin, y + 0.5, f"{fmt_count(headline)}/s",
                    ha="left", va="center", fontsize=10, fontweight="bold")
        else:
            ax.text(margin, y + 0.5, "failed",
                    ha="left", va="center", fontsize=10, color="#cc3333", fontweight="bold")

    # Phase headers + separators
    headers = []
    if n_p1 > 0:
        headers.append((n_p1 / 2.0, "Phase 1 (batch ramp)"))
    if n_p2 > 0:
        headers.append((n_p1 + n_p2 / 2.0, "Phase 2 (collector rate ramp)"))
    if n_p3 > 0:
        headers.append((n_p1 + n_p2 + n_p3 / 2.0, "Phase 3 (SDK-fleet rate ramp)"))
    for cx, label in headers:
        ax.text(cx, n_rows + 0.25, label, ha="center", va="bottom",
                fontsize=10.5, fontweight="bold")

    for sep in [n_p1, n_p1 + n_p2]:
        if 0 < sep < total_cols:
            ax.plot([sep, sep], [0, n_rows], color="black", linewidth=2)

    ax.set_xlim(-4, total_cols + 5)
    ax.set_ylim(-0.6, n_rows + 0.9)
    ax.set_aspect("equal")
    ax.axis("off")

    legend = [
        Patch(facecolor=STATUS_COLOR["pass"], label="pass"),
        Patch(facecolor=STATUS_COLOR["soft-cliff"], label=f"soft-cliff (<{int(SOFT_CLIFF_RATIO*100)}% achieved)"),
        Patch(facecolor=STATUS_COLOR["hard-fail"], label=f"hard fail (err > {ERR_THRESHOLD_PCT:.0f}%)"),
        Patch(facecolor=STATUS_COLOR["dead"], label="0 requests completed"),
        Patch(facecolor=STATUS_COLOR["missing"], edgecolor="#cccccc", label="not run"),
    ]
    ax.legend(handles=legend, loc="lower center", bbox_to_anchor=(0.5, -0.08),
              ncol=5, fontsize=9, frameon=False)
    fig.suptitle(f"Step-status grid — {signal}\nbs = batch size, r = request rate (req/sec)",
                 fontsize=12)
    fig.tight_layout(rect=(0, 0.04, 1, 0.95))
    fig.savefig(out, dpi=130, bbox_inches="tight")
    plt.close(fig)


def render_batch_efficiency(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = [r for r in iter_runs(runs, signal, "throughput") if phase_steps(r, "phase1")]
    if not sig_runs:
        return

    fig, ax = plt.subplots(figsize=(11, 6))
    plotted = False

    for run in sig_runs:
        color = mode_color(run["mode"])
        steps = phase_steps(run, "phase1", sort_by="batch")
        batches = [s["batchSize"] for s in steps]
        actual = [s["actualItemsPerSec"] for s in steps]
        attempted = [s["attemptedItemsPerSec"] for s in steps]
        passed = [bool(s.get("passed")) for s in steps]

        # log scale; drop zeros
        act_pts = [(b, a) for b, a in zip(batches, actual) if a and a > 0]
        att_pts = [(b, a) for b, a in zip(batches, attempted) if a and a > 0]

        if act_pts:
            xs, ys = zip(*act_pts)
            ax.plot(xs, ys, linestyle="-", color=color, marker="o", label=run_label(run))
            plotted = True
        if att_pts:
            xs, ys = zip(*att_pts)
            ax.plot(xs, ys, linestyle="--", color=color, alpha=0.35)

        for b, a, p in zip(batches, actual, passed):
            if not p and a and a > 0:
                ax.plot(b, a, marker="x", color=color, markersize=11,
                        markeredgewidth=2.2, linestyle="")

    if not plotted:
        plt.close(fig)
        return

    ax.set_xlabel("Batch size (items/request, log)")
    ax.set_ylabel(f"{ITEM_LABEL[signal]} (log)\n— actual,  -- attempted (batch × 5 req/s)")
    ax.set_xscale("log")
    ax.set_yscale("log")
    ax.set_title(
        f"Batch efficiency — {signal}\n"
        "Phase 1 at fixed 5 req/sec.  Solid below dashed = SUT dropping batches."
    )
    ax.grid(True, which="both", linestyle=":", alpha=0.4)
    ax.legend(loc="upper left", fontsize=9)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_ch_pressure(runs: list[dict], signal: str, out: Path) -> None:
    """For each (tier, mode) of a signal, plot CH parts count and active merges
    across the steps of its worst-performing phase. Makes the CH-side story —
    "parts piled up before the cliff" or "merges were idle and the cliff was
    backend-side" — visible at a glance."""
    sig_runs = iter_runs(runs, signal, "throughput")
    sig_runs = [r for r in sig_runs if any(
        (step.get("ch") or {}).get("reachable")
        for phase_key in ("phase1", "phase2", "phase3")
        for step in phase_steps(r, phase_key)
    )]
    if not sig_runs:
        return

    cols = min(len(sig_runs), 3)
    rows = (len(sig_runs) + cols - 1) // cols
    fig, axes = plt.subplots(rows, cols, figsize=(cols * 5.5, rows * 3.5), squeeze=False)

    for idx, run in enumerate(sig_runs):
        r, c = divmod(idx, cols)
        ax_parts = axes[r][c]
        ax_merges = ax_parts.twinx()

        phase_key = worst_phase_for_run(run)
        if not phase_key:
            ax_parts.set_visible(False)
            ax_merges.set_visible(False)
            continue
        steps = phase_steps(run, phase_key)
        if not steps:
            ax_parts.set_visible(False)
            ax_merges.set_visible(False)
            continue

        xs = [s.get("step", i + 1) for i, s in enumerate(steps)]
        parts = [(s.get("ch") or {}).get("partsCount", 0) for s in steps]
        merges = [(s.get("ch") or {}).get("activeMerges", 0) for s in steps]
        first_failed_step = next((s.get("step") for s in steps if not s.get("passed")), None)

        ax_parts.plot(xs, parts, marker="o", color="#1f77b4", label="parts")
        ax_merges.plot(xs, merges, marker="s", color="#d62728", linestyle="--", label="active merges")

        if first_failed_step is not None:
            ax_parts.axvline(first_failed_step, color="#aa3333", linestyle=":", alpha=0.6)

        title = f"{run_label(run)} — {PHASE_LABEL[phase_key]}"
        if run.get("chRestarted"):
            title += " (CH↻)"
        ax_parts.set_title(title, fontsize=10)
        ax_parts.set_xlabel("step")
        ax_parts.set_ylabel("parts (total, active)", color="#1f77b4")
        ax_merges.set_ylabel("active merges", color="#d62728")
        ax_parts.grid(True, linestyle=":", alpha=0.4)

    for idx in range(len(sig_runs), rows * cols):
        r, c = divmod(idx, cols)
        axes[r][c].set_visible(False)

    fig.suptitle(f"ClickHouse pressure during the worst-performing phase — {signal}", y=1.02)
    fig.tight_layout()
    fig.savefig(out, dpi=130, bbox_inches="tight")
    plt.close(fig)


def render_signal_mix(runs: list[dict], out: Path) -> None:
    """Cross-signal throughput comparison: bars per (tier, mode), one bar per signal."""
    thr_runs = [r for r in runs if r.get("scenario", "throughput") == "throughput"]
    if not thr_runs:
        return

    keys = sorted({(r["tier"], r["mode"]) for r in thr_runs},
                  key=lambda k: (tier_rank(k[0]), mode_rank(k[1])))
    if not keys:
        return

    fig, ax = plt.subplots(figsize=(max(8, len(keys) * 1.9), 6))

    width = 0.25
    x = list(range(len(keys)))
    any_value = False

    for si, signal in enumerate(SIGNALS):
        ys = []
        for tier, mode in keys:
            match = [r for r in thr_runs if r["tier"] == tier and r["mode"] == mode and r["signal"] == signal]
            ys.append((match[0].get("maxSustainableItemsPerSec", 0) or 0) if match else 0)
        offsets = [xi + (si - 1) * width for xi in x]
        bars = ax.bar(offsets, ys, width=width, color=SIGNAL_COLORS[signal], label=signal)
        for b, y in zip(bars, ys):
            if y > 0:
                ax.annotate(fmt_count(y), xy=(b.get_x() + b.get_width() / 2, y),
                            xytext=(0, 3), textcoords="offset points",
                            ha="center", va="bottom", fontsize=7)
                any_value = True

    if not any_value:
        plt.close(fig)
        return

    ax.set_xticks(x)
    ax.set_xticklabels([f"{t}\n{m}" for t, m in keys], fontsize=9)
    ax.set_ylabel("Max sustainable items/sec (Y unit varies by signal)")
    ax.set_title("Cross-signal comparison — per (tier × mode), spans vs metrics vs logs")
    ax.legend(title="Signal")
    ax.grid(axis="y", linestyle=":", alpha=0.4)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


# ---------- Read-probe charts ----------


def _readprobe_steps(run: dict) -> list[dict]:
    return ((run.get("readProbe") or {}).get("steps") or [])


def render_readprobe_headline(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = iter_runs(runs, signal, "read-probe")
    if not sig_runs:
        return

    tiers_present = sorted({r["tier"] for r in sig_runs}, key=tier_rank)
    modes_present = sorted({r["mode"] for r in sig_runs}, key=mode_rank)
    if not tiers_present:
        return

    fig, ax = plt.subplots(figsize=(max(8, len(tiers_present) * 2.5), 6))

    values = [r.get("maxFillLevelPassed", 0) or 0 for r in sig_runs]
    ymax = max(values) if values else 1
    ghost_height = max(ymax * 0.02, 1)

    width = 0.8 / max(len(modes_present), 1)
    x_centers = list(range(len(tiers_present)))

    for mi, mode in enumerate(modes_present):
        for ti, tier in enumerate(tiers_present):
            match = [r for r in sig_runs if r["tier"] == tier and r["mode"] == mode]
            x = ti + (mi - (len(modes_present) - 1) / 2) * width
            if not match:
                continue
            run = match[0]
            value = run.get("maxFillLevelPassed", 0) or 0
            if value > 0:
                ax.bar(x, value, width=width, color=mode_color(mode))
                # latency at max
                last_pass = None
                for step in _readprobe_steps(run):
                    if step.get("passed"):
                        last_pass = step
                lat_str = f"\n{int(last_pass['readLatencyMs'])}ms" if last_pass else ""
                ax.annotate(
                    f"{fmt_count(value)}{lat_str}",
                    xy=(x, value), xytext=(0, 4), textcoords="offset points",
                    ha="center", va="bottom", fontsize=8,
                )
            else:
                ax.bar(x, ghost_height, width=width, color="#dddddd",
                       edgecolor="#999999", hatch="///", linewidth=0.7)
                ax.annotate(
                    "no fill passed",
                    xy=(x, ghost_height), xytext=(0, 4), textcoords="offset points",
                    ha="center", va="bottom", fontsize=7, color="#aa3333",
                )

    ax.set_xticks(x_centers)
    ax.set_xticklabels(tiers_present)
    ax.set_yscale("log")
    ax.set_ylabel(f"Max passing fill level — {ROW_LABEL[signal]} (log)")
    ax.set_title(
        f"Read-probe headline — {signal} via {read_path_for_signal(signal)}\n"
        "Bar label: max passing fill + read latency at that fill"
    )

    legend = [Patch(facecolor=mode_color(m), label=m) for m in modes_present]
    legend.append(Patch(facecolor="#dddddd", edgecolor="#999999", hatch="///", label="no fill level passed"))
    ax.legend(handles=legend, title="DB mode", loc="upper left")
    ax.grid(axis="y", which="both", linestyle=":", alpha=0.4)

    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_readprobe(runs: list[dict], signal: str, out: Path) -> None:
    """Read latency vs rows ingested — improved over legacy chart-readprobe."""
    sig_runs = [r for r in iter_runs(runs, signal, "read-probe") if _readprobe_steps(r)]
    if not sig_runs:
        return

    fig, ax = plt.subplots(figsize=(11, 6))
    plotted = False
    threshold_ms = 5000

    for run in sig_runs:
        color = mode_color(run["mode"])
        rp = run.get("readProbe") or {}
        threshold_ms = rp.get("readThresholdMs", threshold_ms)
        steps = _readprobe_steps(run)
        xs = [s["rowsIngested"] for s in steps]
        ys = [s["readLatencyMs"] for s in steps]
        passed = [bool(s.get("passed")) for s in steps]
        if not xs:
            continue
        ax.plot(xs, ys, linestyle="-", color=color, label=run_label(run))
        for x, y, p in zip(xs, ys, passed):
            ax.plot(x, y, marker=("o" if p else "x"), color=color,
                    markersize=9, markeredgewidth=2 if not p else 1, linestyle="")
        plotted = True

    if not plotted:
        plt.close(fig)
        return

    ax.axhline(threshold_ms, linestyle="--", color="#cc3333", alpha=0.7,
               label=f"threshold ({threshold_ms}ms)")
    ax.set_xlabel(f"{ROW_LABEL[signal]} ingested (log)")
    ax.set_ylabel("Read latency (ms, log)")
    ax.set_xscale("log")
    ax.set_yscale("log")
    ax.set_title(
        f"Read latency vs table size — {signal} via {read_path_for_signal(signal)}\n"
        "○ = passed (latency under threshold),  × = failed step"
    )
    ax.grid(True, which="both", linestyle=":", alpha=0.4)
    ax.legend(loc="upper left", fontsize=9)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_readprobe_per_endpoint(runs: list[dict], signal: str, out: Path) -> None:
    """Latency vs rows ingested, **one line per probed endpoint**, per (tier, mode).
    Shows which specific query slows down first as the table grows. Replaces
    the single-line render_readprobe view when the JSON has per-endpoint
    probe data (T2+). Older JSON without `step["probes"]` falls back to the
    median-only view in render_readprobe."""
    sig_runs = [r for r in iter_runs(runs, signal, "read-probe") if _readprobe_steps(r)]
    sig_runs = [r for r in sig_runs if any(s.get("probes") for s in _readprobe_steps(r))]
    if not sig_runs:
        return

    fig, ax = plt.subplots(figsize=(12, 7))
    plotted = False
    threshold_ms = 5000
    # Deterministic palette per endpoint name. Falls back to grey if more than 6.
    endpoint_palette = ["#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd", "#8c564b"]
    endpoint_color: dict[str, str] = {}

    for run in sig_runs:
        rp = run.get("readProbe") or {}
        threshold_ms = rp.get("readThresholdMs", threshold_ms)
        steps = _readprobe_steps(run)
        if not steps:
            continue

        # Collect per-endpoint (rowsIngested, latencyMs, ok) series.
        by_endpoint: dict[str, list[tuple[float, float, bool]]] = {}
        for s in steps:
            rows = s.get("rowsIngested", 0)
            for p in (s.get("probes") or []):
                name = p.get("name") or p.get("path") or "unknown"
                by_endpoint.setdefault(name, []).append((rows, p.get("latencyMs", 0), bool(p.get("ok", False))))

        run_lbl = run_label(run)
        for name, series in by_endpoint.items():
            if name not in endpoint_color:
                endpoint_color[name] = endpoint_palette[len(endpoint_color) % len(endpoint_palette)]
            color = endpoint_color[name]
            xs = [pt[0] for pt in series]
            ys = [pt[1] for pt in series]
            oks = [pt[2] for pt in series]
            ax.plot(xs, ys, linestyle="-", color=color, alpha=0.85,
                    label=f"{run_lbl} · {name}")
            for x, y, ok in zip(xs, ys, oks):
                ax.plot(x, y, marker=("o" if ok else "x"), color=color,
                        markersize=8, markeredgewidth=2 if not ok else 1, linestyle="")
            plotted = True

    if not plotted:
        plt.close(fig)
        return

    ax.axhline(threshold_ms, linestyle="--", color="#cc3333", alpha=0.7,
               label=f"threshold ({threshold_ms}ms)")
    ax.set_xlabel(f"{ROW_LABEL[signal]} ingested (log)")
    ax.set_ylabel("Read latency (ms, log)")
    ax.set_xscale("log")
    ax.set_yscale("log")
    ax.set_title(
        f"Per-endpoint read latency vs table size — {signal}\n"
        "○ = HTTP ok,  × = HTTP error.  Step pass uses median across endpoints."
    )
    ax.grid(True, which="both", linestyle=":", alpha=0.4)
    ax.legend(loc="upper left", fontsize=8, ncol=1)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_readprobe_ingest_time(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = [r for r in iter_runs(runs, signal, "read-probe") if _readprobe_steps(r)]
    if not sig_runs:
        return

    fig, ax = plt.subplots(figsize=(11, 6))
    plotted = False

    for run in sig_runs:
        color = mode_color(run["mode"])
        steps = _readprobe_steps(run)
        xs = [s["fillLevelTarget"] for s in steps]
        ys = [s.get("ingestSecondsElapsed", 0) for s in steps]
        pts = [(x, y) for x, y in zip(xs, ys) if y and y > 0]
        if not pts:
            continue
        xs2, ys2 = zip(*pts)
        ax.plot(xs2, ys2, linestyle="-", marker="o", color=color, label=run_label(run))
        plotted = True

    if not plotted:
        plt.close(fig)
        return

    ax.set_xlabel(f"Target fill level — {ROW_LABEL[signal]} (log)")
    ax.set_ylabel("Time to reach fill level (seconds, log)")
    ax.set_xscale("log")
    ax.set_yscale("log")
    ax.set_title(
        f"Fill time vs target rows — {signal}\n"
        "Curve sag = compaction/merge cost rising with table size."
    )
    ax.grid(True, which="both", linestyle=":", alpha=0.4)
    ax.legend(loc="upper left", fontsize=9)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_readprobe_tier_scaling(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = iter_runs(runs, signal, "read-probe")
    if not sig_runs:
        return

    tiers_present = sorted({r["tier"] for r in sig_runs}, key=tier_rank)
    modes_present = sorted({r["mode"] for r in sig_runs}, key=mode_rank)
    if not tiers_present:
        return

    fig, ax = plt.subplots(figsize=(10, 6))
    plotted = False
    x_positions = list(range(len(tiers_present)))

    for mode in modes_present:
        xs, ys = [], []
        for i, tier in enumerate(tiers_present):
            match = [r for r in sig_runs if r["tier"] == tier and r["mode"] == mode]
            if not match:
                continue
            value = match[0].get("maxFillLevelPassed", 0) or 0
            if value <= 0:
                continue
            xs.append(i)
            ys.append(value)
        if not xs:
            continue
        ax.plot(xs, ys, marker="o", color=mode_color(mode), markersize=9,
                linewidth=2, label=mode)
        for x, y in zip(xs, ys):
            ax.annotate(fmt_count(y), xy=(x, y), xytext=(0, 8),
                        textcoords="offset points", ha="center", va="bottom", fontsize=9)
        base_x = xs[0]
        base_y = ys[0]
        base_vcpu = TIER_VCPU.get(tiers_present[base_x], 1)
        ref_xs = list(range(len(tiers_present)))
        ref_ys = [base_y * (TIER_VCPU.get(tiers_present[i], 1) / base_vcpu) for i in ref_xs]
        ax.plot(ref_xs, ref_ys, linestyle="--", color=mode_color(mode), alpha=0.3, linewidth=1)
        plotted = True

    if not plotted:
        plt.close(fig)
        return

    ax.set_xticks(x_positions)
    ax.set_xticklabels([f"{t}\n{TIER_META.get(t, '')}" for t in tiers_present])
    ax.set_yscale("log")
    ax.set_ylabel(f"Max passing fill level — {ROW_LABEL[signal]} (log)")
    ax.set_title(
        f"Read-probe tier scaling — {signal} via {read_path_for_signal(signal)}\n"
        "Dashed = linear-vCPU reference anchored at smallest tier."
    )
    ax.legend(title="DB mode", loc="upper left")
    ax.grid(True, which="both", linestyle=":", alpha=0.4)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


def render_readprobe_cliff(runs: list[dict], signal: str, out: Path) -> None:
    sig_runs = iter_runs(runs, signal, "read-probe")
    if not sig_runs:
        return

    # Build the union of all fill levels seen across runs, sorted.
    all_targets = sorted({s["fillLevelTarget"] for r in sig_runs for s in _readprobe_steps(r)})
    if not all_targets:
        return

    n_cols = len(all_targets)
    n_rows = len(sig_runs)

    fig_w = max(10, n_cols * 1.8 + 3.5)
    fig_h = max(4, n_rows * 0.9 + 2)
    fig, ax = plt.subplots(figsize=(fig_w, fig_h))

    def _probe_status(step: dict) -> str:
        if step.get("passed"):
            return "pass"
        # over-threshold but read OK
        if step.get("readOk"):
            return "soft-cliff"
        return "hard-fail"

    for ri, run in enumerate(sig_runs):
        y = n_rows - ri - 1
        ax.text(-0.5, y + 0.5, run_label(run), ha="right", va="center",
                fontsize=10, fontweight="bold")

        by_target = {s["fillLevelTarget"]: s for s in _readprobe_steps(run)}
        for ci, target in enumerate(all_targets):
            x = ci
            step = by_target.get(target)
            if step is None:
                color = STATUS_COLOR["missing"]
                label_text = ""
            else:
                color = STATUS_COLOR[_probe_status(step)]
                lat = int(step.get("readLatencyMs", 0))
                label_text = f"{lat}ms"
            ax.add_patch(Rectangle((x, y), 1, 1, facecolor=color,
                                   edgecolor="white", linewidth=1.5))
            if label_text:
                ax.text(x + 0.5, y + 0.5, label_text, ha="center", va="center", fontsize=8)

        max_passed = run.get("maxFillLevelPassed", 0) or 0
        margin = n_cols + 0.35
        if max_passed > 0:
            ax.text(margin, y + 0.5, f"max {fmt_count(max_passed)}",
                    ha="left", va="center", fontsize=10, fontweight="bold")
        else:
            ax.text(margin, y + 0.5, "no fill passed",
                    ha="left", va="center", fontsize=10, color="#cc3333", fontweight="bold")

    # Column header (fill level)
    for ci, target in enumerate(all_targets):
        ax.text(ci + 0.5, n_rows + 0.25, fmt_count(target),
                ha="center", va="bottom", fontsize=10, fontweight="bold")
    ax.text(n_cols / 2.0, n_rows + 0.85, f"Fill level ({ROW_LABEL[signal]})",
            ha="center", va="bottom", fontsize=11, fontweight="bold")

    ax.set_xlim(-4, n_cols + 5)
    ax.set_ylim(-0.6, n_rows + 1.5)
    ax.set_aspect("equal")
    ax.axis("off")

    legend = [
        Patch(facecolor=STATUS_COLOR["pass"], label="passed (read OK, under threshold)"),
        Patch(facecolor=STATUS_COLOR["soft-cliff"], label="over threshold (read OK but slow)"),
        Patch(facecolor=STATUS_COLOR["hard-fail"], label="read errored"),
        Patch(facecolor=STATUS_COLOR["missing"], edgecolor="#cccccc", label="not run"),
    ]
    ax.legend(handles=legend, loc="lower center", bbox_to_anchor=(0.5, -0.05),
              ncol=4, fontsize=9, frameon=False)

    fig.suptitle(f"Read-probe step-status grid — {signal}\nread path: {read_path_for_signal(signal)}",
                 fontsize=12)
    fig.tight_layout(rect=(0, 0.04, 1, 0.94))
    fig.savefig(out, dpi=130, bbox_inches="tight")
    plt.close(fig)


def render_readprobe_signal_mix(runs: list[dict], out: Path) -> None:
    rp_runs = [r for r in runs if r.get("scenario") == "read-probe"]
    if not rp_runs:
        return

    keys = sorted({(r["tier"], r["mode"]) for r in rp_runs},
                  key=lambda k: (tier_rank(k[0]), mode_rank(k[1])))
    if not keys:
        return

    fig, ax = plt.subplots(figsize=(max(8, len(keys) * 1.9), 6))

    width = 0.25
    x = list(range(len(keys)))
    any_value = False

    for si, signal in enumerate(SIGNALS):
        ys = []
        for tier, mode in keys:
            match = [r for r in rp_runs if r["tier"] == tier and r["mode"] == mode and r["signal"] == signal]
            ys.append((match[0].get("maxFillLevelPassed", 0) or 0) if match else 0)
        offsets = [xi + (si - 1) * width for xi in x]
        bars = ax.bar(offsets, ys, width=width,
                      color=SIGNAL_COLORS[signal],
                      label=f"{signal} ({read_path_for_signal(signal)})")
        for b, y in zip(bars, ys):
            if y > 0:
                ax.annotate(fmt_count(y), xy=(b.get_x() + b.get_width() / 2, y),
                            xytext=(0, 3), textcoords="offset points",
                            ha="center", va="bottom", fontsize=7)
                any_value = True

    if not any_value:
        plt.close(fig)
        return

    ax.set_xticks(x)
    ax.set_xticklabels([f"{t}\n{m}" for t, m in keys], fontsize=9)
    ax.set_yscale("log")
    ax.set_ylabel("Max passing fill level (rows, log)")
    ax.set_title("Read-probe cross-signal comparison — which read path scales worst?")
    ax.legend(title="Signal (read path)", loc="upper left", fontsize=8)
    ax.grid(axis="y", which="both", linestyle=":", alpha=0.4)
    fig.tight_layout()
    fig.savefig(out, dpi=130)
    plt.close(fig)


# ---------- summary.md ----------


def render_summary(runs: list[dict], out: Path) -> None:
    lines = [
        "# Traceway hardware benchmark — summary",
        "",
        "> See [../charts.md](../charts.md) for a guide to reading these charts.",
        "",
        f"Runs analyzed: {len(runs)}",
        "",
    ]

    throughput_runs = [r for r in runs if r["scenario"] == "throughput"]
    if throughput_runs:
        lines.append("## Throughput scenario")
        lines.append("")
        lines.append("Failure threshold: combined HTTP error rate + OTLP rejected items > 5% of attempted.")
        lines.append("")
        for signal in SIGNALS:
            sig_runs = [r for r in throughput_runs if r["signal"] == signal]
            if not sig_runs:
                continue
            lines.append(f"### {signal.capitalize()}")
            lines.append("")
            lines.append(f"| Tier | Mode | Max {ITEM_LABEL[signal]} | P1 max batch | P2 max req/sec @ batch | P3 max req/sec @ batch=100 |")
            lines.append("|------|------|---------|--------------|-------------------------|-------------------------------|")
            for run in sorted(sig_runs, key=lambda r: (tier_rank(r["tier"]), mode_rank(r["mode"]))):
                max_items = int(run.get("maxSustainableItemsPerSec", 0) or 0)
                max_batch = (run.get("phase1") or {}).get("maxBatchSize", 0)
                p2 = run.get("phase2") or {}
                p2_rate = p2.get("maxRequestRate", 0)
                p2_batch = p2.get("fixedBatchSize", 0)
                p2_cell = f"{p2_rate:g} @ {p2_batch:,}" if p2_rate else "—"
                p3 = run.get("phase3") or {}
                p3_rate = p3.get("maxRequestRate", 0)
                p3_cell = f"{p3_rate:g}" if p3_rate else "—"
                lines.append(f"| {run['tier']} | {run['mode']} | {max_items:,} | {max_batch:,} | {p2_cell} | {p3_cell} |")
            lines.append("")

    readprobe_runs = [r for r in runs if r["scenario"] == "read-probe"]
    if readprobe_runs:
        lines.append("## Read-probe scenario")
        lines.append("")
        lines.append("Failure: a read probe exceeded the configured threshold (default 5000ms) or returned an error. "
                     "Max fill level passed is the largest row count at which the read still came back in time.")
        lines.append("")
        for signal in SIGNALS:
            sig_runs = [r for r in readprobe_runs if r["signal"] == signal]
            if not sig_runs:
                continue
            lines.append(f"### {signal.capitalize()} — probing `{read_path_for_signal(signal)}`")
            lines.append("")
            lines.append(f"| Tier | Mode | Max {ROW_LABEL[signal]} passed | Read latency @ max (ms) | First failing fill | Read latency @ failing (ms) |")
            lines.append("|------|------|----------------|-------------------|--------------------|------------------------|")
            for run in sorted(sig_runs, key=lambda r: (tier_rank(r["tier"]), mode_rank(r["mode"]))):
                steps = _readprobe_steps(run)
                max_passed = run.get("maxFillLevelPassed", 0) or 0
                latency_at_max = 0
                first_fail = ""
                latency_at_fail = ""
                for s in steps:
                    if s.get("passed"):
                        latency_at_max = int(s.get("readLatencyMs", 0))
                    elif not first_fail:
                        first_fail = f"{int(s.get('rowsIngested', 0)):,}"
                        latency_at_fail = f"{int(s.get('readLatencyMs', 0))}"
                lines.append(f"| {run['tier']} | {run['mode']} | {int(max_passed):,} | {latency_at_max} | {first_fail or '—'} | {latency_at_fail or '—'} |")
            lines.append("")

    lines.append("Generated by `benchmarks/scripts/chart.py`.")
    out.write_text("\n".join(lines) + "\n")


# ---------- Entry point ----------


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: chart.py <results-dir>", file=sys.stderr)
        return 2
    results_dir = Path(sys.argv[1])
    if not results_dir.is_dir():
        print(f"not a directory: {results_dir}", file=sys.stderr)
        return 2

    runs = load_runs(results_dir)
    if not runs:
        print(f"no usable *.json files in {results_dir}", file=sys.stderr)
        return 1

    for signal in SIGNALS:
        # Throughput charts (no-op if no throughput runs for this signal)
        render_headline_bar(runs, signal, results_dir / f"chart-{signal}.png")
        render_phase1_latency(runs, signal, results_dir / f"chart-phase1-batch-{signal}.png")
        render_phase2_rate(runs, signal, results_dir / f"chart-phase2-rate-{signal}.png")
        render_phase3_rate(runs, signal, results_dir / f"chart-phase3-rate-{signal}.png")
        render_pareto(runs, signal, results_dir / f"chart-pareto-{signal}.png")
        render_tier_scaling(runs, signal, results_dir / f"chart-tier-scaling-{signal}.png")
        render_cliff_grid(runs, signal, results_dir / f"chart-cliff-{signal}.png")
        render_batch_efficiency(runs, signal, results_dir / f"chart-batch-efficiency-{signal}.png")
        render_ch_pressure(runs, signal, results_dir / f"chart-ch-pressure-{signal}.png")

        # Read-probe charts (no-op if no read-probe runs for this signal)
        render_readprobe_headline(runs, signal, results_dir / f"chart-readprobe-headline-{signal}.png")
        render_readprobe(runs, signal, results_dir / f"chart-readprobe-{signal}.png")
        render_readprobe_per_endpoint(runs, signal, results_dir / f"chart-readprobe-per-endpoint-{signal}.png")
        render_readprobe_ingest_time(runs, signal, results_dir / f"chart-readprobe-ingest-time-{signal}.png")
        render_readprobe_tier_scaling(runs, signal, results_dir / f"chart-readprobe-tier-scaling-{signal}.png")
        render_readprobe_cliff(runs, signal, results_dir / f"chart-readprobe-cliff-{signal}.png")

    render_signal_mix(runs, results_dir / "chart-signal-mix.png")
    render_readprobe_signal_mix(runs, results_dir / "chart-readprobe-signal-mix.png")
    render_summary(runs, results_dir / "summary.md")
    print(f"wrote charts and summary.md into {results_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
