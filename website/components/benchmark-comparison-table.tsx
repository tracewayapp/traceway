"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Check, ChevronDown } from "lucide-react";

import { cn } from "@/lib/utils";
import { MODEL_COLOR } from "@/lib/benchmark-format";
import type { ModelKey } from "@/lib/benchmarks";

export type ComparisonValue = {
  value: number;
  text: string;
  note?: string;
  title?: string;
};

export type ComparisonRow = {
  label: string;
  sub: string;
  better: "higher" | "lower";
  values: Record<ModelKey, ComparisonValue[]>;
};

export type ComparisonModel = { key: ModelKey; label: string; short: string; note: string };

// The widths the fit is computed from, measured on the rendered table: the
// row label column is narrow below the md breakpoint, where the content
// column is 712px or less; a model column needs 125px with the short name
// and 210px with the full one ("DeepSeek V4.1 Flash").
const LABEL_WIDTH = { narrow: 132, wide: 200 };
const WIDE_FROM = 700;
const SHORT_WIDTH = 130;
const FULL_WIDTH = 220;

function labelWidth(container: number) {
  return container >= WIDE_FROM ? LABEL_WIDTH.wide : LABEL_WIDTH.narrow;
}

function columnsThatFit(container: number, count: number) {
  const fit = Math.floor((container - labelWidth(container)) / SHORT_WIDTH);
  return Math.max(1, Math.min(count, fit));
}

function bestOf(row: ComparisonRow, models: ComparisonModel[], i: number) {
  const vs = models.map((m) => row.values[m.key][i].value);
  return row.better === "higher" ? Math.max(...vs) : Math.min(...vs);
}

export function BenchmarkComparisonTable({
  models,
  rows,
}: {
  models: ComparisonModel[];
  rows: ComparisonRow[];
}) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState<number | null>(null);
  const [scrolls, setScrolls] = useState(false);
  // null follows the screen; a list is what the reader picked.
  const [picked, setPicked] = useState<ModelKey[] | null>(null);

  useLayoutEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const measure = () => setWidth(el.clientWidth);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const fit = width === null ? models.length : columnsThatFit(width, models.length);
  const visible = picked
    ? models.filter((m) => picked.includes(m.key))
    : models.slice(0, fit);
  const featured = models[0].key;
  const outline = "color-mix(in oklab, var(--a1) 70%, transparent)";
  // Short names once the full ones would not fit, and past five columns
  // whatever the width.
  const compact =
    visible.length > 5 ||
    (width !== null && labelWidth(width) + visible.length * FULL_WIDTH > width);

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (el) setScrolls(el.scrollWidth > el.clientWidth + 1);
  }, [width, visible.length, compact]);

  function toggle(key: ModelKey) {
    const current = visible.map((m) => m.key);
    const next = current.includes(key)
      ? current.filter((k) => k !== key)
      : models.map((m) => m.key).filter((k) => k === key || current.includes(k));
    if (next.length) setPicked(next);
  }

  const hint = scrolls
    ? "Scroll sideways for the rest of the columns."
    : !picked && fit < models.length
      ? `The ${fit === 1 ? "model" : `${fit} models`} that fit this screen. Pick others from the menu.`
      : null;

  return (
    <div ref={wrapRef}>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
        <p className="text-[0.8125rem] text-fg-3">{hint}</p>
        <ModelPicker
          models={models}
          visible={visible}
          auto={picked === null}
          onAuto={() => setPicked(null)}
          onAll={() => setPicked(models.map((m) => m.key))}
          onToggle={toggle}
        />
      </div>
      <div ref={scrollRef} className="overflow-x-auto">
        <table
          className="w-full border-separate text-left"
          style={{ borderSpacing: 0 }}
        >
          <thead>
            <tr>
              <th
                className="sticky left-0 z-10 w-[132px] pb-3 md:static md:w-[200px]"
                style={{ background: "var(--ink-0)" }}
              />
              {visible.map((m) => {
                const isFeatured = m.key === featured;
                return (
                  <th
                    key={m.key}
                    scope="col"
                    className="px-3 pt-4 pb-3 text-center align-bottom font-normal"
                    style={
                      isFeatured
                        ? {
                            border: `1.5px solid ${outline}`,
                            borderBottom: "none",
                            borderRadius: "14px 14px 0 0",
                          }
                        : undefined
                    }
                  >
                    <span
                      className="inline-flex items-center gap-2 text-[0.9375rem] font-medium whitespace-nowrap text-fg-0 md:text-[1.0625rem]"
                      title={m.label}
                    >
                      <span
                        aria-hidden
                        className="inline-block h-2.5 w-2.5 rounded-full"
                        style={{ background: MODEL_COLOR[m.key] }}
                      />
                      {compact ? m.short : m.label}
                    </span>
                    <span className="block font-mono text-[0.6875rem] text-fg-3">
                      {m.note}
                    </span>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, ri) => {
              const last = ri === rows.length - 1;
              const n = row.values[featured].length;
              // Best is judged against every model, hidden columns included,
              // so a bold cell means the same thing on a phone and a desktop.
              const best = Array.from({ length: n }, (_, i) =>
                bestOf(row, models, i),
              );
              return (
                <tr key={row.label}>
                  <th
                    scope="row"
                    className="sticky left-0 z-10 py-5 pr-3 align-middle font-normal md:static md:pr-4"
                    style={{
                      borderTop: "1px solid var(--hair-2)",
                      background: "var(--ink-0)",
                    }}
                  >
                    <span className="block text-[0.875rem] leading-snug font-medium text-fg-0 md:text-[1rem]">
                      {row.label}
                    </span>
                    <span className="mt-0.5 block text-[0.75rem] text-fg-3 md:text-[0.8125rem]">
                      {row.sub}
                    </span>
                  </th>
                  {visible.map((m) => {
                    const values = row.values[m.key];
                    const wins = values.map((v, i) => v.value === best[i]);
                    const shaded = wins.every(Boolean);
                    const isFeatured = m.key === featured;
                    return (
                      <td
                        key={m.key}
                        className="px-3 py-4 text-center align-middle"
                        style={{
                          borderTop: "1px solid var(--hair-2)",
                          background: shaded
                            ? isFeatured
                              ? "color-mix(in oklab, var(--a1) 13%, var(--ink-0))"
                              : "color-mix(in oklab, var(--fg-0) 5%, var(--ink-0))"
                            : undefined,
                          borderLeft: isFeatured
                            ? `1.5px solid ${outline}`
                            : undefined,
                          borderRight: isFeatured
                            ? `1.5px solid ${outline}`
                            : undefined,
                          borderBottom:
                            isFeatured && last
                              ? `1.5px solid ${outline}`
                              : undefined,
                          borderRadius:
                            isFeatured && last ? "0 0 14px 14px" : undefined,
                        }}
                      >
                        <div className="flex flex-col items-center gap-2">
                          {values.map((v, i) => (
                            <span key={i} className="block" title={v.title}>
                              <span
                                className="block text-[1.0625rem] tabular-nums"
                                style={{
                                  color: "var(--fg-0)",
                                  fontWeight: wins[i] ? 700 : 400,
                                }}
                              >
                                {v.text}
                              </span>
                              {v.note ? (
                                <span className="block text-[0.6875rem] leading-tight text-fg-3">
                                  {v.note}
                                </span>
                              ) : null}
                            </span>
                          ))}
                        </div>
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function ModelPicker({
  models,
  visible,
  auto,
  onAuto,
  onAll,
  onToggle,
}: {
  models: ComparisonModel[];
  visible: ComparisonModel[];
  auto: boolean;
  onAuto: () => void;
  onAll: () => void;
  onToggle: (key: ModelKey) => void;
}) {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const shown = new Set(visible.map((m) => m.key));
  const all = visible.length === models.length;

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const onClick = (e: MouseEvent) => {
      if (!menuRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onClick);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onClick);
    };
  }, [open]);

  return (
    <div ref={menuRef} className="relative ml-auto">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "inline-flex h-9 items-center gap-2 rounded-md border px-3 font-mono text-[0.6875rem] font-medium tracking-[0.1em] uppercase transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-[color:var(--ring)]",
          open
            ? "border-fg-3 text-fg-0"
            : "border-hair-2 text-fg-1 hover:border-fg-3 hover:text-fg-0",
        )}
        style={{ background: "var(--ink-0)" }}
      >
        Models
        <span className="text-fg-3">·</span>
        <span className="tabular-nums">
          {all ? "all" : `${visible.length} of ${models.length}`}
        </span>
        <ChevronDown
          className={cn(
            "size-3.5 opacity-60 transition-transform",
            open && "rotate-180",
          )}
        />
      </button>
      {open ? (
        <div
          role="menu"
          className="absolute top-full right-0 z-20 mt-2 w-[264px] max-w-[calc(100vw-40px)] rounded-xl p-1.5"
          style={{
            background: "var(--ink-0)",
            border: "1px solid var(--hair-2)",
            boxShadow:
              "0 1px 2px rgba(10,14,24,0.06), 0 16px 40px -16px rgba(10,14,24,0.3)",
          }}
        >
          <MenuItem checked={auto} onClick={onAuto}>
            <span className="text-[0.875rem] font-medium text-fg-0">
              Fit the screen
            </span>
            <span className="block text-[0.75rem] text-fg-3">
              As many as fit, best pass rate first
            </span>
          </MenuItem>
          <MenuItem checked={!auto && all} onClick={onAll}>
            <span className="text-[0.875rem] font-medium text-fg-0">
              All {models.length} models
            </span>
            <span className="block text-[0.75rem] text-fg-3">
              Scrolls sideways on a narrow screen
            </span>
          </MenuItem>
          <div className="my-1.5" style={{ borderTop: "1px solid var(--hair)" }} />
          {models.map((m) => (
            <MenuItem
              key={m.key}
              checked={shown.has(m.key)}
              onClick={() => onToggle(m.key)}
            >
              <span className="inline-flex items-center gap-2 text-[0.875rem] font-medium text-fg-0">
                <span
                  aria-hidden
                  className="inline-block h-2 w-2 rounded-full"
                  style={{ background: MODEL_COLOR[m.key] }}
                />
                {m.label}
              </span>
              <span className="block font-mono text-[0.6875rem] text-fg-3">
                {m.note}
              </span>
            </MenuItem>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function MenuItem({
  checked,
  onClick,
  children,
}: {
  checked: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      role="menuitemcheckbox"
      aria-checked={checked}
      onClick={onClick}
      className="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left transition-colors outline-none hover:bg-ink-2 focus-visible:bg-ink-2"
    >
      <span
        aria-hidden
        className={cn(
          "grid size-4 shrink-0 place-items-center rounded-[4px] border transition-colors",
          checked ? "border-transparent" : "border-hair-2",
        )}
        style={checked ? { background: "var(--a1)", color: "#fff" } : undefined}
      >
        {checked ? <Check className="size-3" strokeWidth={3} /> : null}
      </span>
      <span className="min-w-0 flex-1">{children}</span>
    </button>
  );
}
