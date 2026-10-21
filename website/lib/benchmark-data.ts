import fs from "node:fs";
import path from "node:path";
import { parseBenchmark } from "@/lib/benchmarks";

const DIR = path.join(process.cwd(), "content", "benchmarks");
const LATEST = fs
  .readdirSync(DIR)
  .filter((f) => /^\d{4}-\d{2}-\d{2}\.md$/.test(f))
  .sort()
  .at(-1);

if (!LATEST) {
  throw new Error(`no YYYY-MM-DD.md benchmark file in ${DIR}`);
}

export const BENCHMARK = parseBenchmark(
  fs.readFileSync(path.join(DIR, LATEST), "utf8"),
);
