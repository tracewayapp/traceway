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

export type ComparisonModel = { key: ModelKey; label: string; note: string };

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
  const featured = models[0].key;
  const outline = "color-mix(in oklab, var(--a1) 70%, transparent)";

  return (
    <div className="overflow-x-auto">
      <table
        className="w-full min-w-[560px] border-separate text-left md:min-w-[680px]"
        style={{ borderSpacing: 0 }}
      >
        <thead>
          <tr>
            <th
              className="sticky left-0 z-10 w-[132px] pb-3 md:static md:w-[26%]"
              style={{ background: "var(--ink-0)" }}
            />
            {models.map((m) => {
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
                  <span className="inline-flex items-center gap-2 text-[0.9375rem] font-medium text-fg-0 md:text-[1.0625rem]">
                    <span
                      aria-hidden
                      className="inline-block h-2.5 w-2.5 rounded-full"
                      style={{ background: MODEL_COLOR[m.key] }}
                    />
                    {m.label}
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
                {models.map((m) => {
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
  );
}
