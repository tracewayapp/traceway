import type { ModelKey, RunKind } from "@/lib/benchmarks";

export const MODEL_COLOR: Record<ModelKey, string> = {
  haiku: "var(--a3)",
  "deepseek-flash": "var(--a4)",
  opus: "var(--a1)",
};

export const KIND_LABEL: Record<RunKind, string> = {
  pass: "Pass",
  trajectory: "Right answer, missed a required step",
  wrong: "Wrong answer or ungrounded evidence",
  budget: "Ran out of budget",
};

export function rate(pass: number, runs: number) {
  return Math.round((100 * pass) / runs);
}

export function formatPp(pp: number) {
  if (pp === 0) return "±0pp";
  return `${pp > 0 ? "+" : "−"}${Math.abs(pp)}pp`;
}

export function formatUsd(usd: number) {
  if (usd >= 1) return `$${usd.toFixed(2)}`;
  if (usd >= 0.01) return `$${usd.toFixed(3)}`;
  return `$${usd.toFixed(4)}`;
}

export function numberWord(n: number) {
  const words = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"];
  return words[n] ?? String(n);
}
