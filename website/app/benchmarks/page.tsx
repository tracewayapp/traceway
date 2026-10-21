import Link from "next/link";
import type { Metadata } from "next";
import { ArrowRight, Gauge } from "lucide-react";

import { Chip } from "@/components/chip";
import { Eyebrow } from "@/components/eyebrow";
import { SectionHead } from "@/components/section-head";
import { StatsStrip } from "@/components/stats-strip";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { BenchmarkCaseMatrix } from "@/components/benchmark-case-matrix";
import {
  BenchmarkCostChart,
  type CostPoint,
} from "@/components/benchmark-cost-chart";
import { BENCHMARK } from "@/lib/benchmark-data";
import {
  MODEL_COLOR,
  formatPp,
  formatUsd,
  numberWord,
  rate,
} from "@/lib/benchmark-format";
import type { Model, ModelKey } from "@/lib/benchmarks";

const { run, models, suites, setups, comparisons, groups, knownIssues, cases } =
  BENCHMARK;

const modelByKey = Object.fromEntries(models.map((m) => [m.key, m])) as Record<
  ModelKey,
  Model
>;
const setupLabels = Object.fromEntries(setups.map((s) => [s.arm, s.label]));
const suiteLabel = Object.fromEntries(suites.map((s) => [s.id, s.label]));
const groupById = Object.fromEntries(groups.map((g) => [g.id, g]));
const capitalize = (s: string) => s[0].toUpperCase() + s.slice(1);
const joinList = (items: string[]) =>
  items.length < 2
    ? items.join("")
    : `${items.slice(0, -1).join(", ")} and ${items[items.length - 1]}`;
const modelCount = numberWord(models.length);
const modelCountTitle = capitalize(modelCount);

export const metadata: Metadata = {
  title: "Benchmarks · Traceway",
  description: `How well do AI agents debug production with Traceway? ${run.runsPerModel} seeded production failures, ${modelCount} models (${models.map((m) => m.label).join(", ")}), graded by deterministic code. Pass rates, costs and every case.`,
};

const CARD_STYLE = {
  background: "var(--ink-0)",
  border: "1px solid var(--hair-2)",
  boxShadow:
    "0 1px 2px rgba(10,14,24,0.04), 0 12px 30px -18px rgba(10,14,24,0.14)",
};

const GRADERS = [
  {
    name: "answer",
    gates: true,
    body: "The structured answer names the right checklist row and endpoint, and not the case's named distractor.",
  },
  {
    name: "grounding",
    gates: true,
    body: "Every claim cites a tool call id whose output exists, succeeded and contains the signal.",
  },
  {
    name: "trajectory",
    gates: true,
    body: "The required, forbidden and ordered calls.",
  },
  {
    name: "report",
    gates: true,
    body: "The required statements appear in the report.",
  },
  {
    name: "state",
    gates: true,
    body: "For write cases, the system is exactly right afterwards.",
  },
  {
    name: "judge",
    gates: false,
    body: `${run.judge.label} scores the report from 0 to 1 against a rubric. It never gates.`,
  },
];

function totalsOf(key: ModelKey) {
  return modelByKey[key].totals;
}

function setupResult(suite: string, arm: string, key: ModelKey) {
  return setups.find((s) => s.suite === suite && s.arm === arm)!.results[key];
}

function pickBy(score: (m: Model) => number, best: "max" | "min") {
  return models.reduce((a, b) =>
    best === "max"
      ? score(b) > score(a)
        ? b
        : a
      : score(b) < score(a)
        ? b
        : a,
  );
}

const best = pickBy((m) => m.totals.pass, "max");
const value = pickBy((m) => m.totals.costUsd / m.totals.pass, "min");
const upliftOf = (key: ModelKey) =>
  comparisons.skillUplift.find((u) => u.model === key)!;
const biggestUplift = pickBy((m) => upliftOf(m.key).pp, "max");
const cheapest = pickBy((m) => m.totals.costUsd, "min");
const priciest = pickBy((m) => m.totals.costUsd, "max");

const BADGES: Partial<Record<ModelKey, string[]>> = {};
for (const [m, label] of [
  [best, "Highest pass rate"],
  [value, "Lowest cost per pass"],
  [biggestUplift, "Biggest skill uplift"],
] as const) {
  (BADGES[m.key] ??= []).push(label);
}

const controls = groupById["perf-ctl"];
const byControls = [...models].sort(
  (a, b) => controls.results[b.key].pass - controls.results[a.key].pass,
);
const mcpPerf = (key: ModelKey) =>
  comparisons.cliVsMcp.find((c) => c.suite === "perf" && c.model === key)!;
const mcpFavoured = models.filter((m) => mcpPerf(m.key).pp < 0);
const upliftOrder = [...models].sort(
  (a, b) => upliftOf(b.key).pp - upliftOf(a.key).pp,
);

const costPoints: CostPoint[] = [
  ...setups.flatMap((s) =>
    models.map((m) => ({
      id: `${s.suite}-${s.arm}-${m.key}`,
      model: m.key,
      modelLabel: m.label,
      modelShort: m.short,
      label: `${suiteLabel[s.suite]} · ${s.label}`,
      pass: s.results[m.key].pass,
      runs: s.results[m.key].runs,
      costPerRunUsd: s.results[m.key].costPerRunUsd,
      overall: false,
    })),
  ),
  ...models.map((m) => ({
    id: `all-${m.key}`,
    model: m.key,
    modelLabel: m.label,
    modelShort: m.short,
    label: `All ${m.totals.runs} runs`,
    pass: m.totals.pass,
    runs: m.totals.runs,
    costPerRunUsd: m.totals.costPerRunUsd,
    overall: true,
  })),
];

const maxCost = Math.max(...models.map((m) => m.totals.costUsd));
const suiteSelection = suites
  .map(
    (s) =>
      `the ${s.id} suite (${s.cases} cases × ${s.setups.length} setups = ${s.cases * s.setups.length} runs)`,
  )
  .join(" plus ");
const issueRuns = (key: ModelKey) =>
  knownIssues.reduce((n, i) => n + i.byModel[key], 0);
const issueGroups = groups
  .filter((g) => knownIssues.some((i) => cases.find((c) => c.id === i.case)?.group === g.id))
  .map((g) => g.label);

export default function BenchmarksPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <div className="flex flex-col items-center text-center">
            <Chip>
              <Gauge className="h-3 w-3 inline mr-1" />
              Agent benchmarks
            </Chip>
            <h1 className="mt-6 max-w-[24ch]">
              The same {run.runsPerModel} broken production systems.{" "}
              <em>{modelCountTitle} models.</em>
            </h1>
            <p className="hero-sub text-pretty">
              Each run gets a fresh Traceway backend seeded with the telemetry
              of a system broken in one specific way, and a prompt like
              &ldquo;checkout got slow, find out why&rdquo;. Code grades the
              diagnosis. Here is every result.
            </p>
            <div className="hero-cta-row justify-center">
              <Link
                href={run.methodologyUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-accent"
              >
                How we measure
                <ArrowRight className="h-4 w-4" />
              </Link>
              <Link href="#matrix" className="btn btn-ghost">
                See every case
              </Link>
            </div>
            <p className="dim mt-6 font-mono text-[0.75rem]">
              Run {run.date} · commit {run.commit} · {run.runsPerCase} run per
              case
            </p>
            <p className="mt-3 flex flex-wrap justify-center gap-x-6 gap-y-2 font-mono text-[0.6875rem] uppercase tracking-[0.08em]">
              <Link
                href={run.harnessUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-fg-2 hover:text-blue-bright transition-colors"
              >
                The eval harness on GitHub →
              </Link>
            </p>
          </div>
          <StatsStrip
            className="mx-auto max-w-4xl"
            stats={[
              { num: `<em>${run.totalRuns}</em>`, label: "Graded runs" },
              {
                num: `<em>${suites.reduce((n, s) => n + s.cases, 0)}</em>`,
                label: "Cases",
              },
              { num: `<em>${setups.length}</em>`, label: "Setups" },
              { num: `<em>${models.length}</em>`, label: "Models" },
            ]}
          />
        </div>
      </section>

      <div className="band-light">
        <section id="results" className="wrap scroll-mt-20 pt-16 pb-10 md:pt-20">
          <SectionHead
            eyebrow="Headline"
            title={
              <>
                Pass rate, <em>model by model.</em>
              </>
            }
            description={`Every model ran the same ${run.runsPerModel} runs. A run passes when its answer is right, its evidence is grounded and its writes are safe. Tool-call counts are reported but never gate.`}
          />
          <div className="grid gap-5 md:grid-cols-3">
            {models.map((m) => (
              <ModelCard key={m.key} model={m} badges={BADGES[m.key] ?? []} />
            ))}
          </div>
        </section>

        <section id="findings" className="wrap scroll-mt-20 py-16 md:py-20">
          <SectionHead
            eyebrow="Key findings"
            title={
              <>
                What the runs say, <em>and what they don&apos;t.</em>
              </>
            }
          />
          <div className="grid gap-12 md:grid-cols-2">
            <Finding label="01 · Judgment" title={`${byControls[0].short} never false-alarms.`}>
              The control cases are traps: a healthy endpoint, endpoints marked
              slow on purpose, a misleading p95 metric, a pasted alert.{" "}
              {byControls[0].short} passed {controls.results[byControls[0].key].pass}{" "}
              of {controls.results[byControls[0].key].runs} control runs.{" "}
              {joinList(
                byControls
                  .slice(1)
                  .map((m) => `${m.short} passed ${controls.results[m.key].pass}`),
              )}
              . The gap between the models is in judgment, not in reading
              telemetry.
            </Finding>
            <Finding label="02 · Value" title={`${value.short} is the value pick.`}>
              {rate(value.totals.pass, value.totals.runs)}% of its runs passed for{" "}
              {formatUsd(value.totals.costUsd)} of agent spend, against{" "}
              {rate(totalsOf("haiku").pass, totalsOf("haiku").runs)}% for{" "}
              {formatUsd(totalsOf("haiku").costUsd)} with {modelByKey.haiku.short}:{" "}
              {value.totals.pass - totalsOf("haiku").pass} more passing runs at
              about the same price. {best.short} passed{" "}
              {best.totals.pass - value.totals.pass} more again, for{" "}
              {Math.round(best.totals.costUsd / value.totals.costUsd)}× the
              spend. On MCP perf, {value.short} is{" "}
              {mcpPerf(best.key).mcp - mcpPerf(value.key).mcp}pp behind{" "}
              {best.short}.
            </Finding>
            <Finding
              label="03 · The skill"
              title="The skill helps the weakest model most."
            >
              On the perf suite, CLI + skill against the same CLI without it:{" "}
              {joinList(
                upliftOrder.map((m) => `${m.short} ${formatPp(upliftOf(m.key).pp)}`),
              )}
              .{" "}
              {mcpFavoured.length === 1
                ? `${mcpFavoured[0].short} is the only model that does better on MCP (${mcpPerf(mcpFavoured[0].key).mcp}%) than on CLI + skill (${mcpPerf(mcpFavoured[0].key).cli}%).`
                : null}
            </Finding>
            <Finding label="04 · Our bugs" title="Some failures are ours.">
              {capitalize(numberWord(knownIssues.length))} problems behind
              these failures are in our own cases and skill, not in the models.
              They account for{" "}
              {joinList(
                models.map(
                  (m, i) =>
                    `${issueRuns(m.key)} of ${m.short}'s ${m.totals.runs - m.totals.pass}${i === 0 ? " failures" : ""}`,
                ),
              )}
              . We still count them as failures. Here they are.
            </Finding>
          </div>

          <div className="mt-14 grid gap-5 md:grid-cols-2">
            {knownIssues.map((issue) => (
              <div key={issue.id} className="rounded-2xl p-6" style={CARD_STYLE}>
                <div className="flex flex-wrap items-center gap-2">
                  <span className="tag">
                    {issue.kind === "skill" ? "Skill bug" : "Case bug"}
                  </span>
                  <code>{issue.case}</code>
                </div>
                <h3 className="mt-4 text-[1.0625rem] font-semibold text-fg-0">
                  {issue.title}
                </h3>
                <p className="muted mt-2 text-[0.9375rem] text-pretty">
                  {issue.detail}
                </p>
                <p className="mt-4 font-mono text-[0.75rem] text-fg-2">
                  {issue.truncated
                    ? `${issue.affectedRuns} of ${issue.caseRuns} runs failed, ${issue.truncated} of them on truncated output. `
                    : ""}
                  Runs hit:{" "}
                  {models.map((m) => `${m.short} ${issue.byModel[m.key]}`).join(" · ")}
                </p>
              </div>
            ))}
          </div>
        </section>

        <section id="setups" className="wrap scroll-mt-20 py-16 md:py-20">
          <SectionHead
            eyebrow="By setup"
            title={
              <>
                Pass rate by suite <em>and setup.</em>
              </>
            }
            description="Perf cases run with the CLI and skill, the CLI alone as a control group, and the MCP server. Write cases run with the write CLI and skill, and the write MCP server."
          />
          <DataTable minWidth={760}>
            <thead>
              <tr>
                <Th>Setup</Th>
                {models.map((m) => (
                  <ModelTh key={m.key} model={m} />
                ))}
              </tr>
            </thead>
            <tbody>
              {suites.map((s) => (
                <SuiteRows key={s.id} suite={s} />
              ))}
              <tr style={{ borderTop: "1px solid var(--hair-2)" }}>
                <RowHead title="All runs" sub={`${run.runsPerModel} per model`} />
                {models.map((m) => (
                  <Td key={m.key}>
                    <PassCell
                      model={m.key}
                      pass={m.totals.pass}
                      runs={m.totals.runs}
                      rightAnswerEvidence={m.totals.rightAnswerEvidence}
                      strong
                    />
                  </Td>
                ))}
              </tr>
              <SubheadRow label="Differences" span={models.length + 1} />
              <DeltaRow
                title="Skill uplift, perf"
                sub="CLI + skill minus CLI without the skill"
                cells={models.map((m) => {
                  const u = upliftOf(m.key);
                  return { key: m.key, pp: u.pp, detail: `${u.with}% vs ${u.without}%` };
                })}
              />
              {suites.map((s) => (
                <DeltaRow
                  key={s.id}
                  title={`CLI vs MCP, ${s.id}`}
                  sub={`${setupLabels[s.setups[0]]} minus ${setupLabels[s.setups[s.setups.length - 1]]}`}
                  cells={models.map((m) => {
                    const c = comparisons.cliVsMcp.find(
                      (x) => x.suite === s.id && x.model === m.key,
                    )!;
                    return { key: m.key, pp: c.pp, detail: `${c.cli}% vs ${c.mcp}%` };
                  })}
                />
              ))}
            </tbody>
          </DataTable>
          <p className="dim mt-4 text-[0.8125rem]">
            &ldquo;Right answer + evidence&rdquo; counts runs whose answer and
            grounding passed, whatever happened to the trajectory check. The
            differences are in percentage points, computed before rounding.
          </p>
        </section>

        <section id="groups" className="wrap scroll-mt-20 py-16 md:py-20">
          <SectionHead
            eyebrow="By case group"
            title={
              <>
                Where each model <em>wins and loses.</em>
              </>
            }
            description="Passes over every setup a case ran on. Groups follow the case names: perf-db-n-plus-1 is in Database, perf-ctl-healthy in Controls and traps."
          />
          <DataTable minWidth={820}>
            <thead>
              <tr>
                <Th>Group</Th>
                {models.map((m) => (
                  <ModelTh key={m.key} model={m} />
                ))}
              </tr>
            </thead>
            <tbody>
              {groups.map((g) => {
                const top = Math.max(...models.map((m) => g.results[m.key].pass));
                return (
                  <tr key={g.id} style={{ borderTop: "1px solid var(--hair)" }}>
                    <RowHead
                      title={g.label}
                      code={g.id === "write" ? "write-*" : `${g.id}-*`}
                      sub={`${g.cases} cases · ${g.results[models[0].key].runs} runs per model`}
                      description={g.description}
                    />
                    {models.map((m) => (
                      <Td key={m.key}>
                        <PassCell
                          model={m.key}
                          pass={g.results[m.key].pass}
                          runs={g.results[m.key].runs}
                          strong={g.results[m.key].pass === top}
                        />
                      </Td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </DataTable>
          <p className="dim mt-4 text-[0.8125rem]">
            The group&apos;s best count is set in bold. Groups hold{" "}
            {Math.min(...groups.map((g) => g.cases))} to{" "}
            {Math.max(...groups.map((g) => g.cases))} cases, so a single run
            moves a small group a lot. {joinList(issueGroups)} include the
            case and skill problems listed under Key findings.
          </p>
        </section>

        <section id="cost" className="wrap scroll-mt-20 py-16 md:py-20">
          <SectionHead
            eyebrow="Cost and efficiency"
            title={
              <>
                What a diagnosis <em>costs.</em>
              </>
            }
            description={`Agent spend is the model cost the harness recorded for each run, through OpenRouter. The judge's spend is separate. ${priciest.short} cost ${Math.round(priciest.totals.costUsd / cheapest.totals.costUsd)}× as much as ${cheapest.short} for the same ${run.runsPerModel} runs.`}
          />

          <div className="cost-wrap" data-cost-mount>
            {models.map((m) => (
              <div key={m.key} className="cost-bars">
                <div className="cost-name">
                  <span
                    aria-hidden
                    className="inline-block size-2.5 shrink-0 rounded-full"
                    style={{ background: MODEL_COLOR[m.key] }}
                  />
                  {m.short}
                </div>
                <div className="cost-bar h-6">
                  <div
                    className="fill"
                    data-w={((100 * m.totals.costUsd) / maxCost).toFixed(2)}
                    style={{
                      width: `${((100 * m.totals.costUsd) / maxCost).toFixed(2)}%`,
                      minWidth: 3,
                      background: MODEL_COLOR[m.key],
                      borderRadius: "0 4px 4px 0",
                    }}
                  />
                </div>
                <div className="cost-total">
                  <em>{formatUsd(m.totals.costUsd)}</em>
                  <span className="block text-[0.75rem] text-fg-3">
                    {formatUsd(m.totals.costPerRunUsd)} per run
                  </span>
                </div>
              </div>
            ))}
          </div>

          <div className="mt-14 rounded-2xl p-5 sm:p-7" style={CARD_STYLE}>
            <h3 className="text-[1.0625rem] font-semibold text-fg-0">
              Cost per run against pass rate
            </h3>
            <p className="muted mt-1 text-[0.9375rem]">
              Up is better, left is cheaper. Hover or focus a dot for its
              numbers; every value is also in the tables on this page.
            </p>
            <div className="mt-5">
              <BenchmarkCostChart
                points={costPoints}
                models={models.map((m) => ({ key: m.key, label: m.label }))}
              />
            </div>
          </div>

          <div className="mt-10">
            <DataTable minWidth={700}>
              <thead>
                <tr>
                  <Th>All {run.runsPerModel} runs</Th>
                  {models.map((m) => (
                    <ModelTh key={m.key} model={m} />
                  ))}
                </tr>
              </thead>
              <tbody>
                <MetricRow label="Agent spend" values={models.map((m) => formatUsd(m.totals.costUsd))} />
                <MetricRow label="Average cost per run" values={models.map((m) => formatUsd(m.totals.costPerRunUsd))} />
                <MetricRow label="Cost per passing run" values={models.map((m) => formatUsd(m.totals.costUsd / m.totals.pass))} />
                <MetricRow label="Average tool calls per run" values={models.map((m) => m.totals.avgToolCalls.toFixed(1))} />
                <MetricRow label="Average wall time per run" values={models.map((m) => `${m.totals.avgWallS.toFixed(1)}s`)} />
                <MetricRow label="Cache hit rate (input tokens)" values={models.map((m) => `${Math.round(100 * m.totals.cacheHitRate)}%`)} />
                <MetricRow
                  label="Average judge score (0–1)"
                  values={models.map((m) =>
                    m.totals.avgJudge === null ? "–" : m.totals.avgJudge.toFixed(2),
                  )}
                />
                <MetricRow label="Judge spend" values={models.map((m) => formatUsd(m.totals.judgeCostUsd))} />
              </tbody>
            </DataTable>
            <p className="dim mt-4 text-[0.8125rem]">
              Runs went three at a time on one machine, so wall time includes
              some contention.{" "}
              {models
                .filter((m) => m.totals.judged < m.totals.runs)
                .map(
                  (m) =>
                    `The judge scored ${m.totals.judged} of ${m.short}'s ${m.totals.runs} runs.`,
                )
                .join(" ")}
            </p>
          </div>
        </section>

        <section id="matrix" className="wrap scroll-mt-20 py-16 md:py-20">
          <SectionHead
            eyebrow="Every case"
            title={
              <>
                The full matrix: <em>every case, setup and model.</em>
              </>
            }
            description="One cell per run. Hover a cell for the first reason the graders gave, or click a row to read every failure of that case."
          />
          <BenchmarkCaseMatrix
            cases={cases}
            models={models.map((m) => ({ key: m.key, label: m.label, short: m.short }))}
            suites={suites.map((s) => ({ id: s.id, label: s.label, setups: s.setups }))}
            groups={groups.map((g) => ({ id: g.id, label: g.label }))}
            setupLabels={setupLabels}
            issues={knownIssues.map((i) => ({ id: i.id, kind: i.kind, title: i.title }))}
          />
        </section>
      </div>

      <section id="methodology" className="wrap scroll-mt-20 pt-20 pb-6">
        <SectionHead
          eyebrow="Methodology"
          title={
            <>
              How the evals <em>work.</em>
            </>
          }
          description="Traceway ships an agent skill, markdown that teaches an agent to debug production with the traceway CLI, and an MCP server with the same knowledge plus tools. The evals measure how well agents use them."
          cta={
            <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
              <Link
                href={run.methodologyUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-accent"
              >
                How we measure
                <ArrowRight className="h-4 w-4" />
              </Link>
              <Link
                href={run.harnessUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="font-mono text-[0.6875rem] uppercase tracking-[0.08em] text-fg-2 transition-colors hover:text-blue-bright"
              >
                The eval harness on GitHub →
              </Link>
            </div>
          }
        />
        <div className="grid gap-x-14 gap-y-12 md:grid-cols-2">
          <MethodBlock label="The cases">
            <p className="muted text-pretty">
              Each run gets a fresh Traceway backend, seeded with realistic
              telemetry of a system broken in one specific way: an N+1 query,
              pool exhaustion, a noisy neighbor, a cache stampede, GC pauses.
              The model gets a prompt like &ldquo;checkout got slow, find out
              why&rdquo; and has to diagnose it. Write cases ask it to act:
              acknowledge a page, snooze an alert, run a monitor, archive
              issues, write a post-mortem.
            </p>
            <p className="muted mt-4 text-pretty">
              This run covers {suiteSelection}: {run.runsPerModel} runs per
              model, {run.runsPerCase} per case.
              {run.allGraded
                ? " Every run was graded; none hit an infra error, its budget or the cost ceiling."
                : ""}
            </p>
          </MethodBlock>

          <MethodBlock label="The setups">
            <ul className="feat-bullets mt-0!">
              {setups.map((s) => (
                <li key={`${s.suite}-${s.arm}`}>
                  <span>
                    <code>{s.arm}</code> {s.description}
                  </span>
                </li>
              ))}
            </ul>
          </MethodBlock>

          <MethodBlock label="The graders">
            <ul className="feat-bullets mt-0!">
              {GRADERS.map((g) => (
                <li key={g.name}>
                  <span>
                    <code>{g.name}</code> {g.body}
                  </span>
                </li>
              ))}
            </ul>
            <p className="muted mt-4 text-pretty">
              The graders are deterministic code, apart from the judge. A run
              passes when its answer is right, its evidence is grounded and its
              writes are safe. Tool-call counts are reported but never gate.
            </p>
          </MethodBlock>

          <MethodBlock label="The models">
            <ul className="feat-bullets mt-0!">
              {models.map((m) => (
                <li key={m.key}>
                  <span>
                    {m.label} <code>{m.id}</code>, served by{" "}
                    {m.providers[0].name} on {m.providers[0].runs} of{" "}
                    {m.totals.runs} runs
                    {m.providers.length > 1
                      ? `; ${m.providers
                          .slice(1)
                          .map((p) => `${p.runs} also had some turns on ${p.name}`)
                          .join(", ")}`
                      : ""}
                    .
                  </span>
                </li>
              ))}
            </ul>
            <p className="muted mt-4 text-pretty">
              All through OpenRouter with the same settings: reasoning effort{" "}
              {models[0].reasoningEffort},{" "}
              {(models[0].maxTokensPerTurn / 1000).toFixed(0)}k max tokens per
              turn, the same prompts, tools and budgets.
            </p>
          </MethodBlock>

          <MethodBlock label="Caveats" className="md:col-span-2">
            <ul className="feat-bullets mt-0!">
              <li>
                <span>
                  One run per case, so a single cell is noisy. Read the totals
                  and the groups before any one cell.
                </span>
              </li>
              <li>
                <span>
                  Local runs on one machine, not the CI workflow&apos;s
                  baselines.
                </span>
              </li>
              <li>
                <span>
                  The debug and setup suites, which fix code and instrument
                  apps, need Docker and were not run.
                </span>
              </li>
              <li>
                <span>
                  The judge is {run.judge.label} for every model, including
                  when it grades its own runs. It never gates a pass.
                </span>
              </li>
              <li>
                <span>
                  Some failures are our own case and skill bugs, listed under
                  Key findings. They are counted as failures here.
                </span>
              </li>
              <li>
                <span>
                  All runs on {run.date}, at commit <code>{run.commit}</code> (
                  {run.cliVersion}, {run.backendVersion}).{" "}
                  <Link
                    href={run.harnessUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-blue-bright hover:underline"
                  >
                    Read the harness and the cases on GitHub →
                  </Link>
                </span>
              </li>
            </ul>
          </MethodBlock>
        </div>
      </section>

      <FinalCTA
        title={
          <>
            Give your agent <em>production context.</em>
          </>
        }
        description="Install the skill or connect the MCP server, and let your agent take the next slow endpoint."
        primary={{ label: "Explore Agent Skills", href: "/product/agent-skills" }}
        secondary={{ label: "See the MCP server", href: "/product/mcp" }}
      />
    </main>
  );
}

function ModelSwatch({ model, size = 10 }: { model: ModelKey; size?: number }) {
  return (
    <span
      aria-hidden
      className="inline-block shrink-0 rounded-full"
      style={{ width: size, height: size, background: MODEL_COLOR[model] }}
    />
  );
}

function ModelCard({ model, badges }: { model: Model; badges: string[] }) {
  const t = model.totals;
  const pct = rate(t.pass, t.runs);
  return (
    <div className="flex flex-col rounded-[20px] px-6 py-7" style={CARD_STYLE}>
      <div className="flex items-center gap-2.5">
        <ModelSwatch model={model.key} size={12} />
        <span className="text-[17px] font-semibold text-fg-0">{model.label}</span>
      </div>
      <p className="mt-1.5 font-mono text-[0.75rem] break-all text-fg-3">
        {model.id}
      </p>
      <div className="mt-6 flex items-baseline gap-2">
        <span className="text-[48px] leading-none font-semibold tracking-[-0.03em] text-fg-0">
          {pct}%
        </span>
        <span className="text-[0.875rem] text-fg-2">
          {t.pass} of {t.runs} runs passed
        </span>
      </div>
      <div
        className="mt-4 h-1.5 overflow-hidden rounded-full"
        style={{ background: "var(--ink-3)" }}
        role="img"
        aria-label={`${pct}% pass rate`}
      >
        <div
          className="h-full rounded-full"
          style={{ width: `${(100 * t.pass) / t.runs}%`, background: MODEL_COLOR[model.key] }}
        />
      </div>
      <dl className="mt-6 grid gap-3 text-[0.875rem]">
        <CardStat label="Right answer with evidence" value={`${t.rightAnswerEvidence}/${t.runs}`} />
        <CardStat label="Agent cost" value={formatUsd(t.costUsd)} />
        <CardStat label="Cost per run" value={formatUsd(t.costPerRunUsd)} />
      </dl>
      {badges.length ? (
        <div className="mt-6 flex flex-wrap gap-2">
          {badges.map((b) => (
            <span key={b} className="tag">
              {b}
            </span>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function CardStat({ label, value }: { label: string; value: string }) {
  return (
    <div
      className="flex items-baseline justify-between gap-4 pt-3"
      style={{ borderTop: "1px solid var(--hair)" }}
    >
      <dt className="text-fg-2">{label}</dt>
      <dd className="m-0 font-mono text-fg-0 tabular-nums">{value}</dd>
    </div>
  );
}

function Finding({
  label,
  title,
  children,
}: {
  label: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="border-t border-hair pt-8">
      <p className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-fg-3">
        {label}
      </p>
      <h3 className="mt-3">{title}</h3>
      <p className="muted mt-3 max-w-[520px] text-pretty">{children}</p>
    </div>
  );
}

function DataTable({
  minWidth,
  children,
}: {
  minWidth: number;
  children: React.ReactNode;
}) {
  return (
    <div className="overflow-x-auto rounded-2xl" style={CARD_STYLE}>
      <table className="w-full border-collapse text-left" style={{ minWidth }}>
        {children}
      </table>
    </div>
  );
}

function Th({ children }: { children: React.ReactNode }) {
  return (
    <th
      scope="col"
      className="px-5 py-3.5 align-bottom font-mono text-[0.6875rem] font-medium uppercase tracking-[0.1em] text-fg-3"
      style={{ borderBottom: "1px solid var(--hair-2)" }}
    >
      {children}
    </th>
  );
}

function ModelTh({ model }: { model: Model }) {
  return (
    <th
      scope="col"
      className="px-5 py-3.5 align-bottom text-[0.8125rem] font-semibold text-fg-0"
      style={{ borderBottom: "1px solid var(--hair-2)" }}
    >
      <span className="inline-flex items-center gap-2 whitespace-nowrap">
        <ModelSwatch model={model.key} />
        {model.label}
      </span>
    </th>
  );
}

function Td({ children }: { children: React.ReactNode }) {
  return <td className="px-5 py-4 align-top">{children}</td>;
}

function RowHead({
  title,
  sub,
  code,
  description,
}: {
  title: string;
  sub?: string;
  code?: string;
  description?: string;
}) {
  return (
    <th scope="row" className="min-w-[190px] px-5 py-4 align-top font-normal">
      <span className="block text-[0.9375rem] font-semibold text-fg-0">
        {title}
      </span>
      {code ? (
        <span className="mt-0.5 block font-mono text-[0.75rem] text-fg-3">
          {code}
        </span>
      ) : null}
      {description ? (
        <span className="mt-1.5 block max-w-[300px] text-[0.8125rem] leading-snug text-fg-2">
          {description}
        </span>
      ) : null}
      {sub ? (
        <span className="mt-1 block font-mono text-[0.75rem] text-fg-3">{sub}</span>
      ) : null}
    </th>
  );
}

function SubheadRow({ label, span }: { label: string; span: number }) {
  return (
    <tr style={{ borderTop: "1px solid var(--hair-2)" }}>
      <th
        scope="colgroup"
        colSpan={span}
        className="px-5 pt-5 pb-1 font-mono text-[0.6875rem] font-medium uppercase tracking-[0.14em] text-fg-3"
      >
        {label}
      </th>
    </tr>
  );
}

function SuiteRows({
  suite,
}: {
  suite: (typeof suites)[number];
}) {
  return (
    <>
      <SubheadRow
        label={`${suite.label} · ${suite.cases} cases`}
        span={models.length + 1}
      />
      {suite.setups.map((arm) => (
        <tr key={arm} style={{ borderTop: "1px solid var(--hair)" }}>
          <RowHead title={setupLabels[arm]} code={arm} />
          {models.map((m) => {
            const r = setupResult(suite.id, arm, m.key);
            return (
              <Td key={m.key}>
                <PassCell
                  model={m.key}
                  pass={r.pass}
                  runs={r.runs}
                  rightAnswerEvidence={r.rightAnswerEvidence}
                />
              </Td>
            );
          })}
        </tr>
      ))}
    </>
  );
}

function PassCell({
  model,
  pass,
  runs,
  rightAnswerEvidence,
  strong,
}: {
  model: ModelKey;
  pass: number;
  runs: number;
  rightAnswerEvidence?: number;
  strong?: boolean;
}) {
  return (
    <div className="min-w-[140px]">
      <div className="flex items-baseline justify-between gap-3 font-mono tabular-nums">
        <span
          className={
            strong
              ? "text-[0.9375rem] font-semibold text-fg-0"
              : "text-[0.9375rem] text-fg-1"
          }
        >
          {pass}/{runs}
        </span>
        <span className="text-[0.8125rem] text-fg-2">{rate(pass, runs)}%</span>
      </div>
      <div
        className="mt-2 h-1.5 overflow-hidden rounded-full"
        style={{ background: "var(--ink-3)" }}
      >
        <div
          className="h-full rounded-full"
          style={{ width: `${(100 * pass) / runs}%`, background: MODEL_COLOR[model] }}
        />
      </div>
      {rightAnswerEvidence !== undefined ? (
        <p className="mt-2 font-mono text-[0.6875rem] whitespace-nowrap text-fg-3 tabular-nums">
          {rightAnswerEvidence}/{runs} right answer + evidence
        </p>
      ) : null}
    </div>
  );
}

function DeltaRow({
  title,
  sub,
  cells,
}: {
  title: string;
  sub: string;
  cells: { key: ModelKey; pp: number; detail: string }[];
}) {
  return (
    <tr style={{ borderTop: "1px solid var(--hair)" }}>
      <RowHead title={title} sub={sub} />
      {cells.map((c) => (
        <Td key={c.key}>
          <span className="block font-mono text-[0.9375rem] font-semibold text-fg-0 tabular-nums">
            {formatPp(c.pp)}
          </span>
          <span className="mt-1 block font-mono text-[0.6875rem] text-fg-3 tabular-nums">
            {c.detail}
          </span>
        </Td>
      ))}
    </tr>
  );
}

function MetricRow({ label, values }: { label: string; values: string[] }) {
  return (
    <tr style={{ borderTop: "1px solid var(--hair)" }}>
      <th scope="row" className="min-w-[190px] px-5 py-3.5 text-[0.9375rem] font-normal text-fg-1">
        {label}
      </th>
      {values.map((v, i) => (
        <td
          key={models[i].key}
          className="px-5 py-3.5 font-mono text-[0.9375rem] text-fg-0 tabular-nums"
        >
          {v}
        </td>
      ))}
    </tr>
  );
}

function MethodBlock({
  label,
  className,
  children,
}: {
  label: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div className={className}>
      <Eyebrow>{label}</Eyebrow>
      <div className="mt-4">{children}</div>
    </div>
  );
}
