"use client";

import { Fragment, useMemo, useState } from "react";
import { Check, ChevronRight, Redo2, Timer, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { KIND_LABEL, MODEL_COLOR, formatUsd } from "@/lib/benchmark-format";
import type {
  BenchmarkCase,
  CaseRun,
  ModelKey,
  RunKind,
} from "@/lib/benchmarks";

type MatrixModel = { key: ModelKey; label: string; short: string };
type MatrixSuite = { id: string; label: string; setups: string[] };
type MatrixGroup = { id: string; label: string };
type MatrixIssue = { id: string; kind: string; title: string };

type Hover = {
  caseId: string;
  model: MatrixModel;
  arm: string;
  run: CaseRun;
  x: number;
  y: number;
  above: boolean;
};

const KIND_ICON: Record<RunKind, typeof Check> = {
  pass: Check,
  trajectory: Redo2,
  wrong: X,
  budget: Timer,
};

const CELL_STYLE: Record<RunKind, React.CSSProperties> = {
  pass: {
    background: "color-mix(in oklab, var(--ok) 14%, transparent)",
    color: "var(--ok)",
  },
  trajectory: {
    border: "1px solid color-mix(in oklab, var(--crit) 55%, transparent)",
    color: "var(--crit)",
  },
  wrong: {
    background: "color-mix(in oklab, var(--crit) 16%, transparent)",
    color: "var(--crit)",
  },
  budget: {
    background: "color-mix(in oklab, var(--crit) 16%, transparent)",
    color: "var(--crit)",
  },
};

const TOOLTIP_WIDTH = 340;

export function BenchmarkCaseMatrix({
  cases,
  models,
  suites,
  groups,
  setupLabels,
  issues,
}: {
  cases: BenchmarkCase[];
  models: MatrixModel[];
  suites: MatrixSuite[];
  groups: MatrixGroup[];
  setupLabels: Record<string, string>;
  issues: MatrixIssue[];
}) {
  const [group, setGroup] = useState("all");
  const [failuresOnly, setFailuresOnly] = useState(false);
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [hover, setHover] = useState<Hover | null>(null);

  const issueById = useMemo(
    () => Object.fromEntries(issues.map((i) => [i.id, i])),
    [issues],
  );

  const hasFailure = (c: BenchmarkCase) =>
    models.some((m) =>
      Object.values(c.runs[m.key]).some((r) => r.status !== "pass"),
    );

  const visible = cases.filter(
    (c) =>
      (group === "all" || c.group === group) &&
      (!failuresOnly || hasFailure(c)),
  );
  const visibleIds = visible.map((c) => c.id);
  const allExpanded =
    visibleIds.length > 0 && visibleIds.every((id) => expanded.has(id));

  function toggle(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    setExpanded((prev) => {
      const next = new Set(prev);
      for (const id of visibleIds) {
        if (allExpanded) next.delete(id);
        else next.add(id);
      }
      return next;
    });
  }

  function showTooltip(
    el: HTMLElement,
    caseId: string,
    model: MatrixModel,
    arm: string,
    run: CaseRun,
  ) {
    const rect = el.getBoundingClientRect();
    const x = Math.min(
      Math.max(rect.left + rect.width / 2 - TOOLTIP_WIDTH / 2, 12),
      window.innerWidth - TOOLTIP_WIDTH - 12,
    );
    const above = rect.bottom + 220 > window.innerHeight;
    setHover({
      caseId,
      model,
      arm,
      run,
      x,
      y: above ? rect.top - 8 : rect.bottom + 8,
      above,
    });
  }

  const groupCounts = Object.fromEntries(
    groups.map((g) => [g.id, cases.filter((c) => c.group === g.id).length]),
  );

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        <FilterChip active={group === "all"} onClick={() => setGroup("all")}>
          All cases · {cases.length}
        </FilterChip>
        {groups.map((g) => (
          <FilterChip
            key={g.id}
            active={group === g.id}
            onClick={() => setGroup(g.id)}
          >
            {g.label} · {groupCounts[g.id]}
          </FilterChip>
        ))}
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <FilterChip
          active={failuresOnly}
          onClick={() => setFailuresOnly((v) => !v)}
        >
          Only cases with a failure
        </FilterChip>
        <button
          type="button"
          onClick={toggleAll}
          className="tag transition-colors hover:text-fg-0"
          disabled={visibleIds.length === 0}
        >
          {allExpanded ? "Collapse all" : "Expand all"}
        </button>
      </div>

      <Legend />

      {suites.map((suite) => {
        const rows = visible.filter((c) => c.suite === suite.id);
        if (!rows.length) return null;
        const suiteGroups = groups.filter((g) =>
          rows.some((c) => c.group === g.id),
        );
        const columns = suite.setups.flatMap((arm) =>
          models.map((m) => ({ arm, model: m })),
        );
        // Not every case runs on every setup, so the run count is counted,
        // not multiplied out.
        const runCount = rows.reduce(
          (n, c) =>
            n +
            models.reduce(
              (k, m) =>
                k + Object.keys(c.runs[m.key]).filter((arm) => suite.setups.includes(arm)).length,
              0,
            ),
          0,
        );
        return (
          <div key={suite.id} className="mt-8">
            <p className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-fg-3">
              {suite.label} · {rows.length}{" "}
              {rows.length === 1 ? "case" : "cases"} on {suite.setups.length}{" "}
              setups · {models.length} models · {runCount} runs
            </p>
            <div
              className="mt-3 overflow-x-auto rounded-xl"
              style={{
                background: "var(--ink-0)",
                border: "1px solid var(--hair-2)",
              }}
            >
              <table className="w-full border-collapse text-left">
                <thead>
                  <tr>
                    <th
                      rowSpan={2}
                      scope="col"
                      className="sticky left-0 z-10 w-[190px] min-w-[190px] px-4 py-3 align-bottom font-mono text-[0.6875rem] font-medium uppercase tracking-[0.1em] text-fg-3 sm:w-[340px] sm:min-w-[340px]"
                      style={{
                        background: "var(--ink-0)",
                        borderBottom: "1px solid var(--hair-2)",
                      }}
                    >
                      Case
                    </th>
                    {suite.setups.map((arm) => (
                      <th
                        key={arm}
                        colSpan={models.length}
                        scope="colgroup"
                        className="px-2 pt-3 pb-1 text-center font-mono text-[0.6875rem] font-medium uppercase tracking-[0.1em] whitespace-nowrap text-fg-2"
                        style={{ borderLeft: "1px solid var(--hair)" }}
                      >
                        {setupLabels[arm] ?? arm}
                      </th>
                    ))}
                  </tr>
                  <tr>
                    {columns.map(({ arm, model }, i) => (
                      <th
                        key={`${arm}-${model.key}`}
                        scope="col"
                        className="px-1 pb-2 text-center font-mono text-[0.6875rem] font-medium whitespace-nowrap text-fg-2"
                        style={{
                          borderBottom: "1px solid var(--hair-2)",
                          borderLeft:
                            i % models.length === 0
                              ? "1px solid var(--hair)"
                              : undefined,
                        }}
                      >
                        <span className="inline-flex items-center gap-1">
                          <span
                            aria-hidden
                            className="inline-block size-2 rounded-full"
                            style={{ background: MODEL_COLOR[model.key] }}
                          />
                          {model.short}
                        </span>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {suiteGroups.map((g) => (
                    <Fragment key={g.id}>
                      {suiteGroups.length > 1 || suite.id !== g.id ? (
                        <tr>
                          <th
                            scope="colgroup"
                            colSpan={columns.length + 1}
                            className="px-4 pt-5 pb-2 font-mono text-[0.6875rem] font-medium uppercase tracking-[0.14em] text-fg-3"
                          >
                            <span className="sticky left-4 inline-block">
                              {g.label}
                            </span>
                          </th>
                        </tr>
                      ) : null}
                      {rows
                        .filter((c) => c.group === g.id)
                        .map((c) => {
                          const open = expanded.has(c.id);
                          return (
                            <Fragment key={c.id}>
                              <tr
                                className="group cursor-pointer transition-colors hover:bg-[color:var(--ink-1)]"
                                style={{ borderTop: "1px solid var(--hair)" }}
                                onClick={() => toggle(c.id)}
                              >
                                <th
                                  scope="row"
                                  className="sticky left-0 z-10 bg-[color:var(--ink-0)] px-4 py-2.5 align-top font-normal transition-colors group-hover:bg-[color:var(--ink-1)]"
                                >
                                  <button
                                    type="button"
                                    aria-expanded={open}
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      toggle(c.id);
                                    }}
                                    className="flex w-full items-start gap-1.5 text-left"
                                  >
                                    <ChevronRight
                                      aria-hidden
                                      className={cn(
                                        "mt-[3px] size-3.5 shrink-0 text-fg-3 transition-transform",
                                        open && "rotate-90",
                                      )}
                                    />
                                    <span className="min-w-0">
                                      <span className="block font-mono text-[0.8125rem] break-words text-fg-0">
                                        {c.id}
                                      </span>
                                      <span className="mt-0.5 line-clamp-2 text-[0.8125rem] leading-snug text-fg-2">
                                        {c.title}
                                      </span>
                                    </span>
                                  </button>
                                </th>
                                {columns.map(({ arm, model }, i) => {
                                  const run = c.runs[model.key][arm];
                                  return (
                                    <td
                                      key={`${arm}-${model.key}`}
                                      className="px-1 py-2.5 text-center align-top"
                                      style={{
                                        borderLeft:
                                          i % models.length === 0
                                            ? "1px solid var(--hair)"
                                            : undefined,
                                      }}
                                    >
                                      {run ? (
                                        <RunCell
                                          run={run}
                                          label={`${model.label}, ${setupLabels[arm] ?? arm}: ${KIND_LABEL[run.kind]}`}
                                          onClick={(e) => {
                                            e.stopPropagation();
                                            toggle(c.id);
                                          }}
                                          onShow={(el) =>
                                            showTooltip(el, c.id, model, arm, run)
                                          }
                                          onHide={() => setHover(null)}
                                        />
                                      ) : null}
                                    </td>
                                  );
                                })}
                              </tr>
                              {open ? (
                                <tr>
                                  <td colSpan={columns.length + 1} className="p-0">
                                    <CaseDetails
                                      c={c}
                                      models={models}
                                      setups={suite.setups}
                                      setupLabels={setupLabels}
                                      issueById={issueById}
                                    />
                                  </td>
                                </tr>
                              ) : null}
                            </Fragment>
                          );
                        })}
                    </Fragment>
                  ))}
                </tbody>
                <tfoot>
                  <tr style={{ borderTop: "1px solid var(--hair-2)" }}>
                    <th
                      scope="row"
                      className="sticky left-0 z-10 px-4 py-3 text-left font-mono text-[0.6875rem] font-medium uppercase tracking-[0.1em] text-fg-3"
                      style={{ background: "var(--ink-0)" }}
                    >
                      Passed{rows.length !== cases.filter((c) => c.suite === suite.id).length ? " (shown)" : ""}
                    </th>
                    {columns.map(({ arm, model }, i) => {
                      const ran = rows.filter((c) => c.runs[model.key][arm]);
                      const passed = ran.filter(
                        (c) => c.runs[model.key][arm].status === "pass",
                      ).length;
                      return (
                        <td
                          key={`${arm}-${model.key}`}
                          className="px-1 py-3 text-center font-mono text-[0.75rem] whitespace-nowrap text-fg-1 tabular-nums"
                          style={{
                            borderLeft:
                              i % models.length === 0
                                ? "1px solid var(--hair)"
                                : undefined,
                          }}
                        >
                          {passed}/{ran.length}
                        </td>
                      );
                    })}
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>
        );
      })}

      {visible.length === 0 ? (
        <p className="muted mt-8">No case matches these filters.</p>
      ) : null}

      {hover ? (
        <div
          role="tooltip"
          className="pointer-events-none fixed z-50 rounded-lg p-3.5 text-left"
          style={{
            left: hover.x,
            top: hover.y,
            width: TOOLTIP_WIDTH,
            maxWidth: "calc(100vw - 24px)",
            transform: hover.above ? "translateY(-100%)" : undefined,
            background: "var(--ink-0)",
            border: "1px solid var(--hair-2)",
            boxShadow: "0 18px 40px -20px rgba(10,14,24,0.45)",
          }}
        >
          <p className="font-mono text-[0.6875rem] uppercase tracking-[0.1em] text-fg-3">
            {hover.caseId}
          </p>
          <p className="mt-1 flex items-center gap-1.5 text-[0.875rem] font-semibold text-fg-0">
            <span
              aria-hidden
              className="inline-block size-2 rounded-full"
              style={{ background: MODEL_COLOR[hover.model.key] }}
            />
            {hover.model.label} · {setupLabels[hover.arm] ?? hover.arm}
          </p>
          <RunSummary run={hover.run} issueById={issueById} clamp />
        </div>
      ) : null}
    </div>
  );
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn("tag transition-colors", active ? "active" : "hover:text-fg-0")}
    >
      {children}
    </button>
  );
}

function Legend() {
  const items: { kind: RunKind; label: string }[] = [
    { kind: "pass", label: KIND_LABEL.pass },
    { kind: "trajectory", label: KIND_LABEL.trajectory },
    { kind: "wrong", label: KIND_LABEL.wrong },
    { kind: "budget", label: KIND_LABEL.budget },
  ];
  return (
    <div className="mt-6 flex flex-wrap items-center gap-x-6 gap-y-2 text-[0.8125rem] text-fg-2">
      {items.map((it) => (
        <span key={it.kind} className="inline-flex items-center gap-2">
          <CellGlyph kind={it.kind} />
          {it.label}
        </span>
      ))}
      <span className="inline-flex items-center gap-2">
        <span className="relative inline-flex size-[26px] items-center justify-center rounded-md border border-hair-2">
          <IssueDot />
        </span>
        Hit by a known case or skill problem
      </span>
    </div>
  );
}

function IssueDot() {
  return (
    <span
      aria-hidden
      className="absolute top-[3px] right-[3px] size-[5px] rounded-full"
      style={{ background: "var(--fg-1)" }}
    />
  );
}

function CellGlyph({ kind, issue }: { kind: RunKind; issue?: boolean }) {
  const Icon = KIND_ICON[kind];
  return (
    <span
      className="relative inline-flex size-[26px] items-center justify-center rounded-md"
      style={CELL_STYLE[kind]}
    >
      <Icon aria-hidden className="size-3.5" strokeWidth={2.5} />
      {issue ? <IssueDot /> : null}
    </span>
  );
}

function RunCell({
  run,
  label,
  onClick,
  onShow,
  onHide,
}: {
  run: CaseRun;
  label: string;
  onClick: (e: React.MouseEvent) => void;
  onShow: (el: HTMLElement) => void;
  onHide: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      onClick={onClick}
      onPointerEnter={(e) => {
        if (e.pointerType === "mouse") onShow(e.currentTarget);
      }}
      onPointerLeave={onHide}
      onFocus={(e) => {
        if (e.currentTarget.matches(":focus-visible")) onShow(e.currentTarget);
      }}
      onBlur={onHide}
      className="rounded-md p-0.5 transition-transform hover:scale-110 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[color:var(--a1)]"
    >
      <CellGlyph kind={run.kind} issue={!!run.knownIssue} />
    </button>
  );
}

function RunSummary({
  run,
  issueById,
  clamp,
}: {
  run: CaseRun;
  issueById: Record<string, MatrixIssue>;
  clamp?: boolean;
}) {
  const issue = run.knownIssue ? issueById[run.knownIssue] : null;
  return (
    <div className="mt-2 flex flex-col gap-1.5">
      <p className="flex items-center gap-2 text-[0.8125rem] text-fg-1">
        <CellGlyph kind={run.kind} />
        <span>
          {KIND_LABEL[run.kind]}
          {run.graders.length ? (
            <span className="text-fg-3"> · failed {run.graders.join(", ")}</span>
          ) : null}
        </span>
      </p>
      {run.firstFailure ? (
        <p
          className={cn(
            "font-mono text-[0.75rem] leading-relaxed break-words text-fg-1",
            clamp && "line-clamp-4",
          )}
        >
          {run.firstFailure}
        </p>
      ) : null}
      {issue ? (
        <p className="text-[0.75rem] text-fg-2">
          Known {issue.kind} problem: {issue.title}
        </p>
      ) : null}
      <p className="font-mono text-[0.6875rem] text-fg-3 tabular-nums">
        {run.toolCalls} tool calls · {formatUsd(run.costUsd)} ·{" "}
        {Math.round(run.wallS)}s
        {run.judge !== null ? ` · judge ${run.judge.toFixed(2)}` : ""}
      </p>
    </div>
  );
}

function CaseDetails({
  c,
  models,
  setups,
  setupLabels,
  issueById,
}: {
  c: BenchmarkCase;
  models: MatrixModel[];
  setups: string[];
  setupLabels: Record<string, string>;
  issueById: Record<string, MatrixIssue>;
}) {
  const all = setups.flatMap((arm) =>
    models
      .filter((m) => c.runs[m.key][arm])
      .map((m) => ({ arm, model: m, run: c.runs[m.key][arm] })),
  );
  const failed = all.filter((x) => x.run.status !== "pass");
  return (
    <div
      className="sticky left-0 w-[min(100vw_-_40px,880px)] px-4 pt-1 pb-5 sm:pl-10"
      style={{ background: "var(--ink-0)" }}
    >
      <p className="text-[0.8125rem] text-fg-2">
        {all.length - failed.length} of {all.length} runs passed.
        {failed.length ? " The failures, with the first reason the graders gave:" : ""}
      </p>
      {failed.length ? (
        <ul className="mt-3 grid gap-3 md:grid-cols-2">
          {failed.map(({ arm, model, run }) => (
            <li
              key={`${arm}-${model.key}`}
              className="rounded-lg p-3.5"
              style={{
                background: "var(--ink-1)",
                border: "1px solid var(--hair)",
              }}
            >
              <p className="flex items-center gap-1.5 text-[0.8125rem] font-semibold text-fg-0">
                <span
                  aria-hidden
                  className="inline-block size-2 rounded-full"
                  style={{ background: MODEL_COLOR[model.key] }}
                />
                {model.label} · {setupLabels[arm] ?? arm}
              </p>
              <RunSummary run={run} issueById={issueById} />
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
