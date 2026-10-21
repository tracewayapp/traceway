"use client";

import { useEffect, useRef, useState } from "react";

import { MODEL_COLOR, formatUsd, rate } from "@/lib/benchmark-format";
import type { ModelKey } from "@/lib/benchmarks";

export type CostPoint = {
  id: string;
  model: ModelKey;
  modelLabel: string;
  modelShort: string;
  label: string;
  pass: number;
  runs: number;
  costPerRunUsd: number;
  overall: boolean;
};

const HEIGHT = 340;
const MARGIN = { top: 20, right: 24, bottom: 52, left: 52 };
const LOG_TICKS = [0.001, 0.003, 0.01, 0.03, 0.1, 0.3, 1, 3, 10, 30];

function domainFor(points: CostPoint[]) {
  const costs = points.map((p) => p.costPerRunUsd);
  const lo = Math.max(...LOG_TICKS.filter((t) => t <= Math.min(...costs)));
  const hi = Math.min(...LOG_TICKS.filter((t) => t >= Math.max(...costs)));
  const rates = points.map((p) => (100 * p.pass) / p.runs);
  const yLo = Math.max(0, Math.floor(Math.min(...rates) / 10) * 10);
  return { lo, hi, yLo, yHi: 100 };
}

export function BenchmarkCostChart({
  points,
  models,
}: {
  points: CostPoint[];
  models: { key: ModelKey; label: string }[];
}) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(720);
  const [active, setActive] = useState<string | null>(null);

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const ro = new ResizeObserver(([entry]) =>
      setWidth(Math.max(280, Math.round(entry.contentRect.width))),
    );
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const { lo, hi, yLo, yHi } = domainFor(points);
  const plotW = width - MARGIN.left - MARGIN.right;
  const plotH = HEIGHT - MARGIN.top - MARGIN.bottom;
  const x = (usd: number) =>
    MARGIN.left +
    ((Math.log10(usd) - Math.log10(lo)) / (Math.log10(hi) - Math.log10(lo))) *
      plotW;
  const y = (pct: number) =>
    MARGIN.top + (1 - (pct - yLo) / (yHi - yLo)) * plotH;

  const xTicks = LOG_TICKS.filter((t) => t >= lo && t <= hi);
  const yTicks: number[] = [];
  for (let t = yLo; t <= yHi; t += 10) yTicks.push(t);

  const ordered = [
    ...points.filter((p) => !p.overall),
    ...points.filter((p) => p.overall),
  ];
  const activePoint = points.find((p) => p.id === active) ?? null;

  return (
    <div>
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-[0.8125rem] text-fg-1">
        {models.map((m) => (
          <span key={m.key} className="inline-flex items-center gap-2">
            <span
              aria-hidden
              className="inline-block size-2.5 rounded-full"
              style={{ background: MODEL_COLOR[m.key] }}
            />
            {m.label}
          </span>
        ))}
        <span className="text-fg-3">
          Large dot: all runs · small dots: one suite and setup
        </span>
      </div>

      <div ref={wrapRef} className="relative mt-4">
        <svg
          width={width}
          height={HEIGHT}
          viewBox={`0 0 ${width} ${HEIGHT}`}
          role="img"
          aria-label="Pass rate against agent cost per run, one dot per model and per suite and setup"
          className="block max-w-full"
        >
          {yTicks.map((t) => (
            <g key={`y${t}`}>
              <line
                x1={MARGIN.left}
                x2={width - MARGIN.right}
                y1={y(t)}
                y2={y(t)}
                stroke="var(--hair-2)"
                strokeWidth={1}
              />
              <text
                x={MARGIN.left - 10}
                y={y(t)}
                dy="0.32em"
                textAnchor="end"
                fontSize={11}
                fill="var(--fg-3)"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {t}%
              </text>
            </g>
          ))}
          {xTicks.map((t) => (
            <g key={`x${t}`}>
              <line
                x1={x(t)}
                x2={x(t)}
                y1={MARGIN.top}
                y2={HEIGHT - MARGIN.bottom}
                stroke="var(--hair)"
                strokeWidth={1}
              />
              <text
                x={x(t)}
                y={HEIGHT - MARGIN.bottom + 18}
                textAnchor="middle"
                fontSize={11}
                fill="var(--fg-3)"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                ${t}
              </text>
            </g>
          ))}
          <text
            x={MARGIN.left + plotW / 2}
            y={HEIGHT - 8}
            textAnchor="middle"
            fontSize={11}
            fill="var(--fg-2)"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            Agent cost per run (log scale)
          </text>
          <text
            transform={`translate(14 ${MARGIN.top + plotH / 2}) rotate(-90)`}
            textAnchor="middle"
            fontSize={11}
            fill="var(--fg-2)"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            Pass rate
          </text>

          {ordered.map((p) => {
            const cx = x(p.costPerRunUsd);
            const cy = y((100 * p.pass) / p.runs);
            const r = p.overall ? 7 : 4.5;
            const isActive = active === p.id;
            const labelLeft = cx > MARGIN.left + plotW * 0.62;
            return (
              <g key={p.id}>
                <circle
                  cx={cx}
                  cy={cy}
                  r={isActive ? r + 1.5 : r}
                  fill={MODEL_COLOR[p.model]}
                  fillOpacity={p.overall ? 1 : 0.7}
                  stroke="var(--ink-0)"
                  strokeWidth={2}
                />
                {p.overall ? (
                  <text
                    x={labelLeft ? cx - 14 : cx + 14}
                    y={cy}
                    dy="0.32em"
                    textAnchor={labelLeft ? "end" : "start"}
                    fontSize={12}
                    fill="var(--fg-0)"
                    fontWeight={600}
                  >
                    {width < 560 ? p.modelShort : p.modelLabel} ·{" "}
                    {rate(p.pass, p.runs)}%
                  </text>
                ) : null}
                <circle
                  cx={cx}
                  cy={cy}
                  r={12}
                  fill="transparent"
                  tabIndex={0}
                  role="img"
                  aria-label={`${p.modelLabel}, ${p.label}: ${p.pass} of ${p.runs} passed (${rate(p.pass, p.runs)}%), ${formatUsd(p.costPerRunUsd)} per run`}
                  onPointerEnter={() => setActive(p.id)}
                  onPointerLeave={() => setActive(null)}
                  onFocus={() => setActive(p.id)}
                  onBlur={() => setActive(null)}
                  style={{ outline: "none", cursor: "default" }}
                />
              </g>
            );
          })}
        </svg>

        {activePoint ? (
          <Tooltip
            point={activePoint}
            left={x(activePoint.costPerRunUsd)}
            top={y((100 * activePoint.pass) / activePoint.runs)}
            width={width}
          />
        ) : null}
      </div>
    </div>
  );
}

function Tooltip({
  point,
  left,
  top,
  width,
}: {
  point: CostPoint;
  left: number;
  top: number;
  width: number;
}) {
  const boxW = 220;
  const clamped = Math.min(Math.max(left - boxW / 2, 0), width - boxW);
  const below = top < 110;
  return (
    <div
      role="tooltip"
      className="pointer-events-none absolute z-10 rounded-lg px-3 py-2.5"
      style={{
        left: clamped,
        top: below ? top + 16 : top - 16,
        width: boxW,
        transform: below ? undefined : "translateY(-100%)",
        background: "var(--ink-0)",
        border: "1px solid var(--hair-2)",
        boxShadow: "0 18px 40px -20px rgba(10,14,24,0.45)",
      }}
    >
      <p className="font-mono text-[0.9375rem] font-semibold text-fg-0 tabular-nums">
        {rate(point.pass, point.runs)}% · {formatUsd(point.costPerRunUsd)}/run
      </p>
      <p className="mt-1 flex items-center gap-1.5 text-[0.8125rem] text-fg-1">
        <span
          aria-hidden
          className="inline-block h-0.5 w-3 rounded-full"
          style={{ background: MODEL_COLOR[point.model] }}
        />
        {point.modelLabel}
      </p>
      <p className="mt-0.5 text-[0.8125rem] text-fg-2">
        {point.label} · {point.pass}/{point.runs} passed
      </p>
    </div>
  );
}
