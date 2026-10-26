"use client";

import { useState } from "react";

import { MODEL_COLOR, rate } from "@/lib/benchmark-format";
import type { ModelKey } from "@/lib/benchmarks";

export type IndexBar = {
  id: string;
  model: ModelKey;
  weights: "open" | "proprietary";
  label: string;
  detail?: string;
  pass: number;
  rightAnswerEvidence: number;
  runs: number;
};

export type IndexView = {
  id: string;
  tab: string;
  title: string;
  bars: IndexBar[];
};

type Metric = "pass" | "rightAnswerEvidence";

const METRICS: { id: Metric; label: string }[] = [
  { id: "pass", label: "Pass rate" },
  { id: "rightAnswerEvidence", label: "Right answer and evidence" },
];

export const WEIGHTS_COLOR = {
  proprietary: "var(--fg-0)",
  open: "#6aa5f8",
} as const;

const WEIGHTS_TEXT = {
  proprietary: "var(--ink-0)",
  open: "#ffffff",
} as const;

const PLOT_HEIGHT = 300;
const GRID = [0, 25, 50, 75, 100];
const BAR_GAP = 24;
const LABEL_COLUMN_MIN = 68;

export function BenchmarkIndexChart({
  views,
  subtitle,
}: {
  views: IndexView[];
  subtitle: string;
}) {
  const [viewId, setViewId] = useState(views[0].id);
  const [metric, setMetric] = useState<Metric>("pass");
  const view = views.find((v) => v.id === viewId) ?? views[0];
  const value = (b: IndexBar) => rate(b[metric], b.runs);
  const bars = [...view.bars].sort((a, b) => value(b) - value(a));
  const dense = bars.length > 8;
  const minWidth = dense ? 64 + bars.length * 46 : 0;
  const legendBelow =
    bars.length * LABEL_COLUMN_MIN + (bars.length - 1) * BAR_GAP;
  const metricLabel = METRICS.find((m) => m.id === metric)!.label;

  return (
    <div
      className="overflow-hidden rounded-[14px]"
      style={{
        background: "var(--ink-0)",
        border: "1px solid var(--hair-2)",
        boxShadow:
          "0 1px 2px rgba(10,14,24,0.04), 0 12px 30px -18px rgba(10,14,24,0.14)",
      }}
    >
      <div
        role="tablist"
        aria-label="Group the bars"
        className="flex gap-1 overflow-x-auto px-3 py-3 md:px-4"
        style={{
          background: "var(--ink-1)",
          borderBottom: "1px solid var(--hair-2)",
        }}
      >
        {views.map((v) => {
          const active = v.id === view.id;
          return (
            <button
              key={v.id}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => setViewId(v.id)}
              className="shrink-0 cursor-pointer rounded-md px-4 py-2 text-[0.9375rem] transition-colors"
              style={{
                background: active ? "var(--ink-3)" : "transparent",
                color: active ? "var(--fg-0)" : "var(--fg-2)",
                fontWeight: active ? 600 : 500,
              }}
            >
              {v.tab}
            </button>
          );
        })}
      </div>

      <div className="px-5 pt-6 pb-5 md:px-8 md:pt-8">
        <div className="flex flex-col gap-5 md:flex-row md:items-start md:justify-between">
          <div className="max-w-[60ch]">
            <h3 className="text-[1.375rem] leading-tight font-semibold tracking-[-0.01em] text-fg-0 md:text-[1.75rem]">
              {metricLabel} {view.title}
            </h3>
            <p className="mt-2 text-[0.875rem] leading-relaxed text-fg-3">
              {subtitle}
            </p>
          </div>
          <div
            role="group"
            aria-label="Metric"
            className="inline-flex shrink-0 self-start rounded-lg p-1"
            style={{ border: "1px solid var(--hair-2)" }}
          >
            {METRICS.map((m) => {
              const active = m.id === metric;
              return (
                <button
                  key={m.id}
                  type="button"
                  aria-pressed={active}
                  onClick={() => setMetric(m.id)}
                  className="cursor-pointer rounded-md px-3 py-1.5 text-[0.8125rem] transition-colors"
                  style={{
                    background: active ? "var(--ink-2)" : "transparent",
                    color: active ? "var(--fg-0)" : "var(--fg-2)",
                    fontWeight: active ? 600 : 500,
                  }}
                >
                  {m.label}
                </button>
              );
            })}
          </div>
        </div>

        <div className="mt-6 flex flex-wrap items-end justify-between gap-3">
          <div className="flex flex-wrap gap-x-5 gap-y-1 text-[0.9375rem] text-fg-1">
            {(["proprietary", "open"] as const).map((w) => (
              <span key={w} className="inline-flex items-center gap-2">
                <span
                  aria-hidden
                  className="inline-block h-2.5 w-2.5 rounded-full"
                  style={{ background: WEIGHTS_COLOR[w] }}
                />
                {w === "open" ? "Open weights" : "Proprietary"}
              </span>
            ))}
          </div>
          <span className="inline-flex items-center gap-2 text-fg-1">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src="/favicon.svg" alt="" className="h-4 w-4" />
            <span className="font-semibold tracking-[-0.01em]">AI SRE BENCH</span>
            <span className="text-[0.75rem] text-fg-2">by Traceway</span>
          </span>
        </div>

        <div
          className="mt-3 overflow-x-auto pb-1"
          data-bars={bars.length}
          style={{ containerType: "inline-size", containerName: "index-chart" }}
        >
          {dense ? null : (
            <style>{`
              @container index-chart (max-width: ${legendBelow - 1}px) {
                [data-bars="${bars.length}"] .index-chart-row { gap: 10px; }
                [data-bars="${bars.length}"] .index-chart-labels { display: none; }
                [data-bars="${bars.length}"] .index-chart-legend { display: grid; }
              }
            `}</style>
          )}
          <div style={{ minWidth, paddingLeft: dense ? 64 : 0 }}>
            <div className="relative" style={{ height: PLOT_HEIGHT }}>
              {GRID.map((g) => (
                <div
                  key={g}
                  aria-hidden
                  className="absolute inset-x-0"
                  style={{
                    bottom: `${g}%`,
                    borderTop:
                      g === 0
                        ? "1px solid var(--hair-2)"
                        : "1px dashed var(--hair-2)",
                  }}
                />
              ))}
              <div
                className={`absolute inset-0 flex items-end justify-center ${
                  dense ? "gap-1.5" : "index-chart-row gap-6"
                }`}
              >
                {bars.map((b) => {
                  const v = value(b);
                  return (
                    <div
                      key={b.id}
                      className="flex h-full min-w-0 flex-1 items-end justify-center"
                      style={{ maxWidth: dense ? 56 : 120 }}
                    >
                      <div
                        role="img"
                        aria-label={`${b.label}${b.detail ? `, ${b.detail}` : ""}: ${v}% (${b[metric]} of ${b.runs} runs)`}
                        title={`${b.label}${b.detail ? ` · ${b.detail}` : ""}: ${b[metric]}/${b.runs} runs`}
                        className="flex w-full items-center justify-center rounded-t-[4px] transition-[height] duration-500"
                        style={{
                          height: `${Math.max(v, 1)}%`,
                          background: WEIGHTS_COLOR[b.weights],
                        }}
                      >
                        {v >= 12 ? (
                          <span
                            className="font-medium tabular-nums"
                            style={{
                              color: WEIGHTS_TEXT[b.weights],
                              fontSize: dense ? "0.75rem" : "1rem",
                            }}
                          >
                            {v}
                          </span>
                        ) : null}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>

            <div
              className={`flex justify-center ${
                dense ? "gap-1.5" : "index-chart-row gap-6"
              }`}
            >
              {bars.map((b) => (
                <div
                  key={b.id}
                  className="flex min-w-0 flex-1 flex-col items-center"
                  style={{ maxWidth: dense ? 56 : 120 }}
                >
                  <span
                    aria-hidden
                    className="mt-3 inline-block h-3 w-3 rounded-[3px]"
                    style={{ background: MODEL_COLOR[b.model] }}
                  />
                  {dense ? (
                    <div
                      className="relative w-full"
                      style={{ height: 136 }}
                    >
                      <span
                        className="absolute top-1.5 text-right text-[0.75rem] leading-tight whitespace-nowrap text-fg-1"
                        style={{
                          right: "50%",
                          transform: "rotate(-45deg)",
                          transformOrigin: "100% 0",
                        }}
                      >
                        {b.label}
                        {b.detail ? (
                          <span className="block text-fg-3">{b.detail}</span>
                        ) : null}
                      </span>
                    </div>
                  ) : (
                    <div className="index-chart-labels mt-2 text-center text-[0.875rem] leading-snug text-fg-1">
                      {b.label}
                      {b.detail ? (
                        <span className="block text-[0.75rem] text-fg-3">
                          {b.detail}
                        </span>
                      ) : null}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>

          {dense ? null : (
            <ul className="index-chart-legend mt-4 hidden grid-cols-1 gap-x-6 gap-y-2 text-[0.8125rem] leading-snug text-fg-1 @min-[30rem]/index-chart:grid-cols-2">
              {bars.map((b) => (
                <li key={b.id} className="flex items-center gap-2">
                  <span
                    aria-hidden
                    className="inline-block h-3 w-3 shrink-0 rounded-[3px]"
                    style={{ background: MODEL_COLOR[b.model] }}
                  />
                  <span>
                    {b.label}
                    {b.detail ? (
                      <span className="text-fg-3"> · {b.detail}</span>
                    ) : null}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}
