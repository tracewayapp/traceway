#!/usr/bin/env node
// Builds content/benchmarks/<date>.md, the only source of numbers for
// app/benchmarks, from a three-model `twevals` comparison directory
// (runs.csv, results.json, summary.md, stats.md) and the eval case files.
// Before writing, every aggregate is checked against summary.md and stats.md;
// after writing, the file is parsed back with the page's own parser and
// checked again. Any mismatch aborts.
//
//   node scripts/build-benchmarks-data.mjs --source <comparison dir> \
//     [--cases ../cli/test/evals/cases] [--models ../cli/test/evals/models.yaml] \
//     [--out content/benchmarks/<date>.md]

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  COST_METRICS,
  MATRIX_KIND,
  SECTIONS,
  escapeCell,
  parseBenchmark,
} from "../lib/benchmarks.ts";

const WEBSITE = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const HARNESS_URL =
  "https://github.com/tracewayapp/traceway/tree/main/cli/test/evals";
const METHODOLOGY_URL = "https://docs.tracewayapp.com/learn/agent-benchmarks";
const PAGE_URL = "https://tracewayapp.com/benchmarks";

const MODEL_ORDER = ["haiku", "deepseek-flash", "opus"];
const MODEL_SHORT = { haiku: "Haiku", "deepseek-flash": "DeepSeek", opus: "Opus" };
const MODEL_VENDOR = { haiku: "Anthropic", "deepseek-flash": "DeepSeek", opus: "Anthropic" };
const MODEL_WEIGHTS = { haiku: "proprietary", "deepseek-flash": "open", opus: "proprietary" };

const SUITES = [
  {
    id: "perf",
    label: "Performance",
    description: "Diagnose a latency problem from production telemetry.",
  },
  {
    id: "write",
    label: "Write",
    description:
      "Act on the system safely: pages, alerts, monitors, archiving and post-mortems.",
  },
];

const SETUPS = {
  "cli+skill": {
    label: "CLI + skill",
    description: "The traceway CLI plus the Traceway agent skill.",
  },
  "cli-noskill": {
    label: "CLI, no skill",
    description: "The same CLI with no skill: the control group.",
  },
  mcp: {
    label: "MCP server",
    description: "The Traceway MCP server: the same knowledge, plus tools.",
  },
  "cli-write+skill": {
    label: "Write CLI + skill",
    description: "The traceway-write CLI plus the write skill.",
  },
  "mcp-write": {
    label: "Write MCP server",
    description: "The write MCP server.",
  },
};

const GROUPS = [
  {
    id: "perf-app",
    label: "App code",
    description:
      "Slow code in the request path: serialization, quadratic loops, large allocations, debug logging, work that belongs in the background.",
  },
  {
    id: "perf-cache",
    label: "Caching",
    description: "Cold starts after a release, a missing cache, a cache stampede.",
  },
  {
    id: "perf-ctl",
    label: "Controls and traps",
    description:
      "A healthy endpoint, endpoints marked slow on purpose, a misleading metric, a pasted alert and an onset to pin down. Some have nothing to fix.",
  },
  {
    id: "perf-db",
    label: "Database",
    description:
      "N+1 queries, a missing index, pool exhaustion, lock contention, over-fetching, unbounded results.",
  },
  {
    id: "perf-infra",
    label: "Infrastructure",
    description: "CPU saturation, disk I/O, memory pressure, a noisy neighbor.",
  },
  {
    id: "perf-net",
    label: "Network and upstreams",
    description:
      "Retries, chatty fan-out, sequential calls, large payloads, connection reuse, a slow third party.",
  },
  {
    id: "perf-xs",
    label: "Cross-stack and mobile",
    description:
      "A browser or mobile app fails, and the agent has to follow it into the backend.",
  },
  {
    id: "write",
    label: "Write actions",
    description:
      "Acknowledge or resolve pages, snooze or disable alerts, run a monitor, archive issues, write post-mortems, refuse what a token may not do.",
  },
];

// Case titles that would give away the expected diagnosis are replaced by a
// summary of the case prompt, which only states the symptom.
const DESCRIPTION_OVERRIDES = {
  "perf-ctl-alert-handoff":
    "A p95 latency alert pasted in from email: what happened, and does it need action?",
  "perf-ctl-metric-p95-trap":
    "Search gets sluggish every afternoon, and ops says the hosts' p95 CPU looks fine",
  "perf-ctl-onset-bisect":
    "The account page has felt slower for a few days: when did it start, and what changed?",
  "perf-db-missing-query-cache":
    "Every storefront endpoint is slower than it should be, even a single-row read",
  "perf-xs-android-timeouts":
    "The Android app crashes at checkout since the sale went live",
  "perf-xs-frontend-timeouts":
    "Product search on the web shop spins, then says it is taking too long",
  "perf-xs-ios-watchdog":
    "iOS users see the app freeze on the splash screen and close",
  "write-archive-ambiguous": "Archive last week's checkout errors",
};

// The shared problems named in the comparison's write-up. Each matcher picks
// the affected runs out of the data, so every count on the page is computed.
const KNOWN_ISSUES = [
  {
    id: "iowait-recipe",
    kind: "skill",
    case: "perf-infra-disk-io",
    title: "The skill's CPU recipe counts iowait as busy CPU",
    detail:
      "On the disk case the database host's idle CPU falls while iowait rises. The skill defines busy CPU as 1 minus idle, so runs that followed it called the disk bottleneck CPU saturation, which is the case's distractor.",
    matches: (r) => r.firstFailure.includes('checklist_item "cpu_saturation"'),
  },
  {
    id: "stampede-logs-query",
    kind: "case",
    case: "perf-cache-stampede",
    title: "The cache stampede case insists on a logs query",
    detail:
      "The case requires a logs.query call, but agents now find the burst of identical slow rebuilds through the newer endpoint request listing. These runs had the right answer and grounded evidence.",
    matches: (r) =>
      r.kind === "trajectory" &&
      r.firstFailure.includes("required call missing: logs.query"),
  },
  {
    id: "memory-gc-label",
    kind: "case",
    case: "perf-infra-memory-gc",
    title: "The memory case rejects a fair label",
    detail:
      "These runs found the memo that never evicts, but filed it under unbounded in-memory work instead of memory pressure and GC pauses. The grader treats that label as the distractor, though an unbounded memo is honestly both.",
    matches: (r) =>
      r.graders.length === 1 &&
      r.graders[0] === "answer" &&
      r.firstFailure.includes("unbounded_in_memory_work_large_allocations"),
  },
  {
    id: "chatty-fanout-grounding",
    kind: "case",
    case: "perf-net-chatty-fanout",
    title: "The fan-out case fails grounding for every model",
    detail:
      "Every model fails this case's grounding check on most setups, mostly because the output it cites was truncated.",
    matches: (r) => r.graders.includes("grounding"),
  },
];

const KIND_BY_FAILURE = {
  pass: "pass",
  "trajectory-only (other)": "trajectory",
  "trajectory-only (repeat)": "trajectory",
  "wrong answer/evidence": "wrong",
  budget_exhausted: "budget",
};

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith("--")) throw new Error(`unexpected argument ${a}`);
    const value = argv[i + 1];
    if (value === undefined || value.startsWith("--"))
      throw new Error(`${a} needs a value`);
    args[a.slice(2)] = value;
    i++;
  }
  return args;
}

function parseCsv(text) {
  const rows = [];
  let row = [];
  let field = "";
  let quoted = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted) {
      if (c === '"' && text[i + 1] === '"') {
        field += '"';
        i++;
      } else if (c === '"') {
        quoted = false;
      } else {
        field += c;
      }
    } else if (c === '"') {
      quoted = true;
    } else if (c === ",") {
      row.push(field);
      field = "";
    } else if (c === "\n" || c === "\r") {
      if (c === "\r" && text[i + 1] === "\n") i++;
      row.push(field);
      rows.push(row);
      row = [];
      field = "";
    } else {
      field += c;
    }
  }
  if (field !== "" || row.length) {
    row.push(field);
    rows.push(row);
  }
  const [header, ...body] = rows.filter((r) => r.length > 1 || r[0] !== "");
  return body.map((r) => {
    if (r.length !== header.length)
      throw new Error(`runs.csv row has ${r.length} fields, want ${header.length}`);
    return Object.fromEntries(header.map((h, i) => [h, r[i]]));
  });
}

function parseModelsYaml(text) {
  const models = {};
  let section = null;
  let current = null;
  for (const line of text.split("\n")) {
    if (/^\S/.test(line)) {
      section = line.replace(/:.*$/, "").trim();
      current = section === "judge" ? (models.judge = {}) : null;
      continue;
    }
    const block = line.match(/^ {2}([\w-]+):\s*$/);
    if (section === "agents" && block) {
      current = models[block[1]] = {};
      continue;
    }
    const indent = section === "judge" ? 2 : 4;
    const kv = line.match(new RegExp(`^ {${indent}}(id|label|reasoning_effort|max_tokens_per_turn):\\s*(.+?)\\s*$`));
    if (current && kv) current[kv[1]] = kv[2];
  }
  return models;
}

function readCase(file) {
  const text = fs.readFileSync(file, "utf8");
  const field = (name) => {
    const m = text.match(new RegExp(`^${name}:\\s*(.+?)\\s*$`, "m"));
    if (!m) throw new Error(`${file}: no ${name}`);
    return m[1].replace(/^(["'])(.*)\1$/, "$2");
  };
  return { id: field("id"), suite: field("suite"), title: field("title") };
}

const pct = (n, d) => Math.round((100 * n) / d);
const round = (x, digits) => Number(x.toFixed(digits));
const sum = (xs) => xs.reduce((a, b) => a + b, 0);
const mean = (xs) => (xs.length ? sum(xs) / xs.length : null);

function summarize(runs) {
  const input = sum(runs.map((r) => r.inputTokens));
  const judged = runs.filter((r) => r.judge !== null);
  return {
    runs: runs.length,
    pass: runs.filter((r) => r.status === "pass").length,
    rightAnswerEvidence: runs.filter((r) => r.kind === "pass" || r.kind === "trajectory").length,
    costUsd: round(sum(runs.map((r) => r.costUsd)), 4),
    judgeCostUsd: round(sum(runs.map((r) => r.judgeCostUsd)), 4),
    costPerRunUsd: round(sum(runs.map((r) => r.costUsd)) / runs.length, 6),
    avgToolCalls: round(mean(runs.map((r) => r.toolCalls)), 4),
    avgFailedCalls: round(mean(runs.map((r) => r.failedCalls)), 4),
    avgTurns: round(mean(runs.map((r) => r.turns)), 4),
    avgInputTokens: Math.round(input / runs.length),
    avgOutputTokens: Math.round(mean(runs.map((r) => r.outputTokens))),
    cacheHitRate: round(sum(runs.map((r) => r.cachedTokens)) / input, 6),
    avgWallS: round(mean(runs.map((r) => r.wallS)), 4),
    avgJudge: judged.length ? round(mean(judged.map((r) => r.judge)), 6) : null,
    judged: judged.length,
  };
}

function tableRows(markdown, heading) {
  const start = markdown.indexOf(heading);
  if (start < 0) throw new Error(`no "${heading}" section`);
  const lines = markdown.slice(start).split("\n");
  const rows = [];
  let seen = false;
  for (const line of lines) {
    if (line.startsWith("|")) {
      seen = true;
      rows.push(line.split("|").slice(1, -1).map((c) => c.trim()));
    } else if (seen) break;
  }
  return rows.slice(2);
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  if (!args.source) throw new Error("--source <comparison dir> is required");
  const source = path.resolve(args.source);
  const casesDir = path.resolve(WEBSITE, args.cases ?? "../cli/test/evals/cases");
  const modelsFile = path.resolve(WEBSITE, args.models ?? "../cli/test/evals/models.yaml");

  const csvRuns = parseCsv(fs.readFileSync(path.join(source, "runs.csv"), "utf8"));
  const results = JSON.parse(fs.readFileSync(path.join(source, "results.json"), "utf8"));
  const summaryMd = fs.readFileSync(path.join(source, "summary.md"), "utf8");
  const statsMd = fs.readFileSync(path.join(source, "stats.md"), "utf8");
  const modelsYaml = parseModelsYaml(fs.readFileSync(modelsFile, "utf8"));

  const runs = csvRuns.map((r) => {
    const kind = KIND_BY_FAILURE[r.failure_kind];
    if (!kind) throw new Error(`unknown failure_kind ${r.failure_kind}`);
    if ((r.status === "pass") !== (kind === "pass"))
      throw new Error(`${r.case} ${r.model} ${r.arm}: status ${r.status} with kind ${r.failure_kind}`);
    return {
      case: r.case,
      suite: r.suite,
      arm: r.arm,
      model: r.model,
      status: r.status,
      kind,
      graders: r.failed_graders ? r.failed_graders.split("+") : [],
      firstFailure: r.first_failure,
      toolCalls: Number(r.tool_calls),
      failedCalls: Number(r.failed_calls),
      turns: Number(r.turns),
      inputTokens: Number(r.input_tokens),
      cachedTokens: Number(r.cached_tokens),
      outputTokens: Number(r.output_tokens),
      costUsd: Number(r.cost_usd),
      judgeCostUsd: Number(r.judge_cost_usd || 0),
      judge: r.judge_score === "" ? null : Number(r.judge_score),
      wallS: Number(r.wall_s),
    };
  });

  const modelKeys = [...new Set(runs.map((r) => r.model))];
  if (modelKeys.length !== MODEL_ORDER.length || !MODEL_ORDER.every((m) => modelKeys.includes(m)))
    throw new Error(`models in runs.csv: ${modelKeys.join(", ")}`);

  const caseFiles = SUITES.flatMap((s) =>
    fs
      .readdirSync(path.join(casesDir, s.id))
      .filter((f) => f.endsWith(".yaml"))
      .map((f) => readCase(path.join(casesDir, s.id, f))),
  );
  const caseById = new Map(caseFiles.map((c) => [c.id, c]));

  const header = results.header;
  const runDate = header.started.slice(0, 10);
  const commit = header.gitSha.slice(0, 8);

  const providersByModel = {};
  const modelIdByKey = {};
  for (const r of results.runs) {
    modelIdByKey[r.model] ??= r.modelId;
    if (modelIdByKey[r.model] !== r.modelId)
      throw new Error(`${r.model} ran as both ${modelIdByKey[r.model]} and ${r.modelId}`);
    const counts = (providersByModel[r.model] ??= {});
    for (const p of new Set(r.provider ?? [])) counts[p] = (counts[p] ?? 0) + 1;
  }
  if (results.runs.length !== runs.length)
    throw new Error(`results.json has ${results.runs.length} runs, runs.csv ${runs.length}`);

  const models = MODEL_ORDER.map((key) => {
    const yaml = modelsYaml[key];
    if (!yaml) throw new Error(`models.yaml has no ${key}`);
    if (yaml.id !== modelIdByKey[key])
      throw new Error(`${key}: models.yaml id ${yaml.id}, results.json ${modelIdByKey[key]}`);
    const own = runs.filter((r) => r.model === key);
    const failingGraders = {};
    for (const r of own) for (const g of r.graders) failingGraders[g] = (failingGraders[g] ?? 0) + 1;
    const providers = Object.entries(providersByModel[key] ?? {})
      .sort((a, b) => b[1] - a[1])
      .map(([name, runCount]) => ({ name, runs: runCount }));
    return {
      key,
      label: yaml.label,
      short: MODEL_SHORT[key],
      vendor: MODEL_VENDOR[key],
      weights: MODEL_WEIGHTS[key],
      id: yaml.id,
      provider: providers[0]?.name ?? "",
      reasoningEffort: yaml.reasoning_effort,
      maxTokensPerTurn: Number(yaml.max_tokens_per_turn),
      providers,
      totals: {
        ...summarize(own),
        notGraded: own.filter((r) => r.status !== "pass" && r.status !== "fail").length,
        failingGraders,
      },
    };
  });

  const armsBySuite = Object.fromEntries(
    SUITES.map((s) => [
      s.id,
      Object.keys(SETUPS).filter((arm) => runs.some((r) => r.suite === s.id && r.arm === arm)),
    ]),
  );

  const suites = SUITES.map((s) => {
    const ids = [...new Set(runs.filter((r) => r.suite === s.id).map((r) => r.case))];
    return { ...s, cases: ids.length, setups: armsBySuite[s.id] };
  });

  const setups = suites.flatMap((s) =>
    s.setups.map((arm) => ({
      suite: s.id,
      arm,
      ...SETUPS[arm],
      results: Object.fromEntries(
        MODEL_ORDER.map((m) => [
          m,
          summarize(runs.filter((r) => r.suite === s.id && r.arm === arm && r.model === m)),
        ]),
      ),
    })),
  );
  const setup = (suite, arm, m) =>
    setups.find((x) => x.suite === suite && x.arm === arm).results[m];
  const rateOf = (x) => (100 * x.pass) / x.runs;

  const comparisons = {
    skillUplift: MODEL_ORDER.map((m) => {
      const a = setup("perf", "cli+skill", m);
      const b = setup("perf", "cli-noskill", m);
      return { suite: "perf", model: m, with: pct(a.pass, a.runs), without: pct(b.pass, b.runs), pp: Math.round(rateOf(a) - rateOf(b)) };
    }),
    cliVsMcp: [
      ["perf", "cli+skill", "mcp"],
      ["write", "cli-write+skill", "mcp-write"],
    ].flatMap(([suite, cli, mcp]) =>
      MODEL_ORDER.map((m) => {
        const a = setup(suite, cli, m);
        const b = setup(suite, mcp, m);
        return { suite, cliArm: cli, mcpArm: mcp, model: m, cli: pct(a.pass, a.runs), mcp: pct(b.pass, b.runs), pp: Math.round(rateOf(a) - rateOf(b)) };
      }),
    ),
  };

  const groupOf = (caseId) => GROUPS.find((g) => caseId === g.id || caseId.startsWith(`${g.id}-`)).id;
  const groups = GROUPS.map((g) => {
    const own = runs.filter((r) => groupOf(r.case) === g.id);
    return {
      ...g,
      cases: new Set(own.map((r) => r.case)).size,
      results: Object.fromEntries(
        MODEL_ORDER.map((m) => {
          const mine = own.filter((r) => r.model === m);
          return [m, { runs: mine.length, pass: mine.filter((r) => r.status === "pass").length }];
        }),
      ),
    };
  });

  const issueOf = new Map();
  const knownIssues = KNOWN_ISSUES.map((issue) => {
    const caseRuns = runs.filter((r) => r.case === issue.case);
    const hit = caseRuns.filter((r) => r.status !== "pass" && issue.matches(r));
    if (!hit.length) throw new Error(`known issue ${issue.id} matches no run`);
    for (const r of hit) issueOf.set(r, issue.id);
    return {
      id: issue.id,
      kind: issue.kind,
      case: issue.case,
      title: issue.title,
      detail: issue.detail,
      caseRuns: caseRuns.length,
      caseFailures: caseRuns.filter((r) => r.status !== "pass").length,
      affectedRuns: hit.length,
      truncated: hit.filter((r) => r.firstFailure.includes("truncated")).length,
      byModel: Object.fromEntries(MODEL_ORDER.map((m) => [m, hit.filter((r) => r.model === m).length])),
    };
  });

  const caseIds = [...new Set(runs.map((r) => r.case))].sort();
  const cases = caseIds.map((id) => {
    const meta = caseById.get(id);
    if (!meta) throw new Error(`no case file for ${id}`);
    const own = runs.filter((r) => r.case === id);
    return {
      id,
      suite: meta.suite,
      group: groupOf(id),
      title: DESCRIPTION_OVERRIDES[id] ?? meta.title,
      runs: Object.fromEntries(
        MODEL_ORDER.map((m) => [
          m,
          Object.fromEntries(
            own
              .filter((r) => r.model === m)
              .map((r) => [
                r.arm,
                {
                  status: r.status,
                  kind: r.kind,
                  graders: r.graders,
                  firstFailure: r.firstFailure,
                  toolCalls: r.toolCalls,
                  costUsd: round(r.costUsd, 5),
                  judge: r.judge === null ? null : round(r.judge, 2),
                  wallS: r.wallS,
                  knownIssue: issueOf.get(r) ?? null,
                },
              ]),
          ),
        ]),
      ),
    };
  });
  for (const id of Object.keys(DESCRIPTION_OVERRIDES))
    if (!caseIds.includes(id)) throw new Error(`description override for unknown case ${id}`);

  const judge = modelsYaml.judge;
  const data = {
    run: {
      title: `Traceway agent benchmarks: ${runDate}`,
      date: runDate,
      commit,
      cliVersion: header.cliVersion,
      backendVersion: header.backendVersion,
      runsPerCase: Math.max(...results.runs.map((r) => r.runIdx)),
      totalRuns: runs.length,
      runsPerModel: runs.length / MODEL_ORDER.length,
      agentCostUsd: round(sum(runs.map((r) => r.costUsd)), 2),
      judgeCostUsd: round(sum(runs.map((r) => r.judgeCostUsd)), 2),
      judge: { id: judge.id, label: judge.label.replace(/\s*\(judge\)$/, "") },
      harnessUrl: HARNESS_URL,
      methodologyUrl: METHODOLOGY_URL,
      statuses: Object.fromEntries(
        [...new Set(runs.map((r) => r.status))].map((s) => [s, runs.filter((r) => r.status === s).length]),
      ),
    },
    models,
    suites,
    setups,
    comparisons,
    groups,
    knownIssues,
    cases,
  };

  crossCheck(data, summaryMd, statsMd);

  const out = path.resolve(WEBSITE, args.out ?? `content/benchmarks/${runDate}.md`);
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, renderMarkdown(data));
  console.log(`wrote ${path.relative(WEBSITE, out)}: ${data.run.totalRuns} runs, ${cases.length} cases`);

  parseBack(data, fs.readFileSync(out, "utf8"), summaryMd);
}

function crossCheck(data, summaryMd, statsMd) {
  const errors = [];
  const expect = (what, got, want) => {
    if (String(got) !== String(want)) errors.push(`${what}: generated ${got}, report says ${want}`);
  };

  const runsLine = summaryMd.match(/\*\*Runs:\*\* (\d+) \(fail (\d+), pass (\d+)\)/);
  if (!runsLine) errors.push("summary.md: no Runs line");
  else {
    expect("total runs", data.run.totalRuns, runsLine[1]);
    expect("failing runs", data.run.statuses.fail ?? 0, runsLine[2]);
    expect("passing runs", data.run.statuses.pass ?? 0, runsLine[3]);
  }

  const spend = summaryMd.match(/\*\*Spend:\*\* \$([\d.]+) \(judge \$([\d.]+)\)/);
  if (!spend) errors.push("summary.md: no Spend line");
  else {
    expect("judge spend", data.run.judgeCostUsd.toFixed(2), spend[2]);
    expect("total spend", (data.run.agentCostUsd + data.run.judgeCostUsd).toFixed(2), spend[1]);
  }

  const setupOf = (suite, model, arm) =>
    data.setups.find((s) => s.suite === suite && s.arm === arm)?.results[model];

  for (const [suite, model, arm, frac, rate] of tableRows(summaryMd, "## Pass rate by suite")) {
    const s = setupOf(suite, model, arm);
    if (!s) {
      errors.push(`summary.md row ${suite} ${model} ${arm} not generated`);
      continue;
    }
    expect(`${suite} ${model} ${arm} passes`, `${s.pass}/${s.runs}`, frac);
    expect(`${suite} ${model} ${arm} rate`, `${pct(s.pass, s.runs)}%`, rate);
  }

  for (const [, model, w, wo, uplift] of tableRows(summaryMd, "## Skill uplift")) {
    const u = data.comparisons.skillUplift.find((x) => x.model === model);
    expect(`${model} skill uplift`, `${u.with}% ${u.without}% ${u.pp >= 0 ? "+" : ""}${u.pp}pp`, `${w} ${wo} ${uplift}`);
  }

  for (const [suite, model, , , cli, mcp, delta] of tableRows(summaryMd, "## CLI vs MCP")) {
    const c = data.comparisons.cliVsMcp.find((x) => x.suite === suite && x.model === model);
    expect(`${suite} ${model} CLI vs MCP`, `${c.cli}% ${c.mcp}% ${c.pp >= 0 ? "+" : ""}${c.pp}pp`, `${cli} ${mcp} ${delta}`);
  }

  for (const row of tableRows(summaryMd, "## Cost (averages over passing runs)")) {
    const m = data.models.find((x) => x.key === row[0]);
    if (!m) continue;
    expect(`${m.key} passing runs`, m.totals.pass, row[1]);
    expect(`${m.key} cache hit`, `${Math.round(100 * m.totals.cacheHitRate)}%`, row[6]);
    expect(`${m.key} agent spend`, `$${m.totals.costUsd.toFixed(2)}`, row[7]);
  }

  for (const row of tableRows(statsMd, "| Model | Suite | Setup | Pass |")) {
    const [model, suite, arm, , rae, notGraded, cost, calls, , cache, judge] = row;
    const s = setupOf(suite, model, arm);
    if (!s) {
      errors.push(`stats.md row ${suite} ${model} ${arm} not generated`);
      continue;
    }
    expect(`${suite} ${model} ${arm} right answer and evidence`, `${s.rightAnswerEvidence}/${s.runs} (${pct(s.rightAnswerEvidence, s.runs)}%)`, rae);
    expect(`${suite} ${model} ${arm} not graded`, 0, notGraded);
    expect(`${suite} ${model} ${arm} cost`, `$${s.costUsd.toFixed(2)}`, cost);
    expect(`${suite} ${model} ${arm} avg calls`, s.avgToolCalls.toFixed(1), calls);
    expect(`${suite} ${model} ${arm} cache hit`, `${Math.round(100 * s.cacheHitRate)}%`, cache);
    expect(`${suite} ${model} ${arm} judge`, s.avgJudge.toFixed(2), judge);
  }

  if (errors.length) {
    console.error(`cross-check against summary.md and stats.md failed:\n  ${errors.join("\n  ")}`);
    process.exit(1);
  }
  console.log("cross-check against summary.md and stats.md: ok");
}

const usd = (x, digits) => `$${x.toFixed(digits)}`;
const frac = (pass, runs) => `${pass}/${runs} (${pct(pass, runs)}%)`;
const signedPp = (pp) => `${pp >= 0 ? "+" : "-"}${Math.abs(pp)}pp`;
const words = ["zero", "one", "two", "three", "four", "five"];

function yamlScalar(v) {
  return typeof v === "number" ? String(v) : JSON.stringify(v);
}

function yamlBlock(value, indent = "") {
  const lines = [];
  for (const [k, v] of Object.entries(value)) {
    if (Array.isArray(v)) {
      lines.push(`${indent}${k}:`);
      for (const item of v) {
        const [first, ...rest] = yamlBlock(item, `${indent}    `).split("\n");
        lines.push(`${indent}  - ${first.trimStart()}`, ...rest);
      }
    } else if (v && typeof v === "object") {
      lines.push(`${indent}${k}:`, yamlBlock(v, `${indent}  `));
    } else {
      lines.push(`${indent}${k}: ${yamlScalar(v)}`);
    }
  }
  return lines.join("\n");
}

function mdTable(headers, rows, align) {
  const line = (cells) => `| ${cells.map((c) => escapeCell(String(c))).join(" | ")} |`;
  const sep = `|${headers.map((_, i) => (align?.[i] === "r" ? "---:" : "---")).join("|")}|`;
  return [line(headers), sep, ...rows.map(line)].join("\n");
}

function renderMarkdown(data) {
  const { run, models, suites, setups, comparisons, groups, knownIssues, cases } = data;
  const labels = models.map((m) => m.label);
  const right = (n) => Array(n).fill("r");
  const setupRow = (suite, arm) => setups.find((x) => x.suite === suite && x.arm === arm);
  const front = {
    title: run.title,
    date: run.date,
    commit: run.commit,
    cliVersion: run.cliVersion,
    backendVersion: run.backendVersion,
    harnessUrl: run.harnessUrl,
    methodologyUrl: run.methodologyUrl,
    runsPerCase: run.runsPerCase,
    judge: run.judge,
    suites: suites.map((x) => ({ id: x.id, label: x.label })),
    models: models.map((m) => ({
      key: m.key,
      label: m.label,
      short: m.short,
      vendor: m.vendor,
      weights: m.weights,
      id: m.id,
      provider: m.provider,
      providers: m.providers,
      reasoningEffort: m.reasoningEffort,
      maxTokensPerTurn: m.maxTokensPerTurn,
    })),
  };

  const comparisonRows = [
    ...comparisons.skillUplift.length
      ? [["Skill uplift", "perf", "cli+skill", "cli-noskill", ...models.map((m) => {
          const u = comparisons.skillUplift.find((x) => x.model === m.key);
          return `${signedPp(u.pp)} (${u.with}% vs ${u.without}%)`;
        })]]
      : [],
    ...[...new Set(comparisons.cliVsMcp.map((c) => c.suite))].map((suite) => {
      const first = comparisons.cliVsMcp.find((c) => c.suite === suite);
      return ["CLI vs MCP", suite, first.cliArm, first.mcpArm, ...models.map((m) => {
        const c = comparisons.cliVsMcp.find((x) => x.suite === suite && x.model === m.key);
        return `${signedPp(c.pp)} (${c.cli}% vs ${c.mcp}%)`;
      })];
    }),
  ];

  const metricRows = [
    [COST_METRICS.spend, ...models.map((m) => usd(m.totals.costUsd, 2))],
    [COST_METRICS.perRun, ...models.map((m) => usd(m.totals.costPerRunUsd, 5))],
    [COST_METRICS.perPass, ...models.map((m) => usd(m.totals.costUsd / m.totals.pass, 5))],
    [COST_METRICS.toolCalls, ...models.map((m) => m.totals.avgToolCalls.toFixed(1))],
    [COST_METRICS.wall, ...models.map((m) => `${m.totals.avgWallS.toFixed(1)} s`)],
    [COST_METRICS.cacheHit, ...models.map((m) => `${Math.round(100 * m.totals.cacheHitRate)}%`)],
    [COST_METRICS.judge, ...models.map((m) => (m.totals.avgJudge === null ? "" : m.totals.avgJudge.toFixed(2)))],
    [COST_METRICS.judged, ...models.map((m) => m.totals.judged)],
    [COST_METRICS.judgeSpend, ...models.map((m) => usd(m.totals.judgeCostUsd, 4))],
  ];

  const armsOf = (suite) => suites.find((x) => x.id === suite).setups;
  const matrixRows = cases.flatMap((c) =>
    armsOf(c.suite).map((arm) => [
      c.id,
      c.group,
      c.title,
      c.suite,
      arm,
      ...models.map((m) => {
        const r = c.runs[m.key][arm];
        if (!r) return "";
        if (r.kind === "pass") return "pass";
        if (!r.firstFailure) throw new Error(`${c.id} ${arm} ${m.key}: failed run without a first failure`);
        return `fail (${MATRIX_KIND[r.kind]}): ${r.firstFailure}`;
      }),
    ]),
  );
  const detailRows = cases.flatMap((c) =>
    armsOf(c.suite).flatMap((arm) =>
      models
        .filter((m) => c.runs[m.key][arm])
        .map((m) => {
          const r = c.runs[m.key][arm];
          return [
            c.id,
            arm,
            m.label,
            r.graders.join("+"),
            r.toolCalls,
            usd(r.costUsd, 5),
            r.wallS.toFixed(1),
            r.judge === null ? "" : r.judge.toFixed(2),
            r.knownIssue ?? "",
          ];
        }),
    ),
  );

  const modelList = `${labels.slice(0, -1).join(", ")} and ${labels[labels.length - 1]}`;
  return `---
${yamlBlock(front)}
---

# ${run.title}

${words[models.length][0].toUpperCase()}${words[models.length].slice(1)} models, ${modelList}, each ran the same ${run.runsPerModel} eval runs on ${run.date}, at commit \`${run.commit}\`. Every run gets a fresh Traceway backend seeded with the telemetry of a system broken in one specific way, and deterministic code grades the result. [How we measure](${run.methodologyUrl}) · [the harness and the cases](${run.harnessUrl}).

This file is generated by \`website/scripts/build-benchmarks-data.mjs\` from the eval results and is the data behind ${PAGE_URL}. Change the generator, not this file.

## ${SECTIONS.summary}

A run passes when its answer is right, its evidence is grounded and its writes are safe. "Right answer and evidence" also counts runs that failed only the trajectory check.

${mdTable(
  ["Model", "Runs", "Passed", "Pass rate", "Right answer and evidence", "Not graded", "Agent cost", "Cost per run"],
  models.map((m) => [m.label, m.totals.runs, m.totals.pass, `${pct(m.totals.pass, m.totals.runs)}%`, m.totals.rightAnswerEvidence, m.totals.notGraded, usd(m.totals.costUsd, 2), usd(m.totals.costPerRunUsd, 5)]),
  ["l", ...right(7)],
)}

## ${SECTIONS.passRate}

${mdTable(
  ["Suite", "Setup", "Label", "Description", ...labels],
  [
    ...setups.map((x) => [x.suite, x.arm, x.label, x.description, ...models.map((m) => frac(x.results[m.key].pass, x.results[m.key].runs))]),
    ["all", "", "All runs", "", ...models.map((m) => frac(m.totals.pass, m.totals.runs))],
  ],
)}

### ${SECTIONS.rightAnswer}

${mdTable(
  ["Suite", "Setup", ...labels],
  setups.map((x) => [x.suite, x.arm, ...models.map((m) => `${x.results[m.key].rightAnswerEvidence}/${x.results[m.key].runs}`)]),
)}

## ${SECTIONS.comparisons}

Setup A minus setup B, in percentage points, computed before rounding.

${mdTable(["Comparison", "Suite", "Setup A", "Setup B", ...labels], comparisonRows)}

## ${SECTIONS.groups}

Passes over every setup a case ran on.

${mdTable(
  ["Group", "Label", "Description", "Cases", ...labels],
  groups.map((g) => [g.id, g.label, g.description, g.cases, ...models.map((m) => frac(g.results[m.key].pass, g.results[m.key].runs))]),
)}

## ${SECTIONS.cost}

Every run per model. Agent cost is the model spend the harness recorded for each run, through OpenRouter; the judge's spend is separate. Runs went three at a time on one machine.

${mdTable(["Metric", ...labels], metricRows, ["l", ...right(labels.length)])}

### ${SECTIONS.costBySetup}

${mdTable(
  ["Suite", "Setup", "Model", "Agent cost", "Cost per run", "Avg tool calls", "Cache hit", "Judge"],
  setups.flatMap((x) =>
    models.map((m) => {
      const r = setupRow(x.suite, x.arm).results[m.key];
      return [x.suite, x.arm, m.label, usd(r.costUsd, 2), usd(r.costPerRunUsd, 5), r.avgToolCalls.toFixed(1), `${Math.round(100 * r.cacheHitRate)}%`, r.avgJudge === null ? "" : r.avgJudge.toFixed(2)];
    }),
  ),
  ["l", "l", "l", ...right(5)],
)}

## ${SECTIONS.knownIssues}

Problems in our own cases and skill, not in the models. The runs they hit are still counted as failures. The model columns count the runs each issue hit.

${mdTable(
  ["Issue", "Kind", "Case", "Title", "Detail", ...labels, "Runs of the case", "Truncated output"],
  knownIssues.map((i) => [i.id, i.kind, i.case, i.title, i.detail, ...models.map((m) => i.byModel[m.key]), i.caseRuns, i.truncated]),
)}

## ${SECTIONS.matrix}

One row per case and setup. A cell is \`pass\`, or \`fail (<kind>): <first failure reason>\`, where the kind is \`${MATRIX_KIND.wrong}\` (the answer or its evidence failed), \`${MATRIX_KIND.trajectory}\` (the answer and its evidence were right, but a required call was missing or out of order) or \`${MATRIX_KIND.budget}\`. Descriptions summarize the case without giving the answer away.

${mdTable(["Case", "Group", "Description", "Suite", "Setup", ...labels], matrixRows)}

### ${SECTIONS.runDetails}

Every run's failed graders, tool calls, agent cost, wall time, judge score (0-1, never gating) and the known issue it hit, if any.

${mdTable(
  ["Case", "Setup", "Model", "Failed graders", "Tool calls", "Agent cost", "Wall time (s)", "Judge", "Known issue"],
  detailRows,
  ["l", "l", "l", "l", "r", "r", "r", "r", "l"],
)}
`;
}

function parseBack(data, markdown, summaryMd) {
  const parsed = parseBenchmark(markdown);
  const errors = [];
  const expect = (what, got, want) => {
    if (String(got) !== String(want)) errors.push(`${what}: parsed ${got}, generated ${want}`);
  };

  expect("total runs", parsed.run.totalRuns, data.run.totalRuns);
  expect("commit", parsed.run.commit, data.run.commit);
  expect("all graded", parsed.run.allGraded, (data.run.statuses.pass ?? 0) + (data.run.statuses.fail ?? 0) === data.run.totalRuns);
  for (const m of data.models) {
    const p = parsed.models.find((x) => x.key === m.key);
    const t = m.totals;
    expect(`${m.key} passes`, `${p.totals.pass}/${p.totals.runs}`, `${t.pass}/${t.runs}`);
    expect(`${m.key} right answer and evidence`, p.totals.rightAnswerEvidence, t.rightAnswerEvidence);
    expect(`${m.key} agent cost`, p.totals.costUsd.toFixed(2), t.costUsd.toFixed(2));
    expect(`${m.key} cost per run`, p.totals.costPerRunUsd.toFixed(5), t.costPerRunUsd.toFixed(5));
    expect(`${m.key} judge spend`, p.totals.judgeCostUsd.toFixed(4), t.judgeCostUsd.toFixed(4));
    expect(`${m.key} avg calls`, p.totals.avgToolCalls.toFixed(1), t.avgToolCalls.toFixed(1));
    expect(`${m.key} wall`, p.totals.avgWallS.toFixed(1), t.avgWallS.toFixed(1));
    expect(`${m.key} cache hit`, Math.round(100 * p.totals.cacheHitRate), Math.round(100 * t.cacheHitRate));
    expect(`${m.key} judge`, p.totals.avgJudge?.toFixed(2), t.avgJudge?.toFixed(2));
    expect(`${m.key} judged`, p.totals.judged, t.judged);
    expect(`${m.key} providers`, JSON.stringify(p.providers), JSON.stringify(m.providers));
  }
  for (const x of data.setups)
    for (const m of data.models) {
      const p = parsed.setups.find((y) => y.suite === x.suite && y.arm === x.arm)?.results[m.key];
      const r = x.results[m.key];
      expect(`${x.suite} ${x.arm} ${m.key}`, p && `${p.pass}/${p.runs} ${p.rightAnswerEvidence} ${p.costPerRunUsd.toFixed(5)}`, `${r.pass}/${r.runs} ${r.rightAnswerEvidence} ${r.costPerRunUsd.toFixed(5)}`);
    }
  expect("suites", JSON.stringify(parsed.suites.map((x) => [x.id, x.cases, x.setups])), JSON.stringify(data.suites.map((x) => [x.id, x.cases, x.setups])));
  expect("comparisons", JSON.stringify(parsed.comparisons), JSON.stringify(data.comparisons));
  for (const g of data.groups) {
    const p = parsed.groups.find((y) => y.id === g.id);
    const fold = (x) => [x.cases, ...data.models.map((m) => `${x.results[m.key].pass}/${x.results[m.key].runs}`)].join(" ");
    expect(`group ${g.id}`, p && fold(p), fold(g));
  }
  for (const i of data.knownIssues) {
    const p = parsed.knownIssues.find((y) => y.id === i.id);
    expect(`issue ${i.id}`, p && JSON.stringify([p.byModel, p.affectedRuns, p.caseRuns, p.truncated]), JSON.stringify([i.byModel, i.affectedRuns, i.caseRuns, i.truncated]));
  }
  expect("cases", parsed.cases.map((c) => c.id).join(), data.cases.map((c) => c.id).join());
  let cells = 0;
  for (const c of data.cases) {
    const p = parsed.cases.find((y) => y.id === c.id);
    expect(`${c.id} meta`, p && `${p.suite} ${p.group} ${p.title}`, `${c.suite} ${c.group} ${c.title}`);
    for (const m of data.models)
      for (const [arm, r] of Object.entries(c.runs[m.key])) {
        const q = p?.runs[m.key][arm];
        cells++;
        const flat = (x) => x && JSON.stringify([x.status, x.kind, x.graders, x.firstFailure, x.toolCalls, x.costUsd.toFixed(5), x.judge?.toFixed(2) ?? null, x.wallS, x.knownIssue]);
        expect(`${c.id} ${arm} ${m.key}`, flat(q), flat(r));
      }
  }

  for (const [suite, model, arm, frac] of tableRows(summaryMd, "## Pass rate by suite")) {
    const p = parsed.setups.find((x) => x.suite === suite && x.arm === arm)?.results[model];
    expect(`summary.md ${suite} ${model} ${arm}`, p && `${p.pass}/${p.runs}`, frac);
  }
  for (const row of tableRows(summaryMd, "## Cost (averages over passing runs)")) {
    const p = parsed.models.find((x) => x.key === row[0]);
    if (!p) continue;
    expect(`summary.md ${row[0]} passing runs`, p.totals.pass, row[1]);
    expect(`summary.md ${row[0]} agent spend`, `$${p.totals.costUsd.toFixed(2)}`, row[7]);
  }

  if (errors.length) {
    console.error(`parse-back check failed:\n  ${errors.join("\n  ")}`);
    process.exit(1);
  }
  console.log(`parse-back check: ok (${cells} runs compared, totals match summary.md)`);
  for (const m of parsed.models) {
    const t = m.totals;
    console.log(
      `  ${m.label}: ${t.pass}/${t.runs} (${pct(t.pass, t.runs)}%), right answer and evidence ${t.rightAnswerEvidence}, agent $${t.costUsd.toFixed(2)}`,
    );
  }
}

main();
