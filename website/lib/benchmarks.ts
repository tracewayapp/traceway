import matter from "gray-matter";

export type ModelKey =
  | "haiku"
  | "deepseek-flash"
  | "opus"
  | "luna"
  | "minimax-flash"
  | "qwen-27b"
  | "glm";

export type Totals = {
  runs: number;
  pass: number;
  rightAnswerEvidence: number;
  notGraded: number;
  costUsd: number;
  costPerRunUsd: number;
  judgeCostUsd: number;
  avgToolCalls: number;
  avgWallS: number;
  cacheHitRate: number;
  avgJudge: number | null;
  judged: number;
};

export type Model = {
  key: ModelKey;
  label: string;
  short: string;
  vendor: string;
  weights: "open" | "proprietary";
  id: string;
  provider: string;
  providers: { name: string; runs: number }[];
  reasoningEffort: string;
  maxTokensPerTurn: number;
  totals: Totals;
};

export type SetupResult = {
  runs: number;
  pass: number;
  rightAnswerEvidence: number;
  costUsd: number;
  costPerRunUsd: number;
};

export type Setup = {
  suite: string;
  arm: string;
  label: string;
  description: string;
  results: Record<ModelKey, SetupResult>;
};

export type RunKind = "pass" | "trajectory" | "wrong" | "budget";

export type CaseRun = {
  status: "pass" | "fail";
  kind: RunKind;
  graders: string[];
  firstFailure: string;
  toolCalls: number;
  costUsd: number;
  judge: number | null;
  wallS: number;
  knownIssue: string | null;
};

export type BenchmarkCase = {
  id: string;
  suite: string;
  group: string;
  title: string;
  runs: Record<ModelKey, Record<string, CaseRun>>;
};

export type KnownIssue = {
  id: string;
  kind: "skill" | "case" | "harness";
  case: string;
  title: string;
  detail: string;
  caseRuns: number;
  affectedRuns: number;
  truncated: number;
  byModel: Record<ModelKey, number>;
};

export type Group = {
  id: string;
  label: string;
  code: string;
  description: string;
  cases: number;
  results: Record<ModelKey, { runs: number; pass: number }>;
};

// One batch of runs: which suites ran on which day at which commit, for
// which models when the batch did not cover every model on the page. A page
// built from one run has one part.
export type RunPart = {
  suites: string[];
  arms?: string[];
  models?: ModelKey[];
  commit: string;
  date: string;
};

export type BenchmarkData = {
  run: {
    title: string;
    date: string;
    commit: string;
    parts: RunPart[];
    // The date the stored transcripts were last re-scored, when the page's
    // runs were graded after a grader fix rather than at their own run time.
    regraded?: string;
    cliVersion: string;
    backendVersion: string;
    harnessUrl: string;
    methodologyUrl: string;
    runsPerCase: number;
    totalRuns: number;
    runsPerModel: number;
    allGraded: boolean;
    judge: { id: string; label: string };
  };
  models: Model[];
  suites: { id: string; label: string; cases: number; setups: string[] }[];
  setups: Setup[];
  comparisons: {
    skillUplift: {
      suite: string;
      model: ModelKey;
      with: number;
      without: number;
      pp: number;
    }[];
    cliVsMcp: {
      suite: string;
      cliArm: string;
      mcpArm: string;
      model: ModelKey;
      cli: number;
      mcp: number;
      pp: number;
    }[];
  };
  groups: Group[];
  knownIssues: KnownIssue[];
  cases: BenchmarkCase[];
};

export const SECTIONS = {
  summary: "Summary per model",
  passRate: "Pass rate by suite and setup",
  rightAnswer: "Right answer and evidence by suite and setup",
  comparisons: "Skill uplift and CLI vs MCP",
  groups: "By case group",
  cost: "Cost and efficiency",
  costBySetup: "Cost by suite and setup",
  knownIssues: "Known issues",
  matrix: "Full case matrix",
  runDetails: "Run details",
} as const;

export const MATRIX_KIND: Record<Exclude<RunKind, "pass">, string> = {
  wrong: "wrong answer/evidence",
  trajectory: "trajectory only",
  budget: "budget exhausted",
};

export const COST_METRICS = {
  spend: "Agent spend",
  perRun: "Average cost per run",
  perPass: "Cost per passing run",
  toolCalls: "Average tool calls per run",
  wall: "Average wall time per run",
  cacheHit: "Cache hit rate (input tokens)",
  judge: "Average judge score (0-1)",
  judged: "Runs scored by the judge",
  judgeSpend: "Judge spend",
} as const;

type Row = Record<string, string>;
type Table = { headers: string[]; rows: Row[] };

export function escapeCell(text: string) {
  return text.replace(/\|/g, "\\|").replace(/\n/g, " ");
}

function splitRow(line: string) {
  const inner = line.trim().replace(/^\|/, "").replace(/(?<!\\)\|$/, "");
  return inner.split(/(?<!\\)\|/).map((c) => c.trim().replace(/\\\|/g, "|"));
}

export function parseTables(markdown: string) {
  const tables = new Map<string, Table[]>();
  const lines = markdown.split("\n");
  let heading = "";
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const h = line.match(/^#{1,6}\s+(.+?)\s*$/);
    if (h) {
      heading = h[1];
      continue;
    }
    if (!line.trim().startsWith("|")) continue;
    const block: string[] = [];
    while (i < lines.length && lines[i].trim().startsWith("|")) block.push(lines[i++]);
    i--;
    if (block.length < 2 || !/^\|?\s*:?-+/.test(block[1].trim())) continue;
    const headers = splitRow(block[0]);
    const rows = block.slice(2).map((l) => {
      const cells = splitRow(l);
      if (cells.length !== headers.length)
        throw new Error(`"${heading}": row has ${cells.length} cells, header ${headers.length}: ${l}`);
      return Object.fromEntries(headers.map((name, j) => [name, cells[j]]));
    });
    const list = tables.get(heading) ?? [];
    list.push({ headers, rows });
    tables.set(heading, list);
  }
  return tables;
}

function table(tables: Map<string, Table[]>, heading: string): Table {
  const t = tables.get(heading)?.[0];
  if (!t) throw new Error(`benchmark markdown: no table under "${heading}"`);
  return t;
}

function cell(row: Row, header: string) {
  const value = row[header];
  if (value === undefined) throw new Error(`benchmark markdown: no "${header}" column`);
  return value;
}

function num(text: string) {
  const n = Number(text.replace(/[$,%s\s]/g, ""));
  if (!Number.isFinite(n) || text.trim() === "") throw new Error(`benchmark markdown: not a number: "${text}"`);
  return n;
}

function fraction(text: string) {
  const m = text.match(/^(\d+)\/(\d+)/);
  if (!m) throw new Error(`benchmark markdown: not a fraction: "${text}"`);
  return { pass: Number(m[1]), runs: Number(m[2]) };
}

function delta(text: string) {
  const m = text.match(/^([+-]?\d+)pp \((\d+)% vs (\d+)%\)$/);
  if (!m) throw new Error(`benchmark markdown: not a difference: "${text}"`);
  return { pp: Number(m[1]), a: Number(m[2]), b: Number(m[3]) };
}

function matrixCell(text: string) {
  if (text === "pass") return { kind: "pass" as RunKind, firstFailure: "" };
  const m = text.match(/^fail \(([^)]+)\): ([\s\S]*)$/);
  const kind = m
    ? (Object.entries(MATRIX_KIND).find(([, label]) => label === m[1])?.[0] as RunKind | undefined)
    : undefined;
  if (!m || !kind) throw new Error(`benchmark markdown: not a matrix cell: "${text}"`);
  return { kind, firstFailure: m[2] };
}

export function parseBenchmark(source: string): BenchmarkData {
  const { data: fm, content } = matter(source);
  const tables = parseTables(content);
  const fmModels = fm.models as Omit<Model, "totals">[];
  const keys = fmModels.map((m) => m.key);
  const keyByLabel = Object.fromEntries(fmModels.map((m) => [m.label, m.key]));
  const perModel = <T,>(f: (label: string, key: ModelKey) => T) =>
    Object.fromEntries(fmModels.map((m) => [m.key, f(m.label, m.key)])) as Record<ModelKey, T>;

  const cost = table(tables, SECTIONS.cost).rows;
  const metric = (name: string, label: string) => {
    const row = cost.find((r) => r.Metric === name);
    if (!row) throw new Error(`benchmark markdown: no "${name}" cost metric`);
    return cell(row, label);
  };

  const summary = table(tables, SECTIONS.summary).rows;
  const models: Model[] = fmModels.map((m) => {
    const row = summary.find((r) => r.Model === m.label);
    if (!row) throw new Error(`benchmark markdown: no summary row for ${m.label}`);
    const judge = metric(COST_METRICS.judge, m.label);
    return {
      ...m,
      totals: {
        runs: num(cell(row, "Runs")),
        pass: num(cell(row, "Passed")),
        rightAnswerEvidence: num(cell(row, "Right answer and evidence")),
        notGraded: num(cell(row, "Not graded")),
        costUsd: num(cell(row, "Agent cost")),
        costPerRunUsd: num(cell(row, "Cost per run")),
        judgeCostUsd: num(metric(COST_METRICS.judgeSpend, m.label)),
        avgToolCalls: num(metric(COST_METRICS.toolCalls, m.label)),
        avgWallS: num(metric(COST_METRICS.wall, m.label)),
        cacheHitRate: num(metric(COST_METRICS.cacheHit, m.label)) / 100,
        avgJudge: judge === "" ? null : num(judge),
        judged: num(metric(COST_METRICS.judged, m.label)),
      },
    };
  });

  const passRows = table(tables, SECTIONS.passRate).rows.filter((r) => r.Suite !== "all");
  const raeRows = table(tables, SECTIONS.rightAnswer).rows;
  const costRows = table(tables, SECTIONS.costBySetup).rows;
  const setups: Setup[] = passRows.map((r) => {
    const suite = cell(r, "Suite");
    const arm = cell(r, "Setup");
    const rae = raeRows.find((x) => x.Suite === suite && x.Setup === arm);
    if (!rae) throw new Error(`benchmark markdown: no right-answer row for ${suite} ${arm}`);
    return {
      suite,
      arm,
      label: cell(r, "Label"),
      description: cell(r, "Description"),
      results: perModel((label, key) => {
        const c = costRows.find((x) => x.Suite === suite && x.Setup === arm && keyByLabel[x.Model] === key);
        if (!c) throw new Error(`benchmark markdown: no cost row for ${suite} ${arm} ${label}`);
        return {
          ...fraction(cell(r, label)),
          rightAnswerEvidence: fraction(cell(rae, label)).pass,
          costUsd: num(cell(c, "Agent cost")),
          costPerRunUsd: num(cell(c, "Cost per run")),
        };
      }),
    };
  });

  const comparisonRows = table(tables, SECTIONS.comparisons).rows;
  const comparisons: BenchmarkData["comparisons"] = { skillUplift: [], cliVsMcp: [] };
  for (const r of comparisonRows) {
    for (const m of fmModels) {
      const d = delta(cell(r, m.label));
      if (r.Comparison === "Skill uplift")
        comparisons.skillUplift.push({ suite: r.Suite, model: m.key, with: d.a, without: d.b, pp: d.pp });
      else if (r.Comparison === "CLI vs MCP")
        comparisons.cliVsMcp.push({ suite: r.Suite, cliArm: cell(r, "Setup A"), mcpArm: cell(r, "Setup B"), model: m.key, cli: d.a, mcp: d.b, pp: d.pp });
      else throw new Error(`benchmark markdown: unknown comparison "${r.Comparison}"`);
    }
  }

  const groups: Group[] = table(tables, SECTIONS.groups).rows.map((r) => ({
    id: cell(r, "Group"),
    label: cell(r, "Label"),
    code: cell(r, "Code"),
    description: cell(r, "Description"),
    cases: num(cell(r, "Cases")),
    results: perModel((label) => fraction(cell(r, label))),
  }));

  const knownIssues: KnownIssue[] = table(tables, SECTIONS.knownIssues).rows.map((r) => {
    const byModel = perModel((label) => num(cell(r, label)));
    return {
      id: cell(r, "Issue"),
      kind: cell(r, "Kind") as KnownIssue["kind"],
      case: cell(r, "Case"),
      title: cell(r, "Title"),
      detail: cell(r, "Detail"),
      caseRuns: num(cell(r, "Runs of the case")),
      affectedRuns: keys.reduce((n, k) => n + byModel[k], 0),
      truncated: num(cell(r, "Truncated output")),
      byModel,
    };
  });

  const details = new Map<string, Row>();
  for (const r of table(tables, SECTIONS.runDetails).rows)
    details.set(`${r.Case}|${r.Setup}|${keyByLabel[r.Model]}`, r);

  const cases: BenchmarkCase[] = [];
  for (const r of table(tables, SECTIONS.matrix).rows) {
    const id = cell(r, "Case");
    let c = cases.find((x) => x.id === id);
    if (!c) {
      c = {
        id,
        suite: cell(r, "Suite"),
        group: cell(r, "Group"),
        title: cell(r, "Description"),
        runs: perModel(() => ({}) as Record<string, CaseRun>),
      };
      cases.push(c);
    }
    const arm = cell(r, "Setup");
    for (const m of fmModels) {
      const text = cell(r, m.label);
      if (text === "") continue;
      const { kind, firstFailure } = matrixCell(text);
      const d = details.get(`${id}|${arm}|${m.key}`);
      if (!d) throw new Error(`benchmark markdown: no run details for ${id} ${arm} ${m.label}`);
      const judge = cell(d, "Judge");
      const graders = cell(d, "Failed graders");
      c.runs[m.key][arm] = {
        status: kind === "pass" ? "pass" : "fail",
        kind,
        graders: graders ? graders.split("+") : [],
        firstFailure,
        toolCalls: num(cell(d, "Tool calls")),
        costUsd: num(cell(d, "Agent cost")),
        judge: judge === "" ? null : num(judge),
        wallS: num(cell(d, "Wall time (s)")),
        knownIssue: cell(d, "Known issue") || null,
      };
    }
  }

  const suites = (fm.suites as { id: string; label: string }[]).map((s) => ({
    ...s,
    cases: cases.filter((c) => c.suite === s.id).length,
    setups: setups.filter((x) => x.suite === s.id).map((x) => x.arm),
  }));

  const totalRuns = models.reduce((n, m) => n + m.totals.runs, 0);
  const fmParts = (fm.parts ?? []) as { suites: string; arms?: string; models?: string; commit: string; date: string }[];
  const parts: RunPart[] = fmParts.length
    ? fmParts.map((p) => ({
        suites: String(p.suites).split(","),
        ...(p.arms ? { arms: String(p.arms).split(",") } : {}),
        ...(p.models ? { models: String(p.models).split(",") as ModelKey[] } : {}),
        commit: String(p.commit),
        date: String(p.date),
      }))
    : [{ suites: (fm.suites as { id: string }[]).map((s) => s.id), commit: String(fm.commit), date: String(fm.date) }];
  return {
    run: {
      title: String(fm.title),
      date: String(fm.date),
      commit: String(fm.commit),
      parts,
      ...(fm.regraded ? { regraded: String(fm.regraded) } : {}),
      cliVersion: String(fm.cliVersion),
      backendVersion: String(fm.backendVersion),
      harnessUrl: String(fm.harnessUrl),
      methodologyUrl: String(fm.methodologyUrl),
      runsPerCase: Number(fm.runsPerCase),
      totalRuns,
      runsPerModel: totalRuns / models.length,
      allGraded:
        models.every((m) => m.totals.notGraded === 0) &&
        cases.every((c) => keys.every((k) => Object.values(c.runs[k]).every((x) => x.kind !== "budget"))),
      judge: fm.judge as { id: string; label: string },
    },
    models,
    suites,
    setups,
    comparisons,
    groups,
    knownIssues,
    cases,
  };
}
