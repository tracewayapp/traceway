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

// Display order. A model's palette slot (lib/benchmark-format.ts) is fixed
// to its key, so adding a model never recolors the others.
const MODEL_ORDER = ["haiku", "deepseek-flash", "opus", "luna", "minimax-flash", "qwen-27b", "glm"];
const MODEL_SHORT = {
  haiku: "Haiku",
  "deepseek-flash": "DeepSeek",
  opus: "Opus",
  luna: "Luna",
  "minimax-flash": "MiniMax",
  "qwen-27b": "Qwen 27B",
  glm: "GLM",
};
const MODEL_VENDOR = {
  haiku: "Anthropic",
  "deepseek-flash": "DeepSeek",
  opus: "Anthropic",
  luna: "OpenAI",
  "minimax-flash": "MiniMax",
  "qwen-27b": "Alibaba",
  glm: "Z.ai",
};
const MODEL_WEIGHTS = {
  haiku: "proprietary",
  "deepseek-flash": "open",
  opus: "proprietary",
  luna: "proprietary",
  "minimax-flash": "open",
  "qwen-27b": "open",
  glm: "open",
};

// Every suite the harness has, in display order. Only the suites with runs in
// the source directory are rendered.
const SUITES = [
  {
    id: "perf",
    label: "Performance",
    description: "Diagnose a latency problem from production telemetry.",
  },
  {
    id: "debug",
    label: "Debug",
    description:
      "Find the root cause of an issue and fix the code. Hidden tests check the fix.",
  },
  {
    id: "write",
    label: "Write",
    description:
      "Act on the system safely: pages, alerts, monitors, archiving and post-mortems.",
  },
  {
    id: "setup",
    label: "Setup",
    description: "Instrument an app with the setup skill until its telemetry arrives.",
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
  "cli+setup-skill": {
    label: "CLI + setup skill",
    description: "The setup skill and a container shell with the app's toolchain.",
  },
};

// The setup pairs the harness compares (internal/report/aggregate.go), kept
// here so the page's deltas match summary.md row for row.
const SKILL_PAIR = ["cli+skill", "cli-noskill"];
const CLI_MCP_PAIRS = [
  ["cli+skill", "mcp"],
  ["cli-write+skill", "mcp-write"],
];

// A group is matched by its case id prefix, or by an explicit case list where
// the ids do not share one (the debug suite). `code` is what the page prints
// next to the label.
const GROUPS = [
  {
    id: "perf-app",
    code: "perf-app-*",
    label: "App code",
    description:
      "Slow code in the request path: serialization, quadratic loops, large allocations, debug logging, work that belongs in the background.",
  },
  {
    id: "perf-cache",
    code: "perf-cache-*",
    label: "Caching",
    description: "Cold starts after a release, a missing cache, a cache stampede.",
  },
  {
    id: "perf-ctl",
    code: "perf-ctl-*",
    label: "Controls and traps",
    description:
      "A healthy endpoint, endpoints marked slow on purpose, a misleading metric, a pasted alert and an onset to pin down. Some have nothing to fix.",
  },
  {
    id: "perf-db",
    code: "perf-db-*",
    label: "Database",
    description:
      "N+1 queries, a missing index, pool exhaustion, lock contention, over-fetching, unbounded results.",
  },
  {
    id: "perf-infra",
    code: "perf-infra-*",
    label: "Infrastructure",
    description: "CPU saturation, disk I/O, memory pressure, a noisy neighbor.",
  },
  {
    id: "perf-net",
    code: "perf-net-*",
    label: "Network and upstreams",
    description:
      "Retries, chatty fan-out, sequential calls, large payloads, connection reuse, a slow third party.",
  },
  {
    id: "perf-xs",
    code: "perf-xs-*",
    label: "Cross-stack and mobile",
    description:
      "A browser or mobile app fails, and the agent has to follow it into the backend.",
  },
  {
    id: "debug-backend",
    code: "debug, Go / Python / Node",
    label: "Backend fixes",
    description:
      "A panic, a lowered timeout, a race, a renamed field, a config error at startup, two bugs under one issue hash. The patch has to pass hidden tests.",
    cases: [
      "debug-go-nil-regression",
      "debug-go-task-panic",
      "debug-dashboard-url",
      "debug-timeout-downstream",
      "debug-mixed-group",
      "debug-py-keyerror-webhook",
      "debug-log-only-error",
      "debug-race-duplicate",
      "debug-js-contract-rename",
    ],
  },
  {
    id: "debug-mobile",
    code: "debug, Android / iOS / Flutter",
    label: "Mobile crashes",
    description:
      "A Kotlin, Swift or Dart crash: find the culprit from the stack trace and patch it. The patch is checked statically.",
    cases: ["debug-android-npe", "debug-ios-force-unwrap", "debug-flutter-null-check"],
  },
  {
    id: "debug-ctl",
    code: "debug controls",
    label: "Debug controls and policy",
    description:
      "A rare 500 the telemetry cannot explain, and a cleanup request made with a write credential.",
    cases: ["debug-insufficient-evidence", "debug-no-writes"],
  },
  {
    id: "write",
    code: "write-*",
    label: "Write actions",
    description:
      "Acknowledge or resolve pages, snooze or disable alerts, run a monitor, archive issues, write post-mortems, refuse what a token may not do.",
  },
  {
    id: "setup",
    code: "setup-*",
    label: "Setup",
    description:
      "Instrument a Go, Express, FastAPI, Next.js or React app, or a Go + React monorepo, until its telemetry arrives.",
  },
];

const groupOf = (caseId) => {
  const g = GROUPS.find((x) =>
    x.cases ? x.cases.includes(caseId) : caseId === x.id || caseId.startsWith(`${x.id}-`),
  );
  if (!g) throw new Error(`no group for case ${caseId}`);
  return g.id;
};

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
  "debug-android-npe": "The Android app crashes while users scroll the home feed",
  "debug-flutter-null-check":
    "Some product pages in the Flutter app show a grey screen instead of the product",
  "debug-go-task-panic":
    "A pasted new-error email about a runtime.boundsError in the fulfillment service that has not stopped",
  "debug-insufficient-evidence": "A few report exports fail with a 500 every day",
  "debug-ios-force-unwrap":
    "The iOS app closes when customers open support from the Help screen",
  "debug-js-contract-rename": "The shop's order history page has been blank for a few hours",
  "debug-log-only-error":
    "catalog-api answers 500s and the recommendations row on product pages is empty",
  "debug-mixed-group": "An issue on the shipments API went quiet for days and is back",
  "debug-no-writes": "The billing service has a pile of errors: look into it and clean up",
  "debug-py-keyerror-webhook":
    "Failed invoice.paid webhook deliveries for a day, and some payments missing from the records",
  "debug-timeout-downstream":
    "Many customers cannot get a cart total since yesterday, and a retry usually works",
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
      "Every model fails this case's grounding check on most setups: the case wants the two fanned-out routes, GET /stock/{sku} and GET /prices/{sku}, in the output of a cited call, and the models cite the aggregate view that shows the fan-out without those route names.",
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
    if (a === "--part") (args.part ??= []).push(value);
    else args[a.slice(2)] = value;
    i++;
  }
  return args;
}

// A part is one batch of runs: `--part perf,write:3b00b083:2026-10-21`. A
// merged results.json keeps only its first part's commit and date, so the
// batches are named on the command line and written into the page. A part
// may be narrowed to the setups that were run again after a fix:
// `--part debug@cli+skill:1a2b3c4d:2026-10-22` says the debug suite's
// cli+skill runs come from that later batch, and the suite's other setups
// from the part that names the suite alone. A part may also be narrowed to
// the models it ran, when models were added to the page in a later batch:
// `--part perf,debug,write/glm,luna:5e6f7a8b:2026-10-22`. Every suite needs
// exactly one unnarrowed part per model.
function parseParts(values, suites, header, runs) {
  if (!values) return [{ suites: suites.map((s) => s.id), commit: header.gitSha.slice(0, 8), date: header.started.slice(0, 10) }];
  const parts = values.map((v) => {
    const m = v.match(/^([\w,-]+)(?:@([\w+,-]+))?(?:\/([\w,-]+))?:([0-9a-f]{7,40}):(\d{4}-\d{2}-\d{2})$/);
    if (!m) throw new Error(`--part ${v}: want <suites>[@<arms>][/<models>]:<commit>:<YYYY-MM-DD>`);
    const part = { suites: m[1].split(",") };
    if (m[2]) part.arms = m[2].split(",");
    if (m[3]) part.models = m[3].split(",");
    part.commit = m[4].slice(0, 8);
    part.date = m[5];
    return part;
  });
  const covers = (p, model) => !p.models || p.models.includes(model);
  for (const s of suites)
    for (const model of MODEL_ORDER) {
      const owners = parts.filter((p) => !p.arms && p.suites.includes(s.id) && covers(p, model));
      if (owners.length !== 1) throw new Error(`suite ${s.id} for ${model} must be in exactly one --part without arms, found ${owners.length}`);
    }
  for (const p of parts) {
    for (const model of p.models ?? [])
      if (!MODEL_ORDER.includes(model)) throw new Error(`--part names model ${model}, which is not on the page`);
    for (const id of p.suites) {
      if (!suites.some((s) => s.id === id)) throw new Error(`--part names suite ${id}, which has no runs`);
      for (const arm of p.arms ?? [])
        for (const model of p.models ?? MODEL_ORDER)
          if (!runs.some((r) => r.suite === id && r.arm === arm && r.model === model))
            throw new Error(`--part names ${id}@${arm} for ${model}, which has no runs`);
    }
  }
  return parts;
}

// partText names a part in prose: "debug on 2026-10-22 at commit 21bbe026",
// or for a part narrowed to the setups run again after a skill fix, "the
// debug CLI + skill runs again on 2026-10-22, with the skill fix on top of
// commit 21bbe026" (the fix is what the rerun measures, so it is named). A
// part narrowed to some models names them: "... for Haiku, DeepSeek and Opus".
function partText(p, setups, models) {
  const forModels = p.models ? ` for ${joinWords(p.models.map((k) => models.find((m) => m.key === k).short))}` : "";
  if (!p.arms) return `${joinWords(p.suites)}${forModels} on ${p.date} at commit \`${p.commit}\``;
  const labels = p.arms.map((a) => setups.find((x) => x.suite === p.suites[0] && x.arm === a)?.label ?? a);
  return `the ${joinWords(p.suites)} ${joinWords(labels)} runs${forModels} again on ${p.date}, with the skill fix of commit \`${p.commit}\``;
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

// A hidden-test failure carries the test runner's output, several KB over
// many lines. The page shows the first reason in a cell and a tooltip, so
// keep one line and the start of it.
const FIRST_FAILURE_MAX = 400;
function firstFailureText(text) {
  const flat = text.replace(/\s+/g, " ").trim();
  return flat.length > FIRST_FAILURE_MAX ? `${flat.slice(0, FIRST_FAILURE_MAX - 1).trimEnd()}…` : flat;
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
    // A run that ran out of its budget was still graded (its answer check
    // fails on "no submission"), so on the page it is a failure of kind
    // budget. Only an infrastructure error or the cost ceiling leaves a run
    // ungraded.
    return {
      case: r.case,
      suite: r.suite,
      arm: r.arm,
      model: r.model,
      status: r.status === "pass" ? "pass" : "fail",
      rawStatus: r.status,
      graded: r.status !== "infra_error" && r.status !== "cost_ceiling",
      kind,
      graders: r.failed_graders ? r.failed_graders.split("+") : [],
      firstFailure: firstFailureText(r.first_failure),
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
  const activeSuites = SUITES.filter((s) => runs.some((r) => r.suite === s.id));
  for (const r of runs)
    if (!activeSuites.some((s) => s.id === r.suite)) throw new Error(`unknown suite ${r.suite}`);

  const header = results.header;
  const runDate = header.started.slice(0, 10);
  const commit = header.gitSha.slice(0, 8);
  const parts = parseParts(args.part, activeSuites, header, runs);
  // Set when the stored transcripts were re-scored after a grader change
  // (twevals regrade); the page says so instead of comparing old and new graders.
  const regraded = header.regraded ? header.regraded.slice(0, 10) : null;

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
        notGraded: own.filter((r) => !r.graded).length,
        failingGraders,
      },
    };
  });

  const armsBySuite = Object.fromEntries(
    activeSuites.map((s) => [
      s.id,
      Object.keys(SETUPS).filter((arm) => runs.some((r) => r.suite === s.id && r.arm === arm)),
    ]),
  );
  for (const r of runs)
    if (!SETUPS[r.arm]) throw new Error(`unknown setup ${r.arm} in ${r.suite}`);

  const suites = activeSuites.map((s) => {
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

  const hasPair = (suite, [a, b]) => armsBySuite[suite].includes(a) && armsBySuite[suite].includes(b);
  const comparisons = {
    skillUplift: suites
      .filter((s) => hasPair(s.id, SKILL_PAIR))
      .flatMap((s) =>
        MODEL_ORDER.map((m) => {
          const a = setup(s.id, SKILL_PAIR[0], m);
          const b = setup(s.id, SKILL_PAIR[1], m);
          return { suite: s.id, model: m, with: pct(a.pass, a.runs), without: pct(b.pass, b.runs), pp: Math.round(rateOf(a) - rateOf(b)) };
        }),
      ),
    cliVsMcp: suites.flatMap((s) =>
      CLI_MCP_PAIRS.filter((pair) => hasPair(s.id, pair)).flatMap(([cli, mcp]) =>
        MODEL_ORDER.map((m) => {
          const a = setup(s.id, cli, m);
          const b = setup(s.id, mcp, m);
          return { suite: s.id, cliArm: cli, mcpArm: mcp, model: m, cli: pct(a.pass, a.runs), mcp: pct(b.pass, b.runs), pp: Math.round(rateOf(a) - rateOf(b)) };
        }),
      ),
    ),
  };

  const groups = GROUPS.filter((g) => runs.some((r) => groupOf(r.case) === g.id)).map((g) => {
    const own = runs.filter((r) => groupOf(r.case) === g.id);
    return {
      id: g.id,
      code: g.code,
      label: g.label,
      description: g.description,
      cases: new Set(own.map((r) => r.case)).size,
      results: Object.fromEntries(
        MODEL_ORDER.map((m) => {
          const mine = own.filter((r) => r.model === m);
          return [m, { runs: mine.length, pass: mine.filter((r) => r.status === "pass").length }];
        }),
      ),
    };
  });

  // An issue names one case, or a prefix glob such as `debug-*` when it cuts
  // across a suite.
  const issueCovers = (issue, caseId) =>
    issue.case.endsWith("*") ? caseId.startsWith(issue.case.slice(0, -1)) : caseId === issue.case;
  const issueOf = new Map();
  const knownIssues = KNOWN_ISSUES.filter((issue) => runs.some((r) => issueCovers(issue, r.case))).map((issue) => {
    const caseRuns = runs.filter((r) => issueCovers(issue, r.case));
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
    if (!caseById.has(id)) throw new Error(`description override for unknown case ${id}`);

  const judge = modelsYaml.judge;
  const data = {
    run: {
      title: `Traceway agent benchmarks: ${runDate}`,
      date: runDate,
      commit,
      parts,
      regraded,
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
        [...new Set(runs.map((r) => r.rawStatus))].map((s) => [s, runs.filter((r) => r.rawStatus === s).length]),
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
  // runs.csv rounds each run's cost to five decimals, so a sum over hundreds
  // of runs can differ from the harness's total by a cent.
  const expectUsd = (what, got, want) => {
    if (Math.abs(Number(got) - Number(String(want).replace("$", ""))) > 0.011)
      errors.push(`${what}: generated ${got}, report says ${want}`);
  };

  // "**Runs:** 1204 (budget_exhausted 6, fail 270, pass 928)": one count per
  // status the harness saw, in its own order.
  const runsLine = summaryMd.match(/\*\*Runs:\*\* (\d+) \(([^)]*)\)/);
  if (!runsLine) errors.push("summary.md: no Runs line");
  else {
    const counts = Object.fromEntries(runsLine[2].split(", ").map((x) => x.split(" ")).map(([k, v]) => [k, Number(v)]));
    expect("total runs", data.run.totalRuns, runsLine[1]);
    for (const [status, n] of Object.entries(counts)) expect(`${status} runs`, data.run.statuses[status] ?? 0, n);
    expect("status counts", Object.keys(data.run.statuses).length, Object.keys(counts).length);
  }

  const spend = summaryMd.match(/\*\*Spend:\*\* \$([\d.]+) \(judge \$([\d.]+)\)/);
  if (!spend) errors.push("summary.md: no Spend line");
  else {
    expectUsd("judge spend", data.run.judgeCostUsd.toFixed(2), spend[2]);
    expectUsd("total spend", (data.run.agentCostUsd + data.run.judgeCostUsd).toFixed(2), spend[1]);
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

  for (const [suite, model, w, wo, uplift] of tableRows(summaryMd, "## Skill uplift")) {
    const u = data.comparisons.skillUplift.find((x) => x.suite === suite && x.model === model);
    if (!u) {
      errors.push(`summary.md skill uplift ${suite} ${model} not generated`);
      continue;
    }
    expect(`${suite} ${model} skill uplift`, `${u.with}% ${u.without}% ${u.pp >= 0 ? "+" : ""}${u.pp}pp`, `${w} ${wo} ${uplift}`);
  }

  for (const [suite, model, cliArm, , cli, mcp, delta] of tableRows(summaryMd, "## CLI vs MCP")) {
    const c = data.comparisons.cliVsMcp.find((x) => x.suite === suite && x.model === model && x.cliArm === cliArm);
    if (!c) {
      errors.push(`summary.md CLI vs MCP ${suite} ${model} ${cliArm} not generated`);
      continue;
    }
    expect(`${suite} ${model} ${cliArm} CLI vs MCP`, `${c.cli}% ${c.mcp}% ${c.pp >= 0 ? "+" : ""}${c.pp}pp`, `${cli} ${mcp} ${delta}`);
  }
  expect("skill uplift rows", data.comparisons.skillUplift.length, tableRows(summaryMd, "## Skill uplift").length);
  expect("CLI vs MCP rows", data.comparisons.cliVsMcp.length, tableRows(summaryMd, "## CLI vs MCP").length);

  for (const row of tableRows(summaryMd, "## Cost (averages over passing runs)")) {
    const m = data.models.find((x) => x.key === row[0]);
    if (!m) continue;
    expect(`${m.key} passing runs`, m.totals.pass, row[1]);
    expect(`${m.key} cache hit`, `${Math.round(100 * m.totals.cacheHitRate)}%`, row[6]);
    expectUsd(`${m.key} agent spend`, m.totals.costUsd.toFixed(2), row[7]);
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
    expectUsd(`${suite} ${model} ${arm} cost`, s.costUsd.toFixed(2), cost);
    expect(`${suite} ${model} ${arm} avg calls`, s.avgToolCalls.toFixed(1), calls);
    expect(`${suite} ${model} ${arm} cache hit`, `${Math.round(100 * s.cacheHitRate)}%`, cache);
    expect(`${suite} ${model} ${arm} judge`, s.avgJudge === null ? "0.00" : s.avgJudge.toFixed(2), judge);
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
const words = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"];
const joinWords = (xs) => (xs.length < 2 ? xs.join("") : `${xs.slice(0, -1).join(", ")} and ${xs[xs.length - 1]}`);

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
    parts: run.parts.map((p) => ({
      suites: p.suites.join(","),
      ...(p.arms ? { arms: p.arms.join(",") } : {}),
      ...(p.models ? { models: p.models.join(",") } : {}),
      commit: p.commit,
      date: p.date,
    })),
    ...(run.regraded ? { regraded: run.regraded } : {}),
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
    ...[...new Set(comparisons.skillUplift.map((u) => u.suite))].map((suite) => [
      "Skill uplift",
      suite,
      SKILL_PAIR[0],
      SKILL_PAIR[1],
      ...models.map((m) => {
        const u = comparisons.skillUplift.find((x) => x.suite === suite && x.model === m.key);
        return `${signedPp(u.pp)} (${u.with}% vs ${u.without}%)`;
      }),
    ]),
    ...[...new Map(comparisons.cliVsMcp.map((c) => [`${c.suite} ${c.cliArm}`, c])).values()].map((first) => [
      "CLI vs MCP",
      first.suite,
      first.cliArm,
      first.mcpArm,
      ...models.map((m) => {
        const c = comparisons.cliVsMcp.find((x) => x.suite === first.suite && x.cliArm === first.cliArm && x.model === m.key);
        return `${signedPp(c.pp)} (${c.cli}% vs ${c.mcp}%)`;
      }),
    ]),
  ];

  const partsText =
    run.parts.length > 1
      ? ` in ${words[run.parts.length]} batches: ${run.parts.map((p) => partText(p, setups, models)).join(", ")}`
      : ` on ${run.date}, at commit \`${run.commit}\``;
  const regradedText = run.regraded
    ? ` Every saved transcript was scored on ${run.regraded} by the same graders: after a grader fix the transcripts are scored again, so no number here mixes two versions of a grader.`
    : "";

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

${words[models.length][0].toUpperCase()}${words[models.length].slice(1)} models, ${modelList}, each ran the same ${run.runsPerModel} eval runs${partsText}. Every run gets a fresh Traceway backend seeded with the telemetry of a system broken in one specific way, and deterministic code grades the result.${regradedText} [How we measure](${run.methodologyUrl}) · [the harness and the cases](${run.harnessUrl}).

This file is generated by \`website/scripts/build-benchmarks-data.mjs\` from the eval results and is the data behind ${PAGE_URL}. Change the generator, not this file.

## ${SECTIONS.summary}

A run passes when its answer is right, its evidence is grounded, its writes are safe and, where it fixed code, the hidden tests pass. "Right answer and evidence" also counts runs that failed only the trajectory check.

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
  ["Group", "Label", "Code", "Description", "Cases", ...labels],
  groups.map((g) => [g.id, g.label, g.code, g.description, g.cases, ...models.map((m) => frac(g.results[m.key].pass, g.results[m.key].runs))]),
)}

## ${SECTIONS.cost}

Every run per model. Agent cost is the model spend the harness recorded for each run, through OpenRouter; the judge's spend is separate. Runs went three at a time, each batch on one machine.

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
  expect("parts", JSON.stringify(parsed.run.parts), JSON.stringify(data.run.parts));
  expect("regraded", parsed.run.regraded ?? null, data.run.regraded);
  expect(
    "all graded",
    parsed.run.allGraded,
    (data.run.statuses.pass ?? 0) + (data.run.statuses.fail ?? 0) === data.run.totalRuns,
  );
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
    const fold = (x) => [x.code, x.cases, ...data.models.map((m) => `${x.results[m.key].pass}/${x.results[m.key].runs}`)].join(" ");
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
